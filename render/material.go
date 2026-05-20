package render

// Material is the per-entity surface-appearance component. Today it
// carries a base color that the mesh shader multiplies into the vertex
// color before lighting; future fields (metallic, roughness,
// emissive, alpha mode, normal map handle) slot in without changing
// the storage layout once the shader catches up.
//
// Entities without a Material component render with an implicit white
// base color, so adding the component is opt-in for per-instance
// tinting.
type Material struct {
	BaseColor [4]float32
}

// DefaultMaterial returns a fully-opaque white material. Equivalent
// to leaving the Material component off an entity entirely; useful
// when spawning code wants to spell out the default.
func DefaultMaterial() Material {
	return Material{BaseColor: [4]float32{1, 1, 1, 1}}
}

// materialDataUniform is the WGSL-aligned material entry layout the
// mesh pass packs into per-handle storage buffers. Kept separate from
// [Material] so future Material fields don't force a GPU-side layout
// change.
type materialDataUniform struct {
	BaseColor [4]float32
}

const materialDataSize = uint64(16)
