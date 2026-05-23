package asset

import "math"

type BoundingVolume struct {
	Min [3]float32
	Max [3]float32
}

func ComputeBounds(vertices []MeshVertex) BoundingVolume {
	if len(vertices) == 0 {
		return BoundingVolume{}
	}
	min := [3]float32{
		float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(1)),
	}
	max := [3]float32{
		float32(math.Inf(-1)), float32(math.Inf(-1)), float32(math.Inf(-1)),
	}
	for i := range vertices {
		p := vertices[i].Position
		for axis := 0; axis < 3; axis++ {
			if p[axis] < min[axis] {
				min[axis] = p[axis]
			}
			if p[axis] > max[axis] {
				max[axis] = p[axis]
			}
		}
	}
	return BoundingVolume{Min: min, Max: max}
}

func (b BoundingVolume) Center() [3]float32 {
	return [3]float32{
		(b.Min[0] + b.Max[0]) * 0.5,
		(b.Min[1] + b.Max[1]) * 0.5,
		(b.Min[2] + b.Max[2]) * 0.5,
	}
}

func (b BoundingVolume) Radius() float32 {
	dx := (b.Max[0] - b.Min[0]) * 0.5
	dy := (b.Max[1] - b.Min[1]) * 0.5
	dz := (b.Max[2] - b.Min[2]) * 0.5
	return float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
}

func (b BoundingVolume) Corners() [8][3]float32 {
	return [8][3]float32{
		{b.Min[0], b.Min[1], b.Min[2]},
		{b.Max[0], b.Min[1], b.Min[2]},
		{b.Max[0], b.Max[1], b.Min[2]},
		{b.Min[0], b.Max[1], b.Min[2]},
		{b.Min[0], b.Min[1], b.Max[2]},
		{b.Max[0], b.Min[1], b.Max[2]},
		{b.Max[0], b.Max[1], b.Max[2]},
		{b.Min[0], b.Max[1], b.Max[2]},
	}
}

var BoundingBoxEdges = [24]uint8{
	0, 1, 1, 2, 2, 3, 3, 0,
	4, 5, 5, 6, 6, 7, 7, 4,
	0, 4, 1, 5, 2, 6, 3, 7,
}
