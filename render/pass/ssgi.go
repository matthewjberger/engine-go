package pass

import (
	_ "embed"
	"fmt"
	"unsafe"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/go-gl/mathgl/mgl32"

	"indigo/ecs"
	"indigo/render"
)

//go:embed ssgi.wgsl
var ssgiShader string

//go:embed ssgi_blur.wgsl
var ssgiBlurShader string

const ssgiKernelSize = 16

type ssgiParams struct {
	Projection    mgl32.Mat4
	InvProjection mgl32.Mat4
	ScreenSize    mgl32.Vec2
	Radius        float32
	Intensity     float32
	MaxSteps      uint32
	Enabled       float32
	Pad           mgl32.Vec2
}

type ssgiBlurParams struct {
	ScreenSize      mgl32.Vec2
	DepthThreshold  float32
	NormalThreshold float32
}

// ssgiFormat carries indirect-bounce color at HDR range so very
// bright bounces clamp only in the final tonemap.
const ssgiFormat = wgpu.TextureFormatRGBA16Float

// SsgiSettings drives the SSGI pass each frame.
type SsgiSettings struct {
	Enabled   bool
	Radius    float32
	Intensity float32
	MaxSteps  uint32
}

// DefaultSsgiSettings matches the reference engine's defaults.
func DefaultSsgiSettings() SsgiSettings {
	return SsgiSettings{
		Enabled:   true,
		Radius:    2.0,
		Intensity: 1.0,
		MaxSteps:  16,
	}
}

// SsgiResult exposes the blurred indirect bounce texture so the
// postprocess pass can add it into the HDR scene color.
type SsgiResult struct {
	View *wgpu.TextureView
}

type SsgiResource struct {
	Result *SsgiResult
}

type ssgiPassState struct {
	pipeline      *wgpu.RenderPipeline
	bgLayout      *wgpu.BindGroupLayout
	pointSampler  *wgpu.Sampler
	linearSampler *wgpu.Sampler
	noiseSampler  *wgpu.Sampler
	paramsBuffer  *wgpu.Buffer
	kernelBuffer  *wgpu.Buffer
	noiseTexture  *wgpu.Texture
	noiseView     *wgpu.TextureView
	rawTexture    *wgpu.Texture
	rawView       *wgpu.TextureView
	bindGroup     *wgpu.BindGroup
	currentWidth  uint32
	currentHeight uint32
	aspectFn      func() float32
}

type ssgiBlurPassState struct {
	owner         *ssgiPassState
	pipeline      *wgpu.RenderPipeline
	bgLayout      *wgpu.BindGroupLayout
	paramsBuffer  *wgpu.Buffer
	outputTexture *wgpu.Texture
	outputView    *wgpu.TextureView
	bindGroup     *wgpu.BindGroup
	currentWidth  uint32
	currentHeight uint32
}

// AddSsgiPass registers the screen-space indirect-bounce pass and
// its bilateral blur. Depth + view_normals + scene_color are the
// inputs; the blurred output is published as SsgiResource for the
// postprocess pass.
func AddSsgiPass(renderer *render.Renderer, aspect func() float32) (*render.Pass, *render.Pass, error) {
	state, err := newSsgiState(renderer.Device, aspect)
	if err != nil {
		return nil, nil, err
	}
	pass := &render.Pass{
		Name:                 "ssgi",
		Reads:                []string{"depth", "view_normals", "scene_color"},
		State:                state,
		Prepare:              ssgiPrepare,
		Execute:              ssgiExecute,
		InvalidateBindGroups: ssgiInvalidate,
		Release:              ssgiRelease,
	}
	if err := renderer.Graph.AddPass(pass, []render.SlotBinding{
		{Slot: "depth", ResourceID: renderer.DepthID},
		{Slot: "view_normals", ResourceID: renderer.ViewNormalsID},
		{Slot: "scene_color", ResourceID: renderer.SceneColorID},
	}); err != nil {
		return nil, nil, err
	}

	blurState, err := newSsgiBlurState(renderer.Device, state)
	if err != nil {
		return nil, nil, err
	}
	blurPass := &render.Pass{
		Name:                 "ssgi_blur",
		Reads:                []string{"depth", "view_normals"},
		State:                blurState,
		Prepare:              ssgiBlurPrepare,
		Execute:              ssgiBlurExecute,
		InvalidateBindGroups: ssgiBlurInvalidate,
		Release:              ssgiBlurRelease,
	}
	if err := renderer.Graph.AddPass(blurPass, []render.SlotBinding{
		{Slot: "depth", ResourceID: renderer.DepthID},
		{Slot: "view_normals", ResourceID: renderer.ViewNormalsID},
	}); err != nil {
		return nil, nil, err
	}
	return pass, blurPass, nil
}

func newSsgiState(device *wgpu.Device, aspect func() float32) (*ssgiPassState, error) {
	module, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "ssgi shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: ssgiShader},
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: shader: %w", err)
	}
	defer module.Release()

	bgLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "ssgi bg layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeDepth, ViewDimension: wgpu.TextureViewDimension2D}},
			{Binding: 1, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeFloat, ViewDimension: wgpu.TextureViewDimension2D}},
			{Binding: 2, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeFloat, ViewDimension: wgpu.TextureViewDimension2D}},
			{Binding: 3, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeFloat, ViewDimension: wgpu.TextureViewDimension2D}},
			{Binding: 4, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeNonFiltering}},
			{Binding: 5, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeFiltering}},
			{Binding: 6, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeFiltering}},
			{Binding: 7, Visibility: wgpu.ShaderStageFragment, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform}},
			{Binding: 8, Visibility: wgpu.ShaderStageFragment, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: bg layout: %w", err)
	}
	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "ssgi pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bgLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()

	pipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "ssgi pipeline",
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{Module: module, EntryPoint: "vertex_main"},
		Primitive: wgpu.PrimitiveState{
			Topology: wgpu.PrimitiveTopologyTriangleList,
			CullMode: wgpu.CullModeNone,
		},
		Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module:     module,
			EntryPoint: "fragment_main",
			Targets:    []wgpu.ColorTargetState{{Format: ssgiFormat, WriteMask: wgpu.ColorWriteMaskAll}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: pipeline: %w", err)
	}

	pointSampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:         "ssgi point sampler",
		AddressModeU:  wgpu.AddressModeClampToEdge,
		AddressModeV:  wgpu.AddressModeClampToEdge,
		AddressModeW:  wgpu.AddressModeClampToEdge,
		MagFilter:     wgpu.FilterModeNearest,
		MinFilter:     wgpu.FilterModeNearest,
		MipmapFilter:  wgpu.MipmapFilterModeNearest,
		MaxAnisotropy: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: point sampler: %w", err)
	}
	linearSampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:         "ssgi linear sampler",
		AddressModeU:  wgpu.AddressModeClampToEdge,
		AddressModeV:  wgpu.AddressModeClampToEdge,
		AddressModeW:  wgpu.AddressModeClampToEdge,
		MagFilter:     wgpu.FilterModeLinear,
		MinFilter:     wgpu.FilterModeLinear,
		MipmapFilter:  wgpu.MipmapFilterModeNearest,
		MaxAnisotropy: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: linear sampler: %w", err)
	}
	noiseSampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:         "ssgi noise sampler",
		AddressModeU:  wgpu.AddressModeRepeat,
		AddressModeV:  wgpu.AddressModeRepeat,
		AddressModeW:  wgpu.AddressModeRepeat,
		MagFilter:     wgpu.FilterModeNearest,
		MinFilter:     wgpu.FilterModeNearest,
		MipmapFilter:  wgpu.MipmapFilterModeNearest,
		MaxAnisotropy: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: noise sampler: %w", err)
	}

	paramsBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "ssgi params",
		Size:  uint64(unsafe.Sizeof(ssgiParams{})),
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: params: %w", err)
	}

	kernel := buildSsgiKernel()
	kernelBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "ssgi kernel",
		Size:  uint64(len(kernel)) * uint64(unsafe.Sizeof(mgl32.Vec4{})),
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: kernel buffer: %w", err)
	}
	device.GetQueue().WriteBuffer(kernelBuffer, 0, unsafe.Slice((*byte)(unsafe.Pointer(&kernel[0])), len(kernel)*16))

	noise := buildSsgiNoise()
	noiseTex, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "ssgi noise",
		Size:          wgpu.Extent3D{Width: ssaoNoiseSize, Height: ssaoNoiseSize, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatRGBA8Unorm,
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi: noise texture: %w", err)
	}
	noiseView, err := noiseTex.CreateView(nil)
	if err != nil {
		return nil, fmt.Errorf("ssgi: noise view: %w", err)
	}
	device.GetQueue().WriteTexture(
		&wgpu.ImageCopyTexture{Texture: noiseTex, Aspect: wgpu.TextureAspectAll},
		noise,
		&wgpu.TextureDataLayout{BytesPerRow: ssaoNoiseSize * 4, RowsPerImage: ssaoNoiseSize},
		&wgpu.Extent3D{Width: ssaoNoiseSize, Height: ssaoNoiseSize, DepthOrArrayLayers: 1},
	)

	return &ssgiPassState{
		pipeline:      pipeline,
		bgLayout:      bgLayout,
		pointSampler:  pointSampler,
		linearSampler: linearSampler,
		noiseSampler:  noiseSampler,
		paramsBuffer:  paramsBuffer,
		kernelBuffer:  kernelBuffer,
		noiseTexture:  noiseTex,
		noiseView:     noiseView,
		aspectFn:      aspect,
	}, nil
}

func ssgiPrepare(s any, context *render.PassContext) error {
	state := s.(*ssgiPassState)
	fullWidth, fullHeight, err := ssaoSize(context, "depth")
	if err != nil {
		return err
	}
	// SSGI runs at half resolution: 4x less ray-march cost and the
	// 2x2 upsample to full res via the postprocess linear sampler
	// hides the per-pixel kernel jitter that otherwise reads as
	// screen-locked ghosting during camera motion. Matches the
	// reference engine's setup exactly.
	width := fullWidth / 2
	if width == 0 {
		width = 1
	}
	height := fullHeight / 2
	if height == 0 {
		height = 1
	}
	if state.currentWidth != width || state.currentHeight != height {
		if err := state.recreateRawTexture(context.Device, width, height); err != nil {
			return err
		}
		state.currentWidth = width
		state.currentHeight = height
		state.bindGroup = nil
	}

	settings := DefaultSsgiSettings()
	if resource, ok := ecs.Resource[SsgiSettings](context.World); ok {
		settings = *resource
	}
	enabled := float32(0)
	if settings.Enabled {
		enabled = 1
	}
	_, projection := ssaoViewProj(context, state.aspectFn())
	params := ssgiParams{
		Projection:    projection,
		InvProjection: projection.Inv(),
		ScreenSize:    mgl32.Vec2{float32(width), float32(height)},
		Radius:        settings.Radius,
		Intensity:     settings.Intensity,
		MaxSteps:      settings.MaxSteps,
		Enabled:       enabled,
	}
	writeBuffer(context.Device, context.Queue, context.Encoder, state.paramsBuffer, 0, bytesOf(&params))

	if state.bindGroup != nil {
		return nil
	}
	depthView, err := context.TextureView("depth")
	if err != nil {
		return err
	}
	normalView, err := context.TextureView("view_normals")
	if err != nil {
		return err
	}
	sceneView, err := context.TextureView("scene_color")
	if err != nil {
		return err
	}
	bg, err := context.Device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "ssgi bg",
		Layout: state.bgLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: depthView},
			{Binding: 1, TextureView: normalView},
			{Binding: 2, TextureView: sceneView},
			{Binding: 3, TextureView: state.noiseView},
			{Binding: 4, Sampler: state.pointSampler},
			{Binding: 5, Sampler: state.linearSampler},
			{Binding: 6, Sampler: state.noiseSampler},
			{Binding: 7, Buffer: state.paramsBuffer, Offset: 0, Size: uint64(unsafe.Sizeof(ssgiParams{}))},
			{Binding: 8, Buffer: state.kernelBuffer, Offset: 0, Size: uint64(ssgiKernelSize) * uint64(unsafe.Sizeof(mgl32.Vec4{}))},
		},
	})
	if err != nil {
		return fmt.Errorf("ssgi: bind group: %w", err)
	}
	state.bindGroup = bg
	return nil
}

func ssgiExecute(s any, context *render.PassContext) error {
	state := s.(*ssgiPassState)
	if state.rawView == nil || state.bindGroup == nil {
		return nil
	}
	passEnc := context.Encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "ssgi",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       state.rawView,
			LoadOp:     wgpu.LoadOpClear,
			StoreOp:    wgpu.StoreOpStore,
			ClearValue: wgpu.Color{},
		}},
	})
	passEnc.SetPipeline(state.pipeline)
	passEnc.SetBindGroup(0, state.bindGroup, nil)
	passEnc.Draw(3, 1, 0, 0)
	passEnc.End()
	passEnc.Release()
	return nil
}

func ssgiInvalidate(s any) {
	state := s.(*ssgiPassState)
	if state.bindGroup != nil {
		state.bindGroup.Release()
		state.bindGroup = nil
	}
}

func ssgiBlurInvalidate(s any) {
	state := s.(*ssgiBlurPassState)
	if state.bindGroup != nil {
		state.bindGroup.Release()
		state.bindGroup = nil
	}
}

func ssgiRelease(s any) {
	state := s.(*ssgiPassState)
	if state.bindGroup != nil {
		state.bindGroup.Release()
	}
	if state.rawView != nil {
		state.rawView.Release()
	}
	if state.rawTexture != nil {
		state.rawTexture.Release()
	}
	if state.noiseView != nil {
		state.noiseView.Release()
	}
	if state.noiseTexture != nil {
		state.noiseTexture.Release()
	}
	if state.kernelBuffer != nil {
		state.kernelBuffer.Release()
	}
	if state.paramsBuffer != nil {
		state.paramsBuffer.Release()
	}
	if state.noiseSampler != nil {
		state.noiseSampler.Release()
	}
	if state.linearSampler != nil {
		state.linearSampler.Release()
	}
	if state.pointSampler != nil {
		state.pointSampler.Release()
	}
	if state.pipeline != nil {
		state.pipeline.Release()
	}
	if state.bgLayout != nil {
		state.bgLayout.Release()
	}
}

func (state *ssgiPassState) recreateRawTexture(device *wgpu.Device, width, height uint32) error {
	if state.rawView != nil {
		state.rawView.Release()
	}
	if state.rawTexture != nil {
		state.rawTexture.Release()
	}
	tex, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "ssgi raw",
		Size:          wgpu.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        ssgiFormat,
		Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
	})
	if err != nil {
		return fmt.Errorf("ssgi: raw texture: %w", err)
	}
	view, err := tex.CreateView(nil)
	if err != nil {
		tex.Release()
		return fmt.Errorf("ssgi: raw view: %w", err)
	}
	state.rawTexture = tex
	state.rawView = view
	return nil
}

// buildSsgiKernel mirrors the reference engine's kernel build: the
// same LCG seeded with 67890, hemisphere vectors with Z in
// [0.1, 1.0], scaled by 0.1 + (i/N)^2 * 0.9 to cluster samples
// close to the surface.
func buildSsgiKernel() []mgl32.Vec4 {
	rng := nightshadeLCG(67890)
	kernel := make([]mgl32.Vec4, ssgiKernelSize)
	for index := 0; index < ssgiKernelSize; index++ {
		x := rng.nextFloat()*2 - 1
		y := rng.nextFloat()*2 - 1
		z := rng.nextFloat()*0.9 + 0.1
		sample := mgl32.Vec3{x, y, z}.Normalize()
		t := float32(index) / float32(ssgiKernelSize)
		scale := 0.1 + t*t*0.9
		sample = sample.Mul(scale)
		kernel[index] = mgl32.Vec4{sample.X(), sample.Y(), sample.Z(), 0}
	}
	return kernel
}

// buildSsgiNoise is the SSGI counterpart of buildSsaoNoise, seeded
// with 98765 to match the reference engine's noise pattern.
func buildSsgiNoise() []byte {
	rng := nightshadeLCG(98765)
	noise := make([]byte, 0, ssaoNoiseSize*ssaoNoiseSize*4)
	for index := 0; index < ssaoNoiseSize*ssaoNoiseSize; index++ {
		x := rng.nextFloat()*2 - 1
		y := rng.nextFloat()*2 - 1
		noise = append(noise, byte((x*0.5+0.5)*255), byte((y*0.5+0.5)*255), 128, 255)
	}
	return noise
}

func newSsgiBlurState(device *wgpu.Device, owner *ssgiPassState) (*ssgiBlurPassState, error) {
	module, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "ssgi blur shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: ssgiBlurShader},
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi blur: shader: %w", err)
	}
	defer module.Release()

	bgLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "ssgi blur bg layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeFloat, ViewDimension: wgpu.TextureViewDimension2D}},
			{Binding: 1, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeDepth, ViewDimension: wgpu.TextureViewDimension2D}},
			{Binding: 2, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeFloat, ViewDimension: wgpu.TextureViewDimension2D}},
			{Binding: 3, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeFiltering}},
			{Binding: 4, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeNonFiltering}},
			{Binding: 5, Visibility: wgpu.ShaderStageFragment, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi blur: bg layout: %w", err)
	}
	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "ssgi blur pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bgLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi blur: pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()

	pipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "ssgi blur pipeline",
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{Module: module, EntryPoint: "vertex_main"},
		Primitive: wgpu.PrimitiveState{
			Topology: wgpu.PrimitiveTopologyTriangleList,
			CullMode: wgpu.CullModeNone,
		},
		Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module:     module,
			EntryPoint: "fragment_main",
			Targets:    []wgpu.ColorTargetState{{Format: ssgiFormat, WriteMask: wgpu.ColorWriteMaskAll}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi blur: pipeline: %w", err)
	}

	paramsBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "ssgi blur params",
		Size:  uint64(unsafe.Sizeof(ssgiBlurParams{})),
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("ssgi blur: params: %w", err)
	}

	return &ssgiBlurPassState{
		owner:        owner,
		pipeline:     pipeline,
		bgLayout:     bgLayout,
		paramsBuffer: paramsBuffer,
	}, nil
}

func ssgiBlurPrepare(s any, context *render.PassContext) error {
	state := s.(*ssgiBlurPassState)
	fullWidth, fullHeight, err := ssaoSize(context, "depth")
	if err != nil {
		return err
	}
	width := fullWidth / 2
	if width == 0 {
		width = 1
	}
	height := fullHeight / 2
	if height == 0 {
		height = 1
	}
	if state.currentWidth != width || state.currentHeight != height {
		if err := state.recreateOutput(context.Device, width, height); err != nil {
			return err
		}
		state.currentWidth = width
		state.currentHeight = height
		state.bindGroup = nil
	}
	params := ssgiBlurParams{
		ScreenSize:      mgl32.Vec2{float32(width), float32(height)},
		DepthThreshold:  0.5,
		NormalThreshold: 32.0,
	}
	writeBuffer(context.Device, context.Queue, context.Encoder, state.paramsBuffer, 0, bytesOf(&params))

	if state.bindGroup != nil {
		ecsSetSsgiResource(context, state.outputView)
		return nil
	}
	if state.owner == nil || state.owner.rawView == nil {
		return nil
	}
	depthView, err := context.TextureView("depth")
	if err != nil {
		return err
	}
	normalView, err := context.TextureView("view_normals")
	if err != nil {
		return err
	}
	bg, err := context.Device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "ssgi blur bg",
		Layout: state.bgLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: state.owner.rawView},
			{Binding: 1, TextureView: depthView},
			{Binding: 2, TextureView: normalView},
			{Binding: 3, Sampler: state.owner.linearSampler},
			{Binding: 4, Sampler: state.owner.pointSampler},
			{Binding: 5, Buffer: state.paramsBuffer, Offset: 0, Size: uint64(unsafe.Sizeof(ssgiBlurParams{}))},
		},
	})
	if err != nil {
		return fmt.Errorf("ssgi blur: bind group: %w", err)
	}
	state.bindGroup = bg
	ecsSetSsgiResource(context, state.outputView)
	return nil
}

func ssgiBlurExecute(s any, context *render.PassContext) error {
	state := s.(*ssgiBlurPassState)
	if state.outputView == nil || state.bindGroup == nil {
		return nil
	}
	passEnc := context.Encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "ssgi blur",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       state.outputView,
			LoadOp:     wgpu.LoadOpClear,
			StoreOp:    wgpu.StoreOpStore,
			ClearValue: wgpu.Color{},
		}},
	})
	passEnc.SetPipeline(state.pipeline)
	passEnc.SetBindGroup(0, state.bindGroup, nil)
	passEnc.Draw(3, 1, 0, 0)
	passEnc.End()
	passEnc.Release()
	return nil
}

func ssgiBlurRelease(s any) {
	state := s.(*ssgiBlurPassState)
	if state.bindGroup != nil {
		state.bindGroup.Release()
	}
	if state.outputView != nil {
		state.outputView.Release()
	}
	if state.outputTexture != nil {
		state.outputTexture.Release()
	}
	if state.paramsBuffer != nil {
		state.paramsBuffer.Release()
	}
	if state.pipeline != nil {
		state.pipeline.Release()
	}
	if state.bgLayout != nil {
		state.bgLayout.Release()
	}
}

func (state *ssgiBlurPassState) recreateOutput(device *wgpu.Device, width, height uint32) error {
	if state.outputView != nil {
		state.outputView.Release()
	}
	if state.outputTexture != nil {
		state.outputTexture.Release()
	}
	tex, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "ssgi blurred",
		Size:          wgpu.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        ssgiFormat,
		Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
	})
	if err != nil {
		return fmt.Errorf("ssgi blur: output texture: %w", err)
	}
	view, err := tex.CreateView(nil)
	if err != nil {
		tex.Release()
		return fmt.Errorf("ssgi blur: output view: %w", err)
	}
	state.outputTexture = tex
	state.outputView = view
	return nil
}

func ecsSetSsgiResource(context *render.PassContext, view *wgpu.TextureView) {
	if view == nil {
		return
	}
	if resource, ok := ecs.Resource[SsgiResource](context.World); ok && resource != nil && resource.Result != nil {
		resource.Result.View = view
		return
	}
	ecs.SetResource(context.World, SsgiResource{Result: &SsgiResult{View: view}})
}
