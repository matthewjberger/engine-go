package asset

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/qmuntal/gltf"
)

const decomposeEps = 1e-4

func reconstructMatrix(translation [3]float32, rotation [4]float32, scale [3]float32) [16]float32 {
	q := mgl32.Quat{W: rotation[3], V: mgl32.Vec3{rotation[0], rotation[1], rotation[2]}}
	r := q.Mat4()

	r[0] *= scale[0]
	r[1] *= scale[0]
	r[2] *= scale[0]
	r[4] *= scale[1]
	r[5] *= scale[1]
	r[6] *= scale[1]
	r[8] *= scale[2]
	r[9] *= scale[2]
	r[10] *= scale[2]
	r[12] = translation[0]
	r[13] = translation[1]
	r[14] = translation[2]
	out := [16]float32{}
	copy(out[:], r[:])
	return out
}

func roundTripMatrix(t *testing.T, label string, matrix [16]float64) {
	t.Helper()
	node := &gltf.Node{Matrix: matrix}
	tr, rot, sc := nodeTRS(node)
	got := reconstructMatrix(tr, rot, sc)
	for i := 0; i < 16; i++ {
		want := float32(matrix[i])
		diff := float64(got[i] - want)
		if math.Abs(diff) > decomposeEps {
			t.Errorf("%s: matrix[%d] = %f, want %f", label, i, got[i], want)
		}
	}
}

func TestDecompose180YawRoundTrip(t *testing.T) {

	roundTripMatrix(t, "180yaw", [16]float64{
		-1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, -1, 0,
		0, 0, 0, 1,
	})
}

func TestDecompose180PitchRoundTrip(t *testing.T) {

	roundTripMatrix(t, "180pitch", [16]float64{
		1, 0, 0, 0,
		0, -1, 0, 0,
		0, 0, -1, 0,
		0, 0, 0, 1,
	})
}

func TestDecompose180RollRoundTrip(t *testing.T) {

	roundTripMatrix(t, "180roll", [16]float64{
		-1, 0, 0, 0,
		0, -1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})
}

func TestDecompose90YawWithTranslationRoundTrip(t *testing.T) {
	roundTripMatrix(t, "90yaw+trans", [16]float64{
		0, 0, -1, 0,
		0, 1, 0, 0,
		1, 0, 0, 0,
		5, 6, 7, 1,
	})
}

func TestDecomposeIdentityRoundTrip(t *testing.T) {
	roundTripMatrix(t, "identity", [16]float64{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})
}

func TestDecomposeScaleOnlyRoundTrip(t *testing.T) {
	roundTripMatrix(t, "scale_only", [16]float64{
		2, 0, 0, 0,
		0, 3, 0, 0,
		0, 0, 4, 0,
		0, 0, 0, 1,
	})
}
