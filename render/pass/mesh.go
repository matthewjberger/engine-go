package pass

import (
	_ "embed"
	"fmt"
	"math"
	"sort"
	"unsafe"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/go-gl/mathgl/mgl32"

	"indigo/ecs"
	"indigo/render"
	"indigo/render/asset"
	"indigo/transform"
)

//go:embed mesh.wgsl
var meshShader string

// handleInstances owns the GPU and CPU bookkeeping for one mesh
// handle in the mesh pass. Each handle holds three storage buffers
// indexed by per-instance slot: world matrices, MaterialGPU
// entries, and entity IDs. The same slot stays with an entity for
// its whole lifetime so sparse uploads can write to known offsets.
type handleInstances struct {
	modelBuffer    *wgpu.Buffer
	materialBuffer *wgpu.Buffer
	entityIdBuffer *wgpu.Buffer
	bindGroup      *wgpu.BindGroup
	capacity       uint32

	entityToSlot map[ecs.Entity]uint32
	slotEntity   []ecs.Entity
}

func releaseHandleInstances(h *handleInstances) {
	if h.bindGroup != nil {
		h.bindGroup.Release()
		h.bindGroup = nil
	}
	if h.modelBuffer != nil {
		h.modelBuffer.Release()
		h.modelBuffer = nil
	}
	if h.materialBuffer != nil {
		h.materialBuffer.Release()
		h.materialBuffer = nil
	}
	if h.entityIdBuffer != nil {
		h.entityIdBuffer.Release()
		h.entityIdBuffer = nil
	}
}

// meshPassState is the long-lived state the mesh pass keeps.
//
//   - viewProjBindGroup (group 0): view × projection uniform.
//   - globalBindGroup   (group 1): clustered-lighting bindings
//     (lights / light_grid / light_indices / cluster_uniforms /
//     view_matrix) + sRGB / linear texture array views + the
//     shared sampler. The cluster compute passes write into the
//     same lights / light_grid / light_indices buffers from
//     [clusterResources], so the read-only views the mesh shader
//     binds here are consistent the moment the compute passes
//     finish.
//   - per-handle bind group (group 2): models / materials /
//     entity_ids storage buffers for that handle's instances.
type meshPassState struct {
	pipeline       *wgpu.RenderPipeline
	viewProjLayout *wgpu.BindGroupLayout
	globalBgLayout *wgpu.BindGroupLayout
	handleBgLayout *wgpu.BindGroupLayout

	viewProjBuffer    *wgpu.Buffer
	viewProjBindGroup *wgpu.BindGroup

	globalBindGroup *wgpu.BindGroup

	clusters *clusterResources

	perHandle     map[asset.MeshHandle]*handleInstances
	entityHandle  map[ecs.Entity]asset.MeshHandle
	sortedHandles []asset.MeshHandle

	aspectFn func() float32

	// clusterUniformsScratch is reused across frames to avoid
	// reallocating the per-frame ClusterUniforms upload value.
	clusterUniformsScratch ClusterUniforms
	lightScratch           []LightGPU
	prevScreenW            uint32
	prevScreenH            uint32
}

// NewMeshPass builds the engine's instanced PBR mesh pass.
//
// View × projection lives in a small uniform updated every frame;
// per-entity model matrices, materials, and entity IDs live in
// per-handle storage buffers, sparse-updated via [ecs.IterChanged]
// on the respective components. Each entity gets a stable slot in
// its handle's buffers so the GPU side only writes the entries
// that changed this frame. Materials are sampled from the global
// [asset.MaterialTextureArrays] resource, bound once at pass setup.
func NewMeshPass(device *wgpu.Device, surfaceFormat wgpu.TextureFormat, aspect func() float32, arrays *asset.MaterialTextureArrays) (*render.Pass, error) {
	state := &meshPassState{
		perHandle:    make(map[asset.MeshHandle]*handleInstances, 4),
		entityHandle: make(map[ecs.Entity]asset.MeshHandle, 64),
		aspectFn:     aspect,
	}

	viewProjLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "mesh view_proj bind group layout",
		Entries: []wgpu.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: wgpu.ShaderStageVertex,
			Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("mesh pass: view_proj bind group layout: %w", err)
	}
	state.viewProjLayout = viewProjLayout

	globalBgLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "mesh global bind group layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageFragment,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageFragment,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage},
			},
			{
				Binding:    2,
				Visibility: wgpu.ShaderStageFragment,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage},
			},
			{
				Binding:    3,
				Visibility: wgpu.ShaderStageFragment,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform},
			},
			{
				Binding:    4,
				Visibility: wgpu.ShaderStageFragment,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform},
			},
			{
				Binding:    5,
				Visibility: wgpu.ShaderStageFragment,
				Texture: wgpu.TextureBindingLayout{
					SampleType:    wgpu.TextureSampleTypeFloat,
					ViewDimension: wgpu.TextureViewDimension2DArray,
				},
			},
			{
				Binding:    6,
				Visibility: wgpu.ShaderStageFragment,
				Texture: wgpu.TextureBindingLayout{
					SampleType:    wgpu.TextureSampleTypeFloat,
					ViewDimension: wgpu.TextureViewDimension2DArray,
				},
			},
			{
				Binding:    7,
				Visibility: wgpu.ShaderStageFragment,
				Sampler:    wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mesh pass: global bind group layout: %w", err)
	}
	state.globalBgLayout = globalBgLayout

	handleBgLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "mesh per-handle bind group layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageVertex,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageVertex | wgpu.ShaderStageFragment,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage},
			},
			{
				Binding:    2,
				Visibility: wgpu.ShaderStageVertex,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mesh pass: per-handle bind group layout: %w", err)
	}
	state.handleBgLayout = handleBgLayout

	viewProjBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "mesh view_proj buffer",
		Size:  uint64(unsafe.Sizeof(mgl32.Mat4{})),
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("mesh pass: view_proj buffer: %w", err)
	}
	state.viewProjBuffer = viewProjBuffer

	viewProjBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "mesh view_proj bind group",
		Layout: viewProjLayout,
		Entries: []wgpu.BindGroupEntry{{
			Binding: 0,
			Buffer:  viewProjBuffer,
			Offset:  0,
			Size:    uint64(unsafe.Sizeof(mgl32.Mat4{})),
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("mesh pass: view_proj bind group: %w", err)
	}
	state.viewProjBindGroup = viewProjBindGroup

	clusters, err := newClusterResources(device)
	if err != nil {
		return nil, err
	}
	state.clusters = clusters

	globalBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "mesh global bind group",
		Layout: globalBgLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: clusters.lights, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 1, Buffer: clusters.lightGrid, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 2, Buffer: clusters.lightIndices, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 3, Buffer: clusters.clusterUniforms, Offset: 0, Size: ClusterUniformsSize},
			{Binding: 4, Buffer: clusters.viewMatrix, Offset: 0, Size: 64},
			{Binding: 5, TextureView: arrays.SRGBView},
			{Binding: 6, TextureView: arrays.LinearView},
			{Binding: 7, Sampler: arrays.Sampler},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mesh pass: global bind group: %w", err)
	}
	state.globalBindGroup = globalBindGroup

	shader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "mesh shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: meshShader},
	})
	if err != nil {
		return nil, fmt.Errorf("mesh pass: shader: %w", err)
	}
	defer shader.Release()

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "mesh pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{viewProjLayout, globalBgLayout, handleBgLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("mesh pass: pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()

	pipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "mesh pipeline",
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     shader,
			EntryPoint: "vertex_main",
			Buffers: []wgpu.VertexBufferLayout{{
				ArrayStride: uint64(unsafe.Sizeof(asset.MeshVertex{})),
				StepMode:    wgpu.VertexStepModeVertex,
				Attributes: []wgpu.VertexAttribute{
					{Format: wgpu.VertexFormatFloat32x4, Offset: 0, ShaderLocation: 0},
					{Format: wgpu.VertexFormatFloat32x4, Offset: 16, ShaderLocation: 1},
					{Format: wgpu.VertexFormatFloat32x4, Offset: 32, ShaderLocation: 2},
					{Format: wgpu.VertexFormatFloat32x4, Offset: 48, ShaderLocation: 3},
					{Format: wgpu.VertexFormatFloat32x4, Offset: 64, ShaderLocation: 4},
				},
			}},
		},
		Primitive: wgpu.PrimitiveState{
			Topology:  wgpu.PrimitiveTopologyTriangleList,
			FrontFace: wgpu.FrontFaceCCW,
			CullMode:  wgpu.CullModeNone,
		},
		DepthStencil: &wgpu.DepthStencilState{
			Format:            render.DepthFormat,
			DepthWriteEnabled: true,
			DepthCompare:      wgpu.CompareFunctionLess,
			StencilFront: wgpu.StencilFaceState{
				Compare:     wgpu.CompareFunctionAlways,
				FailOp:      wgpu.StencilOperationKeep,
				DepthFailOp: wgpu.StencilOperationKeep,
				PassOp:      wgpu.StencilOperationKeep,
			},
			StencilBack: wgpu.StencilFaceState{
				Compare:     wgpu.CompareFunctionAlways,
				FailOp:      wgpu.StencilOperationKeep,
				DepthFailOp: wgpu.StencilOperationKeep,
				PassOp:      wgpu.StencilOperationKeep,
			},
		},
		Multisample: wgpu.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     shader,
			EntryPoint: "fragment_main",
			Targets: []wgpu.ColorTargetState{
				{
					Format:    surfaceFormat,
					WriteMask: wgpu.ColorWriteMaskAll,
				},
				{
					Format:    render.EntityIdFormat,
					WriteMask: wgpu.ColorWriteMaskAll,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mesh pass: pipeline: %w", err)
	}
	state.pipeline = pipeline

	return &render.Pass{
		Name:    "mesh",
		Writes:  []string{"color", "depth", "entity_id"},
		State:   state,
		Prepare: meshPrepare,
		Execute: meshExecute,
		Release: meshRelease,
	}, nil
}

// meshPrepare runs every frame:
//  1. Write camera view × projection into the uniform.
//  2. Drain EntityDespawned events to free per-handle slots.
//  3. Allocate slots for entities whose RenderMesh changed.
//  4. Sparse-upload only the entities whose GlobalTransform or
//     Material was stamped this tick.
//  5. Rebuild the sorted handle list so the draw order is stable.
func meshPrepare(s any, context *render.PassContext) error {
	state := s.(*meshPassState)

	camera := ecs.MustResource[render.Camera](context.World)
	aspect := state.aspectFn()
	viewProjection := render.CameraViewProjection(camera, aspect)
	writeBuffer(context.Device, context.Queue, context.Encoder, state.viewProjBuffer, 0, bytesOf(&viewProjection))

	view := render.CameraView(camera)
	writeBuffer(context.Device, context.Queue, context.Encoder, state.clusters.viewMatrix, 0, bytesOf(&view))

	state.lightScratch = extractLights(context.World, state.lightScratch[:0])
	uploadLights(context.Device, context.Queue, context.Encoder, state.clusters.lights, state.lightScratch)
	numDirectional, numLocal := splitDirectionalAndLocal(state.lightScratch)

	uniforms := buildClusterUniforms(camera, aspect, context, numDirectional, numLocal)
	if uniforms != state.clusterUniformsScratch {
		state.clusterUniformsScratch = uniforms
		state.clusters.uniformsDirty = true
		state.clusters.prevUniforms = uniforms
	}
	writeBuffer(context.Device, context.Queue, context.Encoder, state.clusters.clusterUniforms, 0, bytesOf(&uniforms))

	for _, event := range ecs.DrainEvents[ecs.EntityDespawned](context.World) {
		releaseEntitySlot(state, context, event.Entity)
	}

	globalMask := ecs.MustMaskOf[transform.GlobalTransform](context.World)
	renderMeshMask := ecs.MustMaskOf[asset.RenderMesh](context.World)

	ecs.IterChanged1[asset.RenderMesh](
		context.World,
		globalMask,
		0,
		func(entity ecs.Entity, mesh *asset.RenderMesh) {
			if existing, already := state.entityHandle[entity]; already {
				if existing == mesh.Mesh {
					return
				}
				releaseEntitySlot(state, context, entity)
			}
			global, ok := ecs.Get[transform.GlobalTransform](context.World, entity)
			if !ok {
				return
			}
			handle := mesh.Mesh
			bucket, ok := state.perHandle[handle]
			if !ok {
				bucket = &handleInstances{
					entityToSlot: make(map[ecs.Entity]uint32, 16),
				}
				state.perHandle[handle] = bucket
			}
			slot := uint32(len(bucket.slotEntity))
			bucket.entityToSlot[entity] = slot
			bucket.slotEntity = append(bucket.slotEntity, entity)
			state.entityHandle[entity] = handle
			if err := ensureHandleCapacity(bucket, context.Device, state.handleBgLayout); err != nil {
				return
			}
			writeBuffer(context.Device, context.Queue, context.Encoder, bucket.modelBuffer, uint64(slot)*matrixSize, bytesOf(&global.Matrix))

			material := asset.DefaultMaterial().ToGPU()
			if mat, ok := ecs.Get[asset.Material](context.World, entity); ok {
				material = mat.ToGPU()
			}
			writeBuffer(context.Device, context.Queue, context.Encoder, bucket.materialBuffer, uint64(slot)*asset.MaterialGPUSize, bytesOf(&material))

			entityID := entity.ID
			writeBuffer(context.Device, context.Queue, context.Encoder, bucket.entityIdBuffer, uint64(slot)*4, bytesOf(&entityID))
		},
	)

	ecs.IterChanged1[transform.GlobalTransform](
		context.World,
		renderMeshMask,
		0,
		func(entity ecs.Entity, global *transform.GlobalTransform) {
			mesh, ok := ecs.Get[asset.RenderMesh](context.World, entity)
			if !ok {
				return
			}
			bucket, ok := state.perHandle[mesh.Mesh]
			if !ok {
				return
			}
			slot, ok := bucket.entityToSlot[entity]
			if !ok {
				return
			}
			writeBuffer(context.Device, context.Queue, context.Encoder, bucket.modelBuffer, uint64(slot)*matrixSize, bytesOf(&global.Matrix))
		},
	)

	ecs.IterChanged1[asset.Material](
		context.World,
		renderMeshMask,
		0,
		func(entity ecs.Entity, material *asset.Material) {
			mesh, ok := ecs.Get[asset.RenderMesh](context.World, entity)
			if !ok {
				return
			}
			bucket, ok := state.perHandle[mesh.Mesh]
			if !ok {
				return
			}
			slot, ok := bucket.entityToSlot[entity]
			if !ok {
				return
			}
			data := material.ToGPU()
			writeBuffer(context.Device, context.Queue, context.Encoder, bucket.materialBuffer, uint64(slot)*asset.MaterialGPUSize, bytesOf(&data))
		},
	)

	state.sortedHandles = state.sortedHandles[:0]
	for handle, bucket := range state.perHandle {
		if len(bucket.slotEntity) == 0 {
			continue
		}
		state.sortedHandles = append(state.sortedHandles, handle)
	}
	sort.Slice(state.sortedHandles, func(i, j int) bool {
		return state.sortedHandles[i] < state.sortedHandles[j]
	})

	return nil
}

func meshExecute(s any, context *render.PassContext) error {
	state := s.(*meshPassState)

	dispatchClusterPasses(state, context)

	if len(state.sortedHandles) == 0 {
		return nil
	}

	assets := ecs.MustResource[asset.MeshAssetsResource](context.World).Assets

	colorAttachment, err := context.ColorAttachment("color")
	if err != nil {
		return err
	}
	entityIdAttachment, err := context.ColorAttachment("entity_id")
	if err != nil {
		return err
	}
	depthAttachment, err := context.DepthAttachment("depth")
	if err != nil {
		return err
	}

	pass := context.Encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label:                  "mesh",
		ColorAttachments:       []wgpu.RenderPassColorAttachment{colorAttachment, entityIdAttachment},
		DepthStencilAttachment: &depthAttachment,
	})
	pass.SetPipeline(state.pipeline)
	pass.SetBindGroup(0, state.viewProjBindGroup, nil)
	pass.SetBindGroup(1, state.globalBindGroup, nil)

	for _, handle := range state.sortedHandles {
		bucket := state.perHandle[handle]
		entry, ok := assets.Lookup(handle)
		if !ok {
			continue
		}
		pass.SetBindGroup(2, bucket.bindGroup, nil)
		pass.SetVertexBuffer(0, entry.Vertices, 0, wgpu.WholeSize)
		pass.Draw(entry.VertexCount, uint32(len(bucket.slotEntity)), 0, 0)
	}

	pass.End()
	pass.Release()
	return nil
}

func meshRelease(s any) {
	state := s.(*meshPassState)
	for _, h := range state.perHandle {
		releaseHandleInstances(h)
	}
	if state.globalBindGroup != nil {
		state.globalBindGroup.Release()
	}
	if state.clusters != nil {
		state.clusters.release()
	}
	if state.viewProjBindGroup != nil {
		state.viewProjBindGroup.Release()
	}
	if state.viewProjBuffer != nil {
		state.viewProjBuffer.Release()
	}
	if state.handleBgLayout != nil {
		state.handleBgLayout.Release()
	}
	if state.globalBgLayout != nil {
		state.globalBgLayout.Release()
	}
	if state.viewProjLayout != nil {
		state.viewProjLayout.Release()
	}
	if state.pipeline != nil {
		state.pipeline.Release()
	}
}

// extractLights walks the engine world for entities with both
// [render.Light] and [transform.GlobalTransform], packs them into
// the [LightGPU] layout the cluster compute + mesh shaders share,
// and returns directional lights first followed by local
// (point/spot) lights. The cluster_light_assign pass culls only
// the local-light suffix; directional lights are iterated by
// every fragment regardless of position.
//
// Color is premultiplied by intensity so the shader doesn't have
// to multiply at sample time (matches nightshade's projection.rs
// packing).
func extractLights(world *ecs.World, scratch []LightGPU) []LightGPU {
	out := scratch
	out = out[:0]
	lightMask := ecs.MustMaskOf[render.Light](world)
	globalMask := ecs.MustMaskOf[transform.GlobalTransform](world)
	world.ForEach(lightMask|globalMask, 0, func(_ ecs.Entity, table *ecs.Archetype, index int) {
		if uint32(len(out)) >= MaxLightsBuffer {
			return
		}
		lights, _ := ecs.Column[render.Light](world, table)
		globals, _ := ecs.Column[transform.GlobalTransform](world, table)
		light := &lights[index]
		matrix := globals[index].Matrix
		out = append(out, LightGPU{
			Position:    [4]float32{matrix[12], matrix[13], matrix[14], 1.0},
			Direction:   [4]float32{-matrix[8], -matrix[9], -matrix[10], 0.0},
			Color:       [4]float32{light.Color[0] * light.Intensity, light.Color[1] * light.Intensity, light.Color[2] * light.Intensity, 1.0},
			LightType:   uint32(light.Type),
			Range:       light.Range,
			InnerCone:   float32(cosOrZero(light.InnerConeAngle)),
			OuterCone:   float32(cosOrZero(light.OuterConeAngle)),
			ShadowIndex: -1,
			LightSize:   0,
			CookieLayer: 0xFFFFFFFF,
			Padding:     0,
		})
	})
	sortDirectionalFirst(out)
	return out
}

func cosOrZero(angle float32) float32 {
	if angle <= 0 {
		return 0
	}
	return float32(math.Cos(float64(angle)))
}

// sortDirectionalFirst stable-partitions the slice so every
// directional light precedes every point/spot light. The cluster
// shader assumes this layout (num_directional_lights is the
// boundary index into the same buffer).
func sortDirectionalFirst(lights []LightGPU) {
	left := 0
	for i := range lights {
		if lights[i].LightType == uint32(render.LightTypeDirectional) {
			if i != left {
				lights[left], lights[i] = lights[i], lights[left]
			}
			left++
		}
	}
}

func splitDirectionalAndLocal(lights []LightGPU) (directional, local uint32) {
	for _, l := range lights {
		if l.LightType == uint32(render.LightTypeDirectional) {
			directional++
		}
	}
	local = uint32(len(lights)) - directional
	return
}

func uploadLights(device *wgpu.Device, queue *wgpu.Queue, encoder *wgpu.CommandEncoder, buffer *wgpu.Buffer, lights []LightGPU) {
	if len(lights) == 0 {
		return
	}
	writeBuffer(device, queue, encoder, buffer, 0, lightsAsBytes(lights))
}

func lightsAsBytes(lights []LightGPU) []byte {
	if len(lights) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&lights[0])), int(LightGPUSize)*len(lights))
}

// buildClusterUniforms snapshots the camera into the WGSL
// ClusterUniforms layout. The compute and fragment shaders read
// this every frame to size tiles and clamp depth slices.
func buildClusterUniforms(camera *render.Camera, aspect float32, context *render.PassContext, numDirectional, numLocal uint32) ClusterUniforms {
	proj := render.CameraProjection(camera, aspect)
	invProj := proj.Inv()
	w, h := framebufferSize(context)
	screenW := float32(w)
	screenH := float32(h)
	if screenW <= 0 {
		screenW = 1
	}
	if screenH <= 0 {
		screenH = 1
	}
	tileX := screenW / float32(ClusterGridX)
	tileY := screenH / float32(ClusterGridY)
	zNear := camera.Near
	zFar := camera.Far
	if zNear <= 0 {
		zNear = 0.1
	}
	if zFar <= zNear {
		zFar = zNear + 1.0
	}
	var u ClusterUniforms
	copy(u.InverseProjection[:], invProj[:])
	u.ScreenSize = [2]float32{screenW, screenH}
	u.ZNear = zNear
	u.ZFar = zFar
	u.ClusterCount = [4]uint32{ClusterGridX, ClusterGridY, ClusterGridZ, 0}
	u.TileSize = [2]float32{tileX, tileY}
	u.NumLights = numDirectional + numLocal
	u.NumDirectionalLights = numDirectional
	return u
}

// framebufferSize returns the renderer's current swapchain
// dimensions so the cluster grid tracks viewport resizes.
func framebufferSize(context *render.PassContext) (uint32, uint32) {
	renderer := ecs.MustResource[render.RendererResource](context.World).Renderer
	return renderer.Config.Width, renderer.Config.Height
}

// dispatchClusterPasses runs the two compute pipelines that
// rebuild the cluster grid and assign lights to each cluster.
// Called from [meshExecute] BEFORE BeginRenderPass — compute
// dispatches can't share an encoder pass with a render pass.
//
//  1. cluster_bounds: writes per-cluster view-space AABBs.
//     Re-dispatched every frame for simplicity; nightshade gates
//     this on a "camera changed" flag, but the cost of running
//     8x8x24 = 1536 invocations is trivial.
//  2. copy lightGridReset -> lightGrid (zeros every count).
//  3. cluster_light_assign: per cluster, tests every local light
//     against the AABB and writes intersecting indices to
//     light_indices + light_grid[cluster].count.
func dispatchClusterPasses(state *meshPassState, context *render.PassContext) {
	dispatchX := (ClusterGridX + 7) / 8
	dispatchY := (ClusterGridY + 7) / 8
	dispatchZ := ClusterGridZ

	boundsPass := context.Encoder.BeginComputePass(&wgpu.ComputePassDescriptor{
		Label: "cluster bounds",
	})
	boundsPass.SetPipeline(state.clusters.boundsPipeline)
	boundsPass.SetBindGroup(0, state.clusters.boundsBindGroup, nil)
	boundsPass.DispatchWorkgroups(dispatchX, dispatchY, dispatchZ)
	boundsPass.End()
	boundsPass.Release()

	lightGridBytes := LightGridSize * uint64(TotalClusters)
	context.Encoder.CopyBufferToBuffer(state.clusters.lightGridReset, 0, state.clusters.lightGrid, 0, lightGridBytes)

	assignPass := context.Encoder.BeginComputePass(&wgpu.ComputePassDescriptor{
		Label: "cluster light assign",
	})
	assignPass.SetPipeline(state.clusters.assignPipeline)
	assignPass.SetBindGroup(0, state.clusters.assignBindGroup, nil)
	assignPass.DispatchWorkgroups(dispatchX, dispatchY, dispatchZ)
	assignPass.End()
	assignPass.Release()
}

// releaseEntitySlot is the despawn handler: swap-remove the
// entity's slot in its handle, then rewrite the moved tail
// entity's data at its new slot so subsequent draws don't read
// stale matrices / materials.
func releaseEntitySlot(state *meshPassState, context *render.PassContext, entity ecs.Entity) {
	handle, ok := state.entityHandle[entity]
	if !ok {
		return
	}
	delete(state.entityHandle, entity)
	bucket, ok := state.perHandle[handle]
	if !ok {
		return
	}
	slot, ok := bucket.entityToSlot[entity]
	if !ok {
		return
	}
	last := uint32(len(bucket.slotEntity) - 1)
	if slot != last {
		moved := bucket.slotEntity[last]
		bucket.slotEntity[slot] = moved
		bucket.entityToSlot[moved] = slot
		if global, ok := ecs.Get[transform.GlobalTransform](context.World, moved); ok {
			writeBuffer(context.Device, context.Queue, context.Encoder, bucket.modelBuffer, uint64(slot)*matrixSize, bytesOf(&global.Matrix))
		}
		material := asset.DefaultMaterial().ToGPU()
		if mat, ok := ecs.Get[asset.Material](context.World, moved); ok {
			material = mat.ToGPU()
		}
		writeBuffer(context.Device, context.Queue, context.Encoder, bucket.materialBuffer, uint64(slot)*asset.MaterialGPUSize, bytesOf(&material))

		movedID := moved.ID
		writeBuffer(context.Device, context.Queue, context.Encoder, bucket.entityIdBuffer, uint64(slot)*4, bytesOf(&movedID))
	}
	bucket.slotEntity = bucket.slotEntity[:last]
	delete(bucket.entityToSlot, entity)
}

// matrixSize is the byte size of a single mat4. Used for offset
// arithmetic in sparse uploads.
const matrixSize uint64 = uint64(unsafe.Sizeof(mgl32.Mat4{}))

// minHandleCapacity is the starting capacity for a handle's
// instance buffers. The buffers double on growth.
const minHandleCapacity uint32 = 64

// ensureHandleCapacity grows the handle's three storage buffers
// and rebuilds its bind group when the slot count exceeds the
// current capacity. Existing contents aren't preserved on grow —
// subsequent IterChanged passes refresh whatever changed this
// frame and the slot-stable layout ensures other entries get
// reuploaded next time their components stamp.
func ensureHandleCapacity(h *handleInstances, device *wgpu.Device, layout *wgpu.BindGroupLayout) error {
	required := uint32(len(h.slotEntity))
	if h.capacity >= required && h.modelBuffer != nil && h.materialBuffer != nil && h.entityIdBuffer != nil {
		return nil
	}
	newCapacity := h.capacity
	if newCapacity == 0 {
		newCapacity = minHandleCapacity
	}
	for newCapacity < required {
		newCapacity *= 2
	}
	if newCapacity < minHandleCapacity {
		newCapacity = minHandleCapacity
	}

	modelBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "mesh model buffer",
		Size:  uint64(newCapacity) * matrixSize,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("mesh pass: model buffer: %w", err)
	}
	materialBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "mesh material buffer",
		Size:  uint64(newCapacity) * asset.MaterialGPUSize,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		modelBuffer.Release()
		return fmt.Errorf("mesh pass: material buffer: %w", err)
	}
	entityIdBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "mesh entity_id buffer",
		Size:  uint64(newCapacity) * 4,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		modelBuffer.Release()
		materialBuffer.Release()
		return fmt.Errorf("mesh pass: entity_id buffer: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "mesh per-handle bind group",
		Layout: layout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: modelBuffer, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 1, Buffer: materialBuffer, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 2, Buffer: entityIdBuffer, Offset: 0, Size: wgpu.WholeSize},
		},
	})
	if err != nil {
		modelBuffer.Release()
		materialBuffer.Release()
		entityIdBuffer.Release()
		return fmt.Errorf("mesh pass: per-handle bind group: %w", err)
	}
	if h.bindGroup != nil {
		h.bindGroup.Release()
	}
	if h.modelBuffer != nil {
		h.modelBuffer.Release()
	}
	if h.materialBuffer != nil {
		h.materialBuffer.Release()
	}
	if h.entityIdBuffer != nil {
		h.entityIdBuffer.Release()
	}
	h.modelBuffer = modelBuffer
	h.materialBuffer = materialBuffer
	h.entityIdBuffer = entityIdBuffer
	h.bindGroup = bindGroup
	h.capacity = newCapacity
	return nil
}
