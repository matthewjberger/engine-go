package render

import "github.com/matthewjberger/indigo/transform"

type LightType uint32

const (
	LightTypeDirectional LightType = 0
	LightTypePoint       LightType = 1
	LightTypeSpot        LightType = 2
)

type Light struct {
	Type           LightType
	Color          transform.Vec3
	Intensity      float32
	Range          float32
	InnerConeAngle float32
	OuterConeAngle float32
	CastShadows    bool
	ShadowBias     float32

	LightSize float32
}

const MaxLights = 8
