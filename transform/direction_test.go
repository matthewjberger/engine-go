package transform_test

import (
	"math"
	"testing"

	"github.com/matthewjberger/indigo/transform"
)

const directionEpsilon = 1e-5

func vec3ApproxEqual(a, b transform.Vec3, eps float32) bool {
	dx := float64(a[0] - b[0])
	dy := float64(a[1] - b[1])
	dz := float64(a[2] - b[2])
	return math.Sqrt(dx*dx+dy*dy+dz*dz) < float64(eps)
}

func TestLocalTransformIdentityForward(t *testing.T) {
	tr := transform.IdentityLocalTransform()
	if got := tr.Forward(); !vec3ApproxEqual(got, transform.Vec3{0, 0, -1}, directionEpsilon) {
		t.Errorf("identity forward = %v, want {0 0 -1}", got)
	}
	if got := tr.Right(); !vec3ApproxEqual(got, transform.Vec3{1, 0, 0}, directionEpsilon) {
		t.Errorf("identity right = %v, want {1 0 0}", got)
	}
	if got := tr.Up(); !vec3ApproxEqual(got, transform.Vec3{0, 1, 0}, directionEpsilon) {
		t.Errorf("identity up = %v, want {0 1 0}", got)
	}
}

func TestLocalTransformForwardRotatedYaw90(t *testing.T) {
	tr := transform.IdentityLocalTransform()
	tr.Rotation = transform.QuatFromAxisAngle(float32(math.Pi/2), transform.Vec3{0, 1, 0})
	// 90 deg yaw around +Y: forward (-Z) rotates to -X.
	if got := tr.Forward(); !vec3ApproxEqual(got, transform.Vec3{-1, 0, 0}, directionEpsilon) {
		t.Errorf("yaw 90 forward = %v, want {-1 0 0}", got)
	}
	// Right (+X) rotates to -Z.
	if got := tr.Right(); !vec3ApproxEqual(got, transform.Vec3{0, 0, -1}, directionEpsilon) {
		t.Errorf("yaw 90 right = %v, want {0 0 -1}", got)
	}
}

func TestGlobalTransformAxesMatchLocalAfterPropagation(t *testing.T) {
	tr := transform.IdentityLocalTransform()
	tr.Rotation = transform.QuatFromAxisAngle(float32(math.Pi/3), transform.Vec3{0, 1, 0})
	matrix := transform.AsMatrix(&tr)
	global := transform.GlobalTransform{Matrix: matrix}

	if !vec3ApproxEqual(global.Forward(), tr.Forward(), directionEpsilon) {
		t.Errorf("global forward %v != local forward %v", global.Forward(), tr.Forward())
	}
	if !vec3ApproxEqual(global.Right(), tr.Right(), directionEpsilon) {
		t.Errorf("global right %v != local right %v", global.Right(), tr.Right())
	}
	if !vec3ApproxEqual(global.Up(), tr.Up(), directionEpsilon) {
		t.Errorf("global up %v != local up %v", global.Up(), tr.Up())
	}
}
