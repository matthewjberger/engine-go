package pass

import (
	_ "embed"
	"fmt"
	"unsafe"

	"github.com/cogentcore/webgpu/wgpu"
)

//go:embed mesh_build_indirect.wgsl
var meshBuildIndirectShader string

// buildIndirectParams mirrors the WGSL BuildParams struct: a single
// indirect command's four u32s, supplied via a uniform buffer.
type buildIndirectParams struct {
	VertexCount   uint32
	InstanceCount uint32
	FirstVertex   uint32
	FirstInstance uint32
}

// buildIndirectPipeline owns the compute pipeline that writes a
// DrawIndirect command into a per-handle indirect storage buffer.
// One pipeline + one bind-group layout are shared across handles;
// each handle owns its own uniform buffer + bind group so the
// mesh pass can dispatch one compute per handle per frame without
// any rebinding churn between dispatches.
type buildIndirectPipeline struct {
	pipeline        *wgpu.ComputePipeline
	bindGroupLayout *wgpu.BindGroupLayout
}

func newBuildIndirectPipeline(device *wgpu.Device) (*buildIndirectPipeline, error) {
	bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "mesh build_indirect bind group layout",
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
		return nil, fmt.Errorf("mesh pass: build_indirect bind group layout: %w", err)
	}

	shader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "mesh build_indirect shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: meshBuildIndirectShader},
	})
	if err != nil {
		bindGroupLayout.Release()
		return nil, fmt.Errorf("mesh pass: build_indirect shader: %w", err)
	}
	defer shader.Release()

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "mesh build_indirect pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		bindGroupLayout.Release()
		return nil, fmt.Errorf("mesh pass: build_indirect pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()

	pipeline, err := device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
		Layout: pipelineLayout,
		Compute: wgpu.ProgrammableStageDescriptor{
			Module:     shader,
			EntryPoint: "main",
		},
	})
	if err != nil {
		bindGroupLayout.Release()
		return nil, fmt.Errorf("mesh pass: build_indirect pipeline: %w", err)
	}

	return &buildIndirectPipeline{
		pipeline:        pipeline,
		bindGroupLayout: bindGroupLayout,
	}, nil
}

func (b *buildIndirectPipeline) release() {
	if b.pipeline != nil {
		b.pipeline.Release()
	}
	if b.bindGroupLayout != nil {
		b.bindGroupLayout.Release()
	}
}

const buildIndirectParamsSize = uint64(unsafe.Sizeof(buildIndirectParams{}))
