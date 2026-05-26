package pass

import (
	_ "embed"
	"fmt"
	"unsafe"

	"github.com/cogentcore/webgpu/wgpu"

	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/render"
	"github.com/matthewjberger/indigo/transform"
	"github.com/matthewjberger/indigo/window"
)

//go:embed particle_update.wgsl
var particleUpdateShader string

//go:embed particle_render.wgsl
var particleRenderShader string

const (
	maxParticles uint32 = 16384
	maxEmitters  uint32 = 16
)

type particleEmitterGPU struct {
	Position          [4]float32
	Direction         [4]float32
	VelocityRange     [4]float32
	LifetimeRange     [4]float32
	SizeRange         [4]float32
	Gravity           [4]float32
	ColorGradient     [16][4]float32
	GradientCount     uint32
	SpawnCount        uint32
	EmitterID         uint32
	ShapeType         uint32
	ShapeParams       [4]float32
	Turbulence        [4]float32
	EmissiveStrength  float32
	Drag              float32
	EmitterType       uint32
	TextureIndex      uint32
	SizeCurve         [8][4]float32
	SizeCurveCount    uint32
	SizeCurvePad      [3]uint32
	OpacityCurve      [8][4]float32
	OpacityCurveCount uint32
	OpacityCurvePad   [3]uint32
}

type particleSimParams struct {
	DeltaTime    float32
	Time         float32
	MaxParticles uint32
	Padding      uint32
}

type particleCameraGPU struct {
	View           [16]float32
	Projection     [16]float32
	ViewProjection [16]float32
	CameraPosition [4]float32
	CameraRight    [4]float32
	CameraUp       [4]float32
}

type particleSystem struct {
	computeLayout  *wgpu.BindGroupLayout
	computeGroup   *wgpu.BindGroup
	resetPipeline  *wgpu.ComputePipeline
	spawnPipeline  *wgpu.ComputePipeline
	updatePipeline *wgpu.ComputePipeline
	particles      *wgpu.Buffer
	emitters       *wgpu.Buffer
	simParams      *wgpu.Buffer
	freeIndices    *wgpu.Buffer
	freeCount      *wgpu.Buffer
	aliveIndices   *wgpu.Buffer
	aliveCount     *wgpu.Buffer
	drawIndirect   *wgpu.Buffer
	emitterScratch []particleEmitterGPU
	activeEmitters uint32

	renderLayout0  *wgpu.BindGroupLayout
	renderLayout1  *wgpu.BindGroupLayout
	renderPipeline *wgpu.RenderPipeline
	cameraBuffer   *wgpu.Buffer
	renderGroup0   *wgpu.BindGroup
	renderGroup1   *wgpu.BindGroup
	dummyTexture   *wgpu.Texture
	dummyView      *wgpu.TextureView
	sampler        *wgpu.Sampler
	aspectFn       func() float32
}

func AddParticlePasses(renderer *render.Renderer, aspect func() float32) (*render.Pass, *render.Pass, error) {
	system, err := newParticleSystem(renderer.Device, aspect)
	if err != nil {
		return nil, nil, err
	}

	computePass := &render.Pass{
		Name:    "particle_compute",
		Prepare: func(c *render.PassContext) error { return particleComputePrepare(system, c) },
		Execute: func(c *render.PassContext) error { return particleComputeExecute(system, c) },
		Release: func() { particleSystemRelease(system) },
	}
	if err := renderer.Graph.AddPass(computePass, nil); err != nil {
		return nil, nil, err
	}

	renderPass := &render.Pass{
		Name:    "particle_render",
		Writes:  []string{"color", "depth"},
		Prepare: func(c *render.PassContext) error { return particleRenderPrepare(system, c) },
		Execute: func(c *render.PassContext) error { return particleRenderExecute(system, c) },
	}
	if err := renderer.Graph.AddPass(renderPass, []render.SlotBinding{
		{Slot: "color", ResourceID: renderer.SceneColorID},
		{Slot: "depth", ResourceID: renderer.DepthID},
	}); err != nil {
		return nil, nil, err
	}
	return computePass, renderPass, nil
}

func newParticleSystem(device *wgpu.Device, aspect func() float32) (*particleSystem, error) {
	queue := device.GetQueue()
	system := &particleSystem{aspectFn: aspect, emitterScratch: make([]particleEmitterGPU, maxEmitters)}

	var err error
	system.particles, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "particles", Size: uint64(maxParticles) * uint64(unsafe.Sizeof([8][4]float32{})),
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("particles: buffer: %w", err)
	}
	writeBufferStandalone(device, queue, system.particles, 0, make([]byte, uint64(maxParticles)*128))

	system.emitters, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "particle emitters", Size: uint64(maxEmitters) * uint64(unsafe.Sizeof(particleEmitterGPU{})),
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("particles: emitters: %w", err)
	}

	system.simParams, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "particle sim params", Size: uint64(unsafe.Sizeof(particleSimParams{})),
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("particles: sim params: %w", err)
	}

	freeInit := make([]uint32, maxParticles)
	for index := range freeInit {
		freeInit[index] = uint32(index)
	}
	system.freeIndices, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "particle free indices", Size: uint64(maxParticles) * 4,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("particles: free indices: %w", err)
	}
	writeBufferStandalone(device, queue, system.freeIndices, 0, sliceBytes(freeInit))

	system.freeCount, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "particle free count", Size: 4,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("particles: free count: %w", err)
	}
	freeCountInit := maxParticles
	writeBufferStandalone(device, queue, system.freeCount, 0, bytesOf(&freeCountInit))

	system.aliveIndices, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "particle alive indices", Size: uint64(maxParticles) * 4,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("particles: alive indices: %w", err)
	}

	system.aliveCount, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "particle alive count", Size: 4,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("particles: alive count: %w", err)
	}

	drawInit := [4]uint32{6, 0, 0, 0}
	system.drawIndirect, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "particle draw indirect", Size: 16,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageIndirect | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("particles: draw indirect: %w", err)
	}
	writeBufferStandalone(device, queue, system.drawIndirect, 0, bytesOf(&drawInit))

	if err := newParticleCompute(device, system); err != nil {
		return nil, err
	}
	if err := newParticleRender(device, queue, system); err != nil {
		return nil, err
	}
	return system, nil
}

func newParticleCompute(device *wgpu.Device, system *particleSystem) error {
	module, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "particle update shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: particleUpdateShader},
	})
	if err != nil {
		return fmt.Errorf("particles: update shader: %w", err)
	}
	defer module.Release()

	storage := func(binding uint32, readOnly bool) wgpu.BindGroupLayoutEntry {
		ty := wgpu.BufferBindingTypeStorage
		if readOnly {
			ty = wgpu.BufferBindingTypeReadOnlyStorage
		}
		return wgpu.BindGroupLayoutEntry{Binding: binding, Visibility: wgpu.ShaderStageCompute, Buffer: wgpu.BufferBindingLayout{Type: ty}}
	}
	layout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "particle compute layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			storage(0, false),
			storage(1, true),
			{Binding: 2, Visibility: wgpu.ShaderStageCompute, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform}},
			storage(3, false),
			storage(4, false),
			storage(5, false),
			storage(6, false),
			storage(7, false),
		},
	})
	if err != nil {
		return fmt.Errorf("particles: compute layout: %w", err)
	}
	system.computeLayout = layout

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "particle compute pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{layout},
	})
	if err != nil {
		return fmt.Errorf("particles: compute pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()

	makePipeline := func(entry string) (*wgpu.ComputePipeline, error) {
		return device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
			Layout:  pipelineLayout,
			Compute: wgpu.ProgrammableStageDescriptor{Module: module, EntryPoint: entry},
		})
	}
	if system.resetPipeline, err = makePipeline("reset_counters"); err != nil {
		return fmt.Errorf("particles: reset pipeline: %w", err)
	}
	if system.spawnPipeline, err = makePipeline("spawn"); err != nil {
		return fmt.Errorf("particles: spawn pipeline: %w", err)
	}
	if system.updatePipeline, err = makePipeline("update"); err != nil {
		return fmt.Errorf("particles: update pipeline: %w", err)
	}

	system.computeGroup, err = device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "particle compute group",
		Layout: layout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: system.particles, Size: wgpu.WholeSize},
			{Binding: 1, Buffer: system.emitters, Size: wgpu.WholeSize},
			{Binding: 2, Buffer: system.simParams, Size: wgpu.WholeSize},
			{Binding: 3, Buffer: system.freeIndices, Size: wgpu.WholeSize},
			{Binding: 4, Buffer: system.freeCount, Size: wgpu.WholeSize},
			{Binding: 5, Buffer: system.aliveIndices, Size: wgpu.WholeSize},
			{Binding: 6, Buffer: system.aliveCount, Size: wgpu.WholeSize},
			{Binding: 7, Buffer: system.drawIndirect, Size: wgpu.WholeSize},
		},
	})
	if err != nil {
		return fmt.Errorf("particles: compute group: %w", err)
	}
	return nil
}

func newParticleRender(device *wgpu.Device, queue *wgpu.Queue, system *particleSystem) error {
	module, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "particle render shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: particleRenderShader},
	})
	if err != nil {
		return fmt.Errorf("particles: render shader: %w", err)
	}
	defer module.Release()

	layout0, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "particle render layout 0",
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform}},
			{Binding: 1, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage}},
			{Binding: 2, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage}},
			{Binding: 3, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage}},
		},
	})
	if err != nil {
		return fmt.Errorf("particles: render layout 0: %w", err)
	}
	system.renderLayout0 = layout0

	layout1, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "particle render layout 1",
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeFloat, ViewDimension: wgpu.TextureViewDimension2DArray}},
			{Binding: 1, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeFiltering}},
		},
	})
	if err != nil {
		return fmt.Errorf("particles: render layout 1: %w", err)
	}
	system.renderLayout1 = layout1

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "particle render pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{layout0, layout1},
	})
	if err != nil {
		return fmt.Errorf("particles: render pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()

	additive := &wgpu.BlendState{
		Color: wgpu.BlendComponent{SrcFactor: wgpu.BlendFactorOne, DstFactor: wgpu.BlendFactorOne, Operation: wgpu.BlendOperationAdd},
		Alpha: wgpu.BlendComponent{SrcFactor: wgpu.BlendFactorOne, DstFactor: wgpu.BlendFactorOne, Operation: wgpu.BlendOperationAdd},
	}
	pipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:     "particle render pipeline",
		Layout:    pipelineLayout,
		Vertex:    wgpu.VertexState{Module: module, EntryPoint: "vs_main"},
		Primitive: wgpu.PrimitiveState{Topology: wgpu.PrimitiveTopologyTriangleList, CullMode: wgpu.CullModeNone},
		DepthStencil: &wgpu.DepthStencilState{
			Format:            render.DepthFormat,
			DepthWriteEnabled: false,
			DepthCompare:      wgpu.CompareFunctionGreaterEqual,
			StencilFront:      wgpu.StencilFaceState{Compare: wgpu.CompareFunctionAlways},
			StencilBack:       wgpu.StencilFaceState{Compare: wgpu.CompareFunctionAlways},
		},
		Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module:     module,
			EntryPoint: "fs_main_additive",
			Targets:    []wgpu.ColorTargetState{{Format: render.HdrFormat, Blend: additive, WriteMask: wgpu.ColorWriteMaskAll}},
		},
	})
	if err != nil {
		return fmt.Errorf("particles: render pipeline: %w", err)
	}
	system.renderPipeline = pipeline

	system.cameraBuffer, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "particle camera", Size: uint64(unsafe.Sizeof(particleCameraGPU{})),
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("particles: camera buffer: %w", err)
	}

	system.dummyTexture, err = device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "particle dummy texture",
		Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatRGBA8Unorm,
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("particles: dummy texture: %w", err)
	}
	queue.WriteTexture(
		&wgpu.ImageCopyTexture{Texture: system.dummyTexture, Aspect: wgpu.TextureAspectAll},
		[]byte{255, 255, 255, 255},
		&wgpu.TextureDataLayout{BytesPerRow: 4, RowsPerImage: 1},
		&wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
	)
	system.dummyView, err = system.dummyTexture.CreateView(&wgpu.TextureViewDescriptor{
		Label:           "particle dummy view",
		Dimension:       wgpu.TextureViewDimension2DArray,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
		Aspect:          wgpu.TextureAspectAll,
	})
	if err != nil {
		return fmt.Errorf("particles: dummy view: %w", err)
	}
	system.sampler, err = device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:         "particle sampler",
		AddressModeU:  wgpu.AddressModeClampToEdge,
		AddressModeV:  wgpu.AddressModeClampToEdge,
		AddressModeW:  wgpu.AddressModeClampToEdge,
		MagFilter:     wgpu.FilterModeLinear,
		MinFilter:     wgpu.FilterModeLinear,
		MipmapFilter:  wgpu.MipmapFilterModeNearest,
		MaxAnisotropy: 1,
	})
	if err != nil {
		return fmt.Errorf("particles: sampler: %w", err)
	}

	system.renderGroup0, err = device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "particle render group 0",
		Layout: layout0,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: system.cameraBuffer, Size: wgpu.WholeSize},
			{Binding: 1, Buffer: system.particles, Size: wgpu.WholeSize},
			{Binding: 2, Buffer: system.aliveIndices, Size: wgpu.WholeSize},
			{Binding: 3, Buffer: system.aliveCount, Size: wgpu.WholeSize},
		},
	})
	if err != nil {
		return fmt.Errorf("particles: render group 0: %w", err)
	}
	system.renderGroup1, err = device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "particle render group 1",
		Layout: layout1,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: system.dummyView},
			{Binding: 1, Sampler: system.sampler},
		},
	})
	if err != nil {
		return fmt.Errorf("particles: render group 1: %w", err)
	}
	return nil
}

func particleComputePrepare(system *particleSystem, context *render.PassContext) error {
	timing := ecs.MustResource[window.Window](context.World).Timing
	delta := timing.DeltaSeconds

	count := uint32(0)
	mask := ecs.MustMaskOf[render.ParticleEmitter](context.World)
	context.World.ForEach(mask, 0, func(entity ecs.Entity, _ *ecs.Archetype, _ int) {
		if count >= maxEmitters {
			return
		}
		emitter, ok := ecs.GetMut[render.ParticleEmitter](context.World, entity)
		if !ok {
			return
		}
		position := transform.Vec3{}
		if global, ok := ecs.Get[transform.GlobalTransform](context.World, entity); ok {
			position = transform.GlobalTransformTranslation(global)
		}
		emitter.Accumulator += emitter.SpawnRate * delta
		spawn := uint32(emitter.Accumulator)
		if spawn > 256 {
			spawn = 256
		}
		emitter.Accumulator -= float32(spawn)
		system.emitterScratch[count] = buildParticleEmitterGPU(emitter, position, spawn, count)
		count++
	})
	system.activeEmitters = count

	if count > 0 {
		writeBuffer(context.Device, context.Queue, context.Encoder, system.emitters, 0, sliceBytes(system.emitterScratch[:count]))
	}
	params := particleSimParams{DeltaTime: delta, Time: timing.UptimeSeconds, MaxParticles: maxParticles}
	writeBuffer(context.Device, context.Queue, context.Encoder, system.simParams, 0, bytesOf(&params))
	return nil
}

func particleComputeExecute(system *particleSystem, context *render.PassContext) error {
	encoder := context.Encoder.BeginComputePass(nil)
	encoder.SetBindGroup(0, system.computeGroup, nil)

	encoder.SetPipeline(system.resetPipeline)
	encoder.DispatchWorkgroups(1, 1, 1)

	if system.activeEmitters > 0 {
		encoder.SetPipeline(system.spawnPipeline)
		encoder.DispatchWorkgroups(system.activeEmitters, 1, 1)
	}

	encoder.SetPipeline(system.updatePipeline)
	encoder.DispatchWorkgroups((maxParticles+255)/256, 1, 1)

	encoder.End()
	encoder.Release()
	return nil
}

func particleRenderPrepare(system *particleSystem, context *render.PassContext) error {
	camera := ecs.MustResource[render.Camera](context.World)
	aspect := system.aspectFn()
	view := render.CameraView(camera)
	projection := render.CameraProjection(camera, aspect)
	viewProjection := projection.Mul4(view)
	viewInverse := view.Inv()
	right := viewInverse.Col(0)
	up := viewInverse.Col(1)

	uniform := particleCameraGPU{
		View:           view,
		Projection:     projection,
		ViewProjection: viewProjection,
		CameraPosition: [4]float32{camera.Eye[0], camera.Eye[1], camera.Eye[2], 1},
		CameraRight:    [4]float32{right.X(), right.Y(), right.Z(), 0},
		CameraUp:       [4]float32{up.X(), up.Y(), up.Z(), 0},
	}
	writeBuffer(context.Device, context.Queue, context.Encoder, system.cameraBuffer, 0, bytesOf(&uniform))
	return nil
}

func particleRenderExecute(system *particleSystem, context *render.PassContext) error {
	colorAttachment, err := context.ColorAttachment("color")
	if err != nil {
		return err
	}
	depthAttachment, err := context.DepthAttachment("depth")
	if err != nil {
		return err
	}
	pass := context.Encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label:                  "particle render",
		ColorAttachments:       []wgpu.RenderPassColorAttachment{colorAttachment},
		DepthStencilAttachment: &depthAttachment,
	})
	pass.SetPipeline(system.renderPipeline)
	pass.SetBindGroup(0, system.renderGroup0, nil)
	pass.SetBindGroup(1, system.renderGroup1, nil)
	particleDraw(pass, system)
	pass.End()
	pass.Release()
	return nil
}

func buildParticleEmitterGPU(emitter *render.ParticleEmitter, position transform.Vec3, spawn, id uint32) particleEmitterGPU {
	direction := emitter.Direction
	if direction.Len() < 1e-4 {
		direction = transform.Vec3{0, 1, 0}
	} else {
		direction = direction.Normalize()
	}
	gpu := particleEmitterGPU{
		Position:         [4]float32{position.X(), position.Y(), position.Z(), 1},
		Direction:        [4]float32{direction.X(), direction.Y(), direction.Z(), 0},
		VelocityRange:    [4]float32{emitter.VelocityMin, emitter.VelocityMax, emitter.Spread, 0},
		LifetimeRange:    [4]float32{emitter.LifetimeMin, emitter.LifetimeMax, 0, 0},
		SizeRange:        [4]float32{emitter.SizeStart, emitter.SizeEnd, 0, 0},
		Gravity:          [4]float32{0, emitter.Gravity, 0, 0},
		GradientCount:    2,
		SpawnCount:       spawn,
		EmitterID:        id,
		ShapeType:        uint32(emitter.Shape),
		ShapeParams:      emitter.ShapeParams,
		Turbulence:       [4]float32{emitter.TurbulenceStrength, emitter.TurbulenceFreq, 0, 0},
		EmissiveStrength: emitter.EmissiveStrength,
		Drag:             emitter.Drag,
		EmitterType:      uint32(emitter.EmitterType),
		TextureIndex:     0,
	}
	gpu.ColorGradient[0] = [4]float32{0.0, emitter.ColorStart[0], emitter.ColorStart[1], emitter.ColorStart[2]}
	gpu.ColorGradient[1] = [4]float32{emitter.ColorStart[3], 0, 0, 0}
	gpu.ColorGradient[2] = [4]float32{1.0, emitter.ColorEnd[0], emitter.ColorEnd[1], emitter.ColorEnd[2]}
	gpu.ColorGradient[3] = [4]float32{emitter.ColorEnd[3], 0, 0, 0}
	return gpu
}

func particleSystemRelease(system *particleSystem) {
	for _, buffer := range []*wgpu.Buffer{
		system.particles, system.emitters, system.simParams, system.freeIndices,
		system.freeCount, system.aliveIndices, system.aliveCount, system.drawIndirect, system.cameraBuffer,
	} {
		if buffer != nil {
			buffer.Release()
		}
	}
	if system.computeGroup != nil {
		system.computeGroup.Release()
	}
	if system.renderGroup0 != nil {
		system.renderGroup0.Release()
	}
	if system.renderGroup1 != nil {
		system.renderGroup1.Release()
	}
	if system.resetPipeline != nil {
		system.resetPipeline.Release()
	}
	if system.spawnPipeline != nil {
		system.spawnPipeline.Release()
	}
	if system.updatePipeline != nil {
		system.updatePipeline.Release()
	}
	if system.renderPipeline != nil {
		system.renderPipeline.Release()
	}
	if system.dummyView != nil {
		system.dummyView.Release()
	}
	if system.dummyTexture != nil {
		system.dummyTexture.Release()
	}
	if system.sampler != nil {
		system.sampler.Release()
	}
	if system.computeLayout != nil {
		system.computeLayout.Release()
	}
	if system.renderLayout0 != nil {
		system.renderLayout0.Release()
	}
	if system.renderLayout1 != nil {
		system.renderLayout1.Release()
	}
}
