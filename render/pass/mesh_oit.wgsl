// Weighted-blended OIT accumulation pass (McGuire / Bavoil with
// the reference engine's depth-aware weight). Vertex stage is the
// same instanced mesh layout the opaque mesh pass uses (model +
// material id + entity id storage buffers per handle). Fragment
// stage shades with a basic lit color (no full PBR yet -- shadows
// + IBL on the OIT path can come later) and writes two targets:
//
//   accum  (Rgba16Float)  blend One+One       += w * vec4(c, 1)
//   reveal (R8Unorm)      blend Zero+OneMinus *= (1 - alpha)
//
// The composite pass then resolves `accum.rgb / accum.a` blended
// over scene_color with `1 - reveal` as the alpha.
//
// Materials that are NOT blend-mode are discarded so this shader
// only contributes to genuinely transparent geometry.

struct ViewProj {
    view_proj:       mat4x4<f32>,
    camera_position: vec4<f32>,
    camera_z_far:    f32,
    oit_z_scale:     f32,
    _pad0:           f32,
    _pad1:           f32,
};

struct DirectionalLight {
    direction: vec4<f32>,
    color:     vec4<f32>,
    ambient:   vec4<f32>,
};

struct Material {
    base_color:                vec4<f32>,
    emissive_factor:           vec3<f32>,
    alpha_mode:                u32,
    base_layer:                u32,
    emissive_layer:            u32,
    normal_layer:              u32,
    metallic_roughness_layer:  u32,
    occlusion_layer:           u32,
    normal_scale:              f32,
    occlusion_strength:        f32,
    metallic_factor:           f32,
    roughness_factor:          f32,
    alpha_cutoff:              f32,
    unlit:                     u32,
    ior:                       f32,
    emissive_strength:         f32,
    _pad0:                     f32,
    _pad1:                     f32,
    _pad2:                     f32,
};

@group(0) @binding(0) var<uniform>       view_proj_uniform: ViewProj;
@group(0) @binding(1) var<uniform>       directional:       DirectionalLight;
@group(0) @binding(2) var<storage, read> materials:         array<Material>;
@group(0) @binding(3) var                material_srgb_array: texture_2d_array<f32>;
@group(0) @binding(4) var                material_sampler:    sampler;

@group(1) @binding(0) var<storage, read> models:           array<mat4x4<f32>>;
@group(1) @binding(1) var<storage, read> material_indices: array<u32>;
@group(1) @binding(2) var<storage, read> entity_ids:       array<u32>;

struct VertexInput {
    @location(0) position: vec4<f32>,
    @location(1) normal:   vec4<f32>,
    @location(2) tangent:  vec4<f32>,
    @location(3) uv:       vec4<f32>,
    @location(4) color:    vec4<f32>,
};

struct VertexOutput {
    @builtin(position) clip_position: vec4<f32>,
    @location(0) world_position: vec3<f32>,
    @location(1) world_normal:   vec3<f32>,
    @location(2) uv:             vec2<f32>,
    @location(3) color:          vec4<f32>,
    @location(4) @interpolate(flat) material_index: u32,
    @location(5) @interpolate(flat) entity_id:      u32,
    @location(6) view_z:         f32,
};

struct OitOutput {
    @location(0) accum:  vec4<f32>,
    @location(1) reveal: f32,
};

const NO_TEXTURE_LAYER: u32 = 0xFFFFFFFFu;

@vertex
fn vertex_main(input: VertexInput, @builtin(instance_index) instance_index: u32) -> VertexOutput {
    let model = models[instance_index];
    let world_position = model * vec4<f32>(input.position.xyz, 1.0);
    let world_normal = normalize((model * vec4<f32>(input.normal.xyz, 0.0)).xyz);
    let clip = view_proj_uniform.view_proj * world_position;
    var out: VertexOutput;
    out.clip_position = clip;
    out.world_position = world_position.xyz;
    out.world_normal = world_normal;
    out.uv = input.uv.xy;
    out.color = input.color;
    out.material_index = material_indices[instance_index];
    out.entity_id = entity_ids[instance_index];
    // The cascade-uniform here drives the depth-aware weight in
    // fs_main. Linear positive depth (camera-forward distance).
    out.view_z = max(clip.w, 0.0001);
    return out;
}

// weight is the reference engine's depth-aware OIT weight: alpha
// times a clamp of 0.03 / (epsilon + (view_z / z_scale)^4). Higher
// values for closer fragments so they dominate the accumulation;
// the alpha factor zeros out fully-transparent fragments. The
// clamp prevents both underflow at the far plane and overflow at
// the near plane.
fn oit_weight(view_z: f32, a: f32) -> f32 {
    let z_scale = max(view_proj_uniform.oit_z_scale, 1.0);
    let z_ratio = view_z / z_scale;
    return a * clamp(0.03 / (1e-5 + pow(z_ratio, 4.0)), 1e-2, 3e3);
}

@fragment
fn fragment_main(in: VertexOutput) -> OitOutput {
    // Derivatives must be evaluated under uniform control flow,
    // before any non-uniform discard or material branch. Pass
    // them explicitly to textureSampleGrad so the sampler call
    // itself is allowed inside the per-material branch below.
    let ddx_uv = dpdx(in.uv);
    let ddy_uv = dpdy(in.uv);

    let mat = materials[in.material_index];
    // Skip non-blend materials so the OIT pass only contributes
    // genuine transparency. The opaque mesh pass renders them.
    if (mat.alpha_mode != 2u) {
        discard;
    }

    var base_color = mat.base_color * in.color;
    if (mat.base_layer != NO_TEXTURE_LAYER) {
        let layer = i32(mat.base_layer & 0xFFFFu);
        base_color = base_color * textureSampleGrad(material_srgb_array, material_sampler, in.uv, layer, ddx_uv, ddy_uv);
    }

    var albedo = base_color.rgb;
    let alpha = base_color.a;
    if (alpha < 0.0039) {
        discard;
    }

    if (mat.unlit == 0u) {
        let normal = normalize(in.world_normal);
        let light_dir = -normalize(directional.direction.xyz);
        let lambert = max(dot(normal, light_dir), 0.0);
        albedo = albedo * (directional.ambient.rgb + directional.color.rgb * lambert);
    }
    albedo = albedo + mat.emissive_factor * mat.emissive_strength;

    let w = oit_weight(in.view_z, alpha);

    var out: OitOutput;
    out.accum = vec4<f32>(albedo * alpha, alpha) * w;
    out.reveal = alpha;
    return out;
}
