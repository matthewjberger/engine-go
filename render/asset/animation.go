package asset

type AnimationProperty uint8

const (
	AnimationTranslation AnimationProperty = iota
	AnimationRotation
	AnimationScale
	AnimationMorphWeights
)

type AnimationInterpolation uint8

const (
	InterpolationLinear AnimationInterpolation = iota
	InterpolationStep
	InterpolationCubicSpline
)

type AnimationSampler struct {
	Interpolation AnimationInterpolation
	Inputs        []float32
	Vec3Outputs   [][3]float32
	Vec4Outputs   [][4]float32
	ScalarOutputs [][]float32
}

type AnimationChannel struct {
	TargetNode int
	Property   AnimationProperty
	Sampler    AnimationSampler
}

type AnimationClip struct {
	Name     string
	Duration float32
	Channels []AnimationChannel
}
