package render

import "github.com/go-gl/mathgl/mgl32"

var ndcZTo01 = mgl32.Mat4{
	1, 0, 0, 0,
	0, 1, 0, 0,
	0, 0, 0.5, 0,
	0, 0, 0.5, 1,
}

func PerspectiveZO(fovY, aspect, near, far float32) mgl32.Mat4 {
	return ndcZTo01.Mul4(mgl32.Perspective(fovY, aspect, near, far))
}
