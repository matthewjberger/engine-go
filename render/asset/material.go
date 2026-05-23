package asset

type AlphaMode uint8

const (
	AlphaModeOpaque AlphaMode = iota
	AlphaModeMask
	AlphaModeBlend
)

type Material struct {
	BaseColor              [4]float32
	BaseColorLayer         uint32
	MetallicRoughnessLayer uint32
	NormalLayer            uint32
	OcclusionLayer         uint32
	EmissiveLayer          uint32

	MetallicFactor    float32
	RoughnessFactor   float32
	NormalScale       float32
	OcclusionStrength float32
	EmissiveFactor    [3]float32
	EmissiveStrength  float32

	AlphaMode   AlphaMode
	AlphaCutoff float32
	DoubleSided bool
	Unlit       bool

	IOR float32
}

func DefaultMaterial() Material {
	return Material{
		BaseColor:              [4]float32{1, 1, 1, 1},
		BaseColorLayer:         NoTextureLayer,
		MetallicRoughnessLayer: NoTextureLayer,
		NormalLayer:            NoTextureLayer,
		OcclusionLayer:         NoTextureLayer,
		EmissiveLayer:          NoTextureLayer,
		MetallicFactor:         1.0,
		RoughnessFactor:        1.0,
		NormalScale:            1.0,
		OcclusionStrength:      1.0,
		AlphaCutoff:            0.5,
		IOR:                    1.5,
		EmissiveStrength:       1.0,
	}
}

func EmissiveMaterial(color [3]float32, strength float32) Material {
	m := DefaultMaterial()
	m.BaseColor = [4]float32{color[0], color[1], color[2], 1.0}
	m.EmissiveFactor = color
	m.EmissiveStrength = strength
	return m
}

func AlbedoMaterial(color [4]float32) Material {
	m := DefaultMaterial()
	m.BaseColor = color
	return m
}

type MaterialGPU struct {
	BaseColor      [4]float32
	EmissiveFactor [3]float32
	AlphaMode      uint32

	BaseLayer              uint32
	EmissiveLayer          uint32
	NormalLayer            uint32
	MetallicRoughnessLayer uint32

	OcclusionLayer    uint32
	NormalScale       float32
	OcclusionStrength float32
	MetallicFactor    float32

	RoughnessFactor float32
	AlphaCutoff     float32
	Unlit           uint32
	IOR             float32

	EmissiveStrength float32
	Pad1a            float32
	Pad1b            float32
	Pad1c            float32
}

const MaterialGPUSize = uint64(96)

func (m Material) ToGPU() MaterialGPU {
	var alpha uint32
	switch m.AlphaMode {
	case AlphaModeMask:
		alpha = 1
	case AlphaModeBlend:
		alpha = 2
	}
	var unlit uint32
	if m.Unlit {
		unlit = 1
	}
	ior := m.IOR
	if ior <= 0 {
		ior = 1.5
	}
	emissiveStrength := m.EmissiveStrength
	if emissiveStrength <= 0 {
		emissiveStrength = 1.0
	}
	return MaterialGPU{
		BaseColor:              m.BaseColor,
		EmissiveFactor:         m.EmissiveFactor,
		AlphaMode:              alpha,
		BaseLayer:              m.BaseColorLayer,
		EmissiveLayer:          m.EmissiveLayer,
		NormalLayer:            m.NormalLayer,
		MetallicRoughnessLayer: m.MetallicRoughnessLayer,
		OcclusionLayer:         m.OcclusionLayer,
		NormalScale:            m.NormalScale,
		OcclusionStrength:      m.OcclusionStrength,
		MetallicFactor:         m.MetallicFactor,
		RoughnessFactor:        m.RoughnessFactor,
		AlphaCutoff:            m.AlphaCutoff,
		Unlit:                  unlit,
		IOR:                    ior,
		EmissiveStrength:       emissiveStrength,
	}
}
