// mesh_build_indirect: writes a single DrawIndirect command into
// the per-handle indirect buffer. Inputs come from a uniform
// supplied by the CPU. Today the inputs are CPU-computed; a
// future GPU-culling kernel can replace this shader with one that
// computes instance_count from per-instance visibility tests
// without touching the mesh render pass.

struct BuildParams {
    vertex_count:   u32,
    instance_count: u32,
    first_vertex:   u32,
    first_instance: u32,
};

struct IndirectCommand {
    vertex_count:   u32,
    instance_count: u32,
    first_vertex:   u32,
    first_instance: u32,
};

@group(0) @binding(0) var<uniform>             params:   BuildParams;
@group(0) @binding(1) var<storage, read_write> indirect: IndirectCommand;

@compute @workgroup_size(1, 1, 1)
fn main() {
    indirect.vertex_count   = params.vertex_count;
    indirect.instance_count = params.instance_count;
    indirect.first_vertex   = params.first_vertex;
    indirect.first_instance = params.first_instance;
}
