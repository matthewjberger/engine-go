package transform

import "github.com/matthewjberger/indigo/ecs"

type LocalTransform struct {
	Translation Vec3
	Rotation    Quat
	Scale       Vec3
}

func IdentityLocalTransform() LocalTransform {
	return LocalTransform{
		Translation: Vec3{0, 0, 0},
		Rotation:    QuatIdentity(),
		Scale:       Vec3{1, 1, 1},
	}
}

func FromTranslation(translation Vec3) LocalTransform {
	return LocalTransform{
		Translation: translation,
		Rotation:    QuatIdentity(),
		Scale:       Vec3{1, 1, 1},
	}
}

func AsMatrix(t *LocalTransform) Mat4 {
	matrix := QuatToMat4(QuatNormalize(t.Rotation))

	matrix[0] *= t.Scale[0]
	matrix[1] *= t.Scale[0]
	matrix[2] *= t.Scale[0]

	matrix[4] *= t.Scale[1]
	matrix[5] *= t.Scale[1]
	matrix[6] *= t.Scale[1]

	matrix[8] *= t.Scale[2]
	matrix[9] *= t.Scale[2]
	matrix[10] *= t.Scale[2]

	matrix[12] = t.Translation[0]
	matrix[13] = t.Translation[1]
	matrix[14] = t.Translation[2]

	return matrix
}

func (t LocalTransform) Forward() Vec3 {
	return t.Rotation.Rotate(Vec3{0, 0, -1})
}

func (t LocalTransform) Right() Vec3 {
	return t.Rotation.Rotate(Vec3{1, 0, 0})
}

func (t LocalTransform) Up() Vec3 {
	return t.Rotation.Rotate(Vec3{0, 1, 0})
}

type GlobalTransform struct {
	Matrix Mat4
}

func (g GlobalTransform) Forward() Vec3 {
	return Vec3{-g.Matrix[8], -g.Matrix[9], -g.Matrix[10]}.Normalize()
}

func (g GlobalTransform) Right() Vec3 {
	return Vec3{g.Matrix[0], g.Matrix[1], g.Matrix[2]}.Normalize()
}

func (g GlobalTransform) Up() Vec3 {
	return Vec3{g.Matrix[4], g.Matrix[5], g.Matrix[6]}.Normalize()
}

func GlobalTransformTranslation(g *GlobalTransform) Vec3 {
	return Vec3{g.Matrix[12], g.Matrix[13], g.Matrix[14]}
}

func IdentityGlobalTransform() GlobalTransform {
	return GlobalTransform{Matrix: Mat4Identity()}
}

type Parent struct {
	Entity ecs.Entity
	IsRoot bool
}

type LocalTransformDirty struct{}

type IgnoreParentScale struct{}

type GroupRoot struct{}
