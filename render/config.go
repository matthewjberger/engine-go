package render

type EngineConfig struct {
	NumShadowCascades   int
	ShadowMapSize       uint32
	MaxSpotShadows      uint32
	MaxPointShadows     uint32
	SpotShadowAtlasSize uint32

	MaxLightsBuffer     uint32
	MaxLightsPerCluster uint32
	ClusterGridX        uint32
	ClusterGridY        uint32
	ClusterGridZ        uint32

	BrdfLutSize           uint32
	PrefilteredSize       uint32
	IrradianceSamples     uint32
	ProceduralCubemapSize uint32

	SelectionMaskMaxEntities uint32

	MaxJointsPerSkin uint32

	BloomMipCount uint32
}

func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		NumShadowCascades:        4,
		ShadowMapSize:            2048,
		MaxSpotShadows:           4,
		MaxPointShadows:          4,
		SpotShadowAtlasSize:      2048,
		MaxLightsBuffer:          1024,
		MaxLightsPerCluster:      256,
		ClusterGridX:             16,
		ClusterGridY:             9,
		ClusterGridZ:             24,
		BrdfLutSize:              256,
		PrefilteredSize:          512,
		IrradianceSamples:        1024,
		ProceduralCubemapSize:    1024,
		SelectionMaskMaxEntities: 4096,
		MaxJointsPerSkin:         128,
		BloomMipCount:            5,
	}
}
