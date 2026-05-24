
struct ViewProjOit {
    view_proj:       mat4x4<f32>,
    camera_position: vec4<f32>,
    z_scale:         f32,
    _pad0:           f32,
    _pad1:           f32,
    _pad2:           f32,
};

@group(0) @binding(0) var<uniform> view_proj_uniform: ViewProjOit;

struct SkinnedUniforms {
    light_direction: vec4<f32>,
    light_color:     vec4<f32>,
    ambient_color:   vec4<f32>,
};

@group(1) @binding(0) var<uniform> skinned_uniforms: SkinnedUniforms;
@group(1) @binding(1) var material_srgb_array: texture_2d_array<f32>;
@group(1) @binding(2) var material_sampler:    sampler;

struct SkinnedInstance {
    base_color:   vec4<f32>,
    entity_id:    u32,
    joint_offset: u32,
    base_layer:   u32,
    alpha_mode:   u32,
    transmission_factor:  f32,
    ior:                  f32,
    roughness_factor:     f32,
    metallic_factor:      f32,
    dispersion:           f32,
    thickness:            f32,
    attenuation_distance: f32,
    _pad0:                f32,
    attenuation_color:    vec3<f32>,
    _pad1:                f32,
};

@group(2) @binding(0) var<storage, read> joint_matrices: array<mat4x4<f32>>;
@group(2) @binding(1) var<storage, read> instances:      array<SkinnedInstance>;

@group(3) @binding(0) var prefiltered_env: texture_cube<f32>;
@group(3) @binding(1) var brdf_lut:        texture_2d<f32>;
@group(3) @binding(2) var ibl_sampler:     sampler;
@group(3) @binding(3) var transmission_color_texture: texture_2d<f32>;
@group(3) @binding(4) var transmission_color_sampler: sampler;

const MAX_REFLECTION_LOD: f32 = 4.0;

struct VertexInput {
    @location(0) position:      vec4<f32>,
    @location(1) normal:        vec4<f32>,
    @location(2) tangent:       vec4<f32>,
    @location(3) uv:            vec4<f32>,
    @location(4) color:         vec4<f32>,
    @location(5) joint_indices: vec4<u32>,
    @location(6) joint_weights: vec4<f32>,
};

struct VertexOutput {
    @builtin(position) clip_position: vec4<f32>,
    @location(0) world_normal: vec3<f32>,
    @location(1) color:        vec4<f32>,
    @location(2) uv:           vec2<f32>,
    @location(3) @interpolate(flat) entity_id: u32,
    @location(4) view_z:       f32,
    @location(5) @interpolate(flat) instance: u32,
    @location(6) world_position: vec3<f32>,
};

struct OitOutput {
    @location(0) accum:     vec4<f32>,
    @location(1) reveal:    f32,
    @location(2) entity_id: u32,
};

const NO_TEXTURE_LAYER: u32 = 0xFFFFFFFFu;

@vertex
fn vertex_main(input: VertexInput, @builtin(instance_index) instance_index: u32) -> VertexOutput {
    let position = vec4<f32>(input.position.xyz, 1.0);
    let normal = input.normal.xyz;
    let joint_offset = instances[instance_index].joint_offset;
    var skinned_position = vec3<f32>(0.0, 0.0, 0.0);
    var skinned_normal = vec3<f32>(0.0, 0.0, 0.0);
    for (var index = 0u; index < 4u; index = index + 1u) {
        let joint_index = input.joint_indices[index];
        let joint_weight = input.joint_weights[index];
        if (joint_weight > 0.0) {
            let joint_matrix = joint_matrices[joint_offset + joint_index];
            let transformed_pos = joint_matrix * position;
            skinned_position = skinned_position + transformed_pos.xyz * joint_weight;
            let normal_matrix = mat3x3<f32>(joint_matrix[0].xyz, joint_matrix[1].xyz, joint_matrix[2].xyz);
            skinned_normal = skinned_normal + (normal_matrix * normal) * joint_weight;
        }
    }
    if (length(skinned_normal) < 0.0001) {
        skinned_normal = vec3<f32>(0.0, 0.0, 1.0);
    } else {
        skinned_normal = normalize(skinned_normal);
    }

    let clip = view_proj_uniform.view_proj * vec4<f32>(skinned_position, 1.0);
    var out: VertexOutput;
    out.clip_position = clip;
    out.world_normal = skinned_normal;
    out.color = input.color;
    out.uv = input.uv.xy;
    out.entity_id = instances[instance_index].entity_id;
    out.view_z = max(clip.w, 0.0001);
    out.instance = instance_index;
    out.world_position = skinned_position;
    return out;
}

fn oit_weight(view_z: f32, a: f32) -> f32 {
    let z_scale = max(view_proj_uniform.z_scale, 1.0);
    let z_ratio = view_z / z_scale;
    return a * clamp(0.03 / (1e-5 + pow(z_ratio, 4.0)), 1e-2, 3e3);
}

fn apply_ior_to_roughness(roughness: f32, ior: f32) -> f32 {
    return roughness * clamp(ior * 2.0 - 2.0, 0.0, 1.0);
}

fn apply_volume_attenuation(radiance: vec3<f32>, transmission_distance: f32, attenuation_color: vec3<f32>, attenuation_distance: f32) -> vec3<f32> {
    if (attenuation_distance <= 0.0) {
        return radiance;
    }
    let attenuation_coefficient = -log(max(attenuation_color, vec3<f32>(0.0001))) / attenuation_distance;
    let transmittance = exp(-attenuation_coefficient * transmission_distance);
    return transmittance * radiance;
}

fn get_transmission_sample(reflection: vec3<f32>, roughness: f32, ior: f32) -> vec3<f32> {
    let transmission_roughness = apply_ior_to_roughness(roughness, ior);
    return textureSampleLevel(prefiltered_env, ibl_sampler, reflection, transmission_roughness * MAX_REFLECTION_LOD).rgb;
}

fn get_screen_space_transmission(world_pos: vec3<f32>, refracted_dir: vec3<f32>, thickness: f32, ior: f32, roughness: f32) -> vec3<f32> {
    let exit_world = world_pos + refracted_dir * max(thickness, 0.001);
    let exit_clip = view_proj_uniform.view_proj * vec4<f32>(exit_world, 1.0);
    let ibl_sample = get_transmission_sample(refracted_dir, roughness, ior);
    if (exit_clip.w <= 0.0001) {
        return ibl_sample;
    }
    let exit_ndc = exit_clip.xyz / exit_clip.w;
    let uv = vec2<f32>(exit_ndc.x * 0.5 + 0.5, 0.5 - exit_ndc.y * 0.5);
    let clamped_uv = clamp(uv, vec2<f32>(0.0), vec2<f32>(1.0));
    let scene_sample = textureSampleLevel(transmission_color_texture, transmission_color_sampler, clamped_uv, 0.0).rgb;
    let edge_dist = min(min(uv.x, uv.y), min(1.0 - uv.x, 1.0 - uv.y));
    let blend = clamp(edge_dist * 8.0, 0.0, 1.0);
    return mix(ibl_sample, scene_sample, blend);
}

fn refract_safe(view: vec3<f32>, normal: vec3<f32>, eta: f32) -> vec3<f32> {
    let refraction = refract(-view, normal, eta);
    let len_sq = dot(refraction, refraction);
    if (len_sq > 0.0001) {
        return refraction / sqrt(len_sq);
    }
    return -view;
}

fn get_ibl_volume_refraction(
    normal: vec3<f32>,
    view: vec3<f32>,
    world_pos: vec3<f32>,
    roughness: f32,
    base_color: vec3<f32>,
    f0: vec3<f32>,
    ior: f32,
    dispersion: f32,
    thickness: f32,
    attenuation_color: vec3<f32>,
    attenuation_distance: f32,
) -> vec3<f32> {
    let transmission_distance = thickness;
    var transmitted_light: vec3<f32>;
    if (dispersion > 0.0) {
        let half_spread = (ior - 1.0) * 0.025 * dispersion;
        let ior_r = ior - half_spread;
        let ior_b = ior + half_spread;
        let dir_r = refract_safe(view, normal, 1.0 / ior_r);
        let dir_g = refract_safe(view, normal, 1.0 / ior);
        let dir_b = refract_safe(view, normal, 1.0 / ior_b);
        let r = get_screen_space_transmission(world_pos, dir_r, transmission_distance, ior_r, roughness).r;
        let g = get_screen_space_transmission(world_pos, dir_g, transmission_distance, ior, roughness).g;
        let b = get_screen_space_transmission(world_pos, dir_b, transmission_distance, ior_b, roughness).b;
        transmitted_light = vec3<f32>(r, g, b);
    } else {
        let dir = refract_safe(view, normal, 1.0 / ior);
        transmitted_light = get_screen_space_transmission(world_pos, dir, transmission_distance, ior, roughness);
    }
    let attenuated_color = apply_volume_attenuation(transmitted_light, transmission_distance, attenuation_color, attenuation_distance);
    let n_dot_v = clamp(dot(normal, view), 0.001, 1.0);
    let brdf = textureSampleLevel(brdf_lut, ibl_sampler, vec2<f32>(n_dot_v, roughness), 0.0).rg;
    let specular_color = f0 * brdf.x + brdf.y;
    return (vec3<f32>(1.0) - specular_color) * attenuated_color * base_color;
}

const BLEND_OPAQUE_ALPHA_THRESHOLD: f32 = 0.99;

@fragment
fn fs_blend_opaque_prepass(in: VertexOutput) {
    let instance = instances[in.instance];
    if (instance.alpha_mode != 2u) {
        discard;
    }
    var alpha = instance.base_color.a * in.color.a;
    if (instance.base_layer != NO_TEXTURE_LAYER) {
        let layer = i32(instance.base_layer & 0xFFFFu);
        alpha = alpha * textureSampleLevel(material_srgb_array, material_sampler, in.uv, layer, 0.0).a;
    }
    if (alpha < BLEND_OPAQUE_ALPHA_THRESHOLD) {
        discard;
    }
}

@fragment
fn fragment_main(in: VertexOutput) -> OitOutput {
    let instance = instances[in.instance];
    if (instance.alpha_mode != 2u) {
        discard;
    }
    var base_color = instance.base_color * in.color;
    if (instance.base_layer != NO_TEXTURE_LAYER) {
        let layer = i32(instance.base_layer & 0xFFFFu);
        base_color = base_color * textureSampleLevel(material_srgb_array, material_sampler, in.uv, layer, 0.0);
    }
    let alpha = base_color.a;
    if (alpha < 0.0039) {
        discard;
    }
    let normal = normalize(in.world_normal);
    let light_dir = -normalize(skinned_uniforms.light_direction.xyz);
    let lambert = max(dot(normal, light_dir), 0.0);
    let lit = skinned_uniforms.ambient_color.rgb + skinned_uniforms.light_color.rgb * lambert;
    var albedo = base_color.rgb * lit;

    if (instance.transmission_factor > 0.0) {
        let view = normalize(view_proj_uniform.camera_position.xyz - in.world_position);
        let f0 = mix(vec3<f32>(0.04), base_color.rgb, instance.metallic_factor);
        let transmission = get_ibl_volume_refraction(
            normal, view, in.world_position, instance.roughness_factor, base_color.rgb, f0,
            instance.ior, instance.dispersion, instance.thickness, instance.attenuation_color, instance.attenuation_distance,
        );
        albedo = mix(albedo, transmission, instance.transmission_factor);
    }

    let w = oit_weight(in.view_z, alpha);
    var out: OitOutput;
    out.accum = vec4<f32>(albedo * alpha, alpha) * w;
    out.reveal = alpha;
    out.entity_id = in.entity_id;
    return out;
}
