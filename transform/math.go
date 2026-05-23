package transform

import "github.com/go-gl/mathgl/mgl32"

type (
	Vec2 = mgl32.Vec2
	Vec3 = mgl32.Vec3
	Mat4 = mgl32.Mat4
	Quat = mgl32.Quat
)

func QuatIdentity() Quat { return mgl32.QuatIdent() }

func QuatFromAxisAngle(angle float32, axis Vec3) Quat {
	return mgl32.QuatRotate(angle, axis)
}

func QuatNormalize(q Quat) Quat { return q.Normalize() }

func QuatToMat4(q Quat) Mat4 { return q.Mat4() }

func Mat4Identity() Mat4 { return mgl32.Ident4() }
