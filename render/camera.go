package render

import "github.com/go-gl/mathgl/mgl32"

type Camera struct {
	Eye    mgl32.Vec3
	Target mgl32.Vec3
	Up     mgl32.Vec3

	FovYRadians float32
	Near        float32
	Far         float32
}

func DefaultCamera() Camera {
	return Camera{
		Eye:         mgl32.Vec3{0, 2, 6},
		Target:      mgl32.Vec3{0, 0, 0},
		Up:          mgl32.Vec3{0, 1, 0},
		FovYRadians: mgl32.DegToRad(60),
		Near:        0.1,
		Far:         1000.0,
	}
}

func CameraView(c *Camera) mgl32.Mat4 {
	return mgl32.LookAtV(c.Eye, c.Target, c.Up)
}

func CameraProjection(c *Camera, aspect float32) mgl32.Mat4 {
	return PerspectiveZO(c.FovYRadians, aspect, c.Near, c.Far)
}

func CameraViewProjection(c *Camera, aspect float32) mgl32.Mat4 {
	return CameraProjection(c, aspect).Mul4(CameraView(c))
}
