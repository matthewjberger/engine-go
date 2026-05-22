package pass

import (
	_ "embed"
	"fmt"
	"unsafe"

	"github.com/cogentcore/webgpu/wgpu"
)

//go:embed cluster_bounds.wgsl
var clusterBoundsShader string

//go:embed cluster_light_assign.wgsl
var clusterLightAssignShader string

// Cluster grid layout matches nightshade exactly: 16x9x24 = 3456
// clusters tiling the camera frustum, with up to 256 local lights
// per cluster. Directional lights live at the front of the lights
// buffer and skip cluster culling entirely (they're iterated by
// every fragment).
const (
	ClusterGridX        uint32 = 16
	ClusterGridY        uint32 = 9
	ClusterGridZ        uint32 = 24
	MaxLightsPerCluster uint32 = 256
	TotalClusters       uint32 = ClusterGridX * ClusterGridY * ClusterGridZ
)

// MaxLightsBuffer is the upper bound on lights uploaded into the
// global lights storage buffer each frame. Mirrors nightshade's
// MAX_LIGHTS constant.
const MaxLightsBuffer uint32 = 1024

// ClusterUniforms is the std140-equivalent uniform layout the
// cluster bounds + cluster light assign + mesh shaders all share.
// Field order and padding match the WGSL ClusterUniforms struct
// exactly so the driver reads without surprises.
type ClusterUniforms struct {
	InverseProjection    [16]float32 // 64
	ScreenSize           [2]float32  // 8
	ZNear                float32     // 4
	ZFar                 float32     // 4
	ClusterCount         [4]uint32   // 16
	TileSize             [2]float32  // 8
	NumLights            uint32      // 4
	NumDirectionalLights uint32      // 4
}

// ClusterBounds is the per-cluster view-space AABB the cluster
// bounds compute pass writes and the cluster light assign pass
// reads. 32 bytes per entry, TotalClusters entries in the buffer.
type ClusterBounds struct {
	MinPoint [4]float32
	MaxPoint [4]float32
}

// LightGrid holds the per-cluster count of intersecting local
// lights. Mirrors nightshade's LightGrid; offset is unused (the
// fragment shader reads light_indices[cluster_idx *
// MaxLightsPerCluster + i] for i in [0, count)) but kept so the
// WGSL struct layout matches verbatim.
type LightGrid struct {
	Offset uint32
	Count  uint32
}

// LightGPU is the std430 GPU layout of one entry in the lights
// storage buffer. 80 bytes; matches nightshade's LightData
// exactly so the cluster_light_assign shader can be lifted
// without remapping fields.
type LightGPU struct {
	Position    [4]float32
	Direction   [4]float32
	Color       [4]float32
	LightType   uint32
	Range       float32
	InnerCone   float32
	OuterCone   float32
	ShadowIndex int32
	LightSize   float32
	CookieLayer uint32
	Padding     float32
}

const LightGPUSize = uint64(unsafe.Sizeof(LightGPU{}))
const ClusterUniformsSize = uint64(unsafe.Sizeof(ClusterUniforms{}))
const ClusterBoundsSize = uint64(unsafe.Sizeof(ClusterBounds{}))
const LightGridSize = uint64(unsafe.Sizeof(LightGrid{}))

// clusterResources owns the GPU objects that the cluster compute
// + mesh draw share each frame: the two compute pipelines, the
// shared bind groups, and the storage / uniform buffers backing
// them. Constructed once at mesh-pass setup and destroyed when
// the mesh pass releases.
//
// Buffer responsibilities:
//
//   - clusterUniforms (uniform): rewritten each frame from the
//     active camera. Bound by both compute passes AND the mesh
//     fragment shader.
//   - viewMatrix (uniform): camera view rewritten each frame for
//     the light-assign pass (lights are in world space and the
//     culler transforms into view space per-light).
//   - clusterBounds (storage): output of cluster_bounds compute.
//     Persistent until the camera changes.
//   - lights (storage): the world's lights packed via
//     [packLights]. Directional lights first, then local lights.
//   - lightGrid (storage RW): per-cluster (offset, count). Reset
//     each frame via the precomputed zero buffer below.
//   - lightGridReset (copy_src): zero-filled snapshot used to
//     reset lightGrid before each light-assign dispatch.
//   - lightIndices (storage RW): flat per-cluster slot table the
//     light-assign pass writes into.
type clusterResources struct {
	boundsPipeline *wgpu.ComputePipeline
	assignPipeline *wgpu.ComputePipeline

	boundsBindGroupLayout *wgpu.BindGroupLayout
	assignBindGroupLayout *wgpu.BindGroupLayout

	boundsBindGroup *wgpu.BindGroup
	assignBindGroup *wgpu.BindGroup

	clusterUniforms *wgpu.Buffer
	viewMatrix      *wgpu.Buffer

	clusterBounds  *wgpu.Buffer
	lightGrid      *wgpu.Buffer
	lightGridReset *wgpu.Buffer
	lightIndices   *wgpu.Buffer
	lights         *wgpu.Buffer

	prevUniforms ClusterUniforms
	uniformsDirty bool
}

// newClusterResources allocates every cluster-side GPU buffer and
// builds the two compute pipelines + their bind groups. Returns
// the bundle for the mesh pass to own. Called once at NewMeshPass
// time.
func newClusterResources(device *wgpu.Device) (*clusterResources, error) {
	r := &clusterResources{}

	clusterUniforms, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "cluster uniforms",
		Size:  ClusterUniformsSize,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: uniforms buffer: %w", err)
	}
	r.clusterUniforms = clusterUniforms

	viewMatrix, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "cluster view matrix",
		Size:  64,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: view matrix buffer: %w", err)
	}
	r.viewMatrix = viewMatrix

	clusterBounds, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "cluster bounds",
		Size:  ClusterBoundsSize * uint64(TotalClusters),
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: bounds buffer: %w", err)
	}
	r.clusterBounds = clusterBounds

	lightGridSize := LightGridSize * uint64(TotalClusters)
	lightGrid, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "cluster light grid",
		Size:  lightGridSize,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: light grid buffer: %w", err)
	}
	r.lightGrid = lightGrid

	lightGridReset, err := device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "cluster light grid reset",
		Contents: make([]byte, lightGridSize),
		Usage:    wgpu.BufferUsageCopySrc,
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: light grid reset buffer: %w", err)
	}
	r.lightGridReset = lightGridReset

	lightIndices, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "cluster light indices",
		Size:  uint64(TotalClusters) * uint64(MaxLightsPerCluster) * 4,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: light indices buffer: %w", err)
	}
	r.lightIndices = lightIndices

	lightsBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "cluster lights",
		Size:  LightGPUSize * uint64(MaxLightsBuffer),
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: lights buffer: %w", err)
	}
	r.lights = lightsBuffer

	boundsLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "cluster bounds bind group layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeStorage},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: bounds bgl: %w", err)
	}
	r.boundsBindGroupLayout = boundsLayout

	assignLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "cluster light assign bind group layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage},
			},
			{
				Binding:    2,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeStorage},
			},
			{
				Binding:    3,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeStorage},
			},
			{
				Binding:    4,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage},
			},
			{
				Binding:    5,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: assign bgl: %w", err)
	}
	r.assignBindGroupLayout = assignLayout

	boundsShader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "cluster bounds shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: clusterBoundsShader},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: bounds shader: %w", err)
	}
	defer boundsShader.Release()

	boundsPipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "cluster bounds pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{boundsLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: bounds pipeline layout: %w", err)
	}
	defer boundsPipelineLayout.Release()

	boundsPipeline, err := device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
		Label:  "cluster bounds pipeline",
		Layout: boundsPipelineLayout,
		Compute: wgpu.ProgrammableStageDescriptor{
			Module:     boundsShader,
			EntryPoint: "main",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: bounds pipeline: %w", err)
	}
	r.boundsPipeline = boundsPipeline

	assignShader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "cluster light assign shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: clusterLightAssignShader},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: assign shader: %w", err)
	}
	defer assignShader.Release()

	assignPipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "cluster light assign pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{assignLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: assign pipeline layout: %w", err)
	}
	defer assignPipelineLayout.Release()

	assignPipeline, err := device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
		Label:  "cluster light assign pipeline",
		Layout: assignPipelineLayout,
		Compute: wgpu.ProgrammableStageDescriptor{
			Module:     assignShader,
			EntryPoint: "main",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: assign pipeline: %w", err)
	}
	r.assignPipeline = assignPipeline

	boundsBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "cluster bounds bind group",
		Layout: boundsLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: clusterUniforms, Offset: 0, Size: ClusterUniformsSize},
			{Binding: 1, Buffer: clusterBounds, Offset: 0, Size: wgpu.WholeSize},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: bounds bind group: %w", err)
	}
	r.boundsBindGroup = boundsBindGroup

	assignBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "cluster light assign bind group",
		Layout: assignLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: clusterUniforms, Offset: 0, Size: ClusterUniformsSize},
			{Binding: 1, Buffer: clusterBounds, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 2, Buffer: lightGrid, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 3, Buffer: lightIndices, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 4, Buffer: lightsBuffer, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 5, Buffer: viewMatrix, Offset: 0, Size: 64},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: assign bind group: %w", err)
	}
	r.assignBindGroup = assignBindGroup

	return r, nil
}

func (r *clusterResources) release() {
	if r.assignBindGroup != nil {
		r.assignBindGroup.Release()
	}
	if r.boundsBindGroup != nil {
		r.boundsBindGroup.Release()
	}
	if r.assignPipeline != nil {
		r.assignPipeline.Release()
	}
	if r.boundsPipeline != nil {
		r.boundsPipeline.Release()
	}
	if r.assignBindGroupLayout != nil {
		r.assignBindGroupLayout.Release()
	}
	if r.boundsBindGroupLayout != nil {
		r.boundsBindGroupLayout.Release()
	}
	if r.lights != nil {
		r.lights.Release()
	}
	if r.lightIndices != nil {
		r.lightIndices.Release()
	}
	if r.lightGridReset != nil {
		r.lightGridReset.Release()
	}
	if r.lightGrid != nil {
		r.lightGrid.Release()
	}
	if r.clusterBounds != nil {
		r.clusterBounds.Release()
	}
	if r.viewMatrix != nil {
		r.viewMatrix.Release()
	}
	if r.clusterUniforms != nil {
		r.clusterUniforms.Release()
	}
}
