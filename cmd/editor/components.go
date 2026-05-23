package main

import "github.com/matthewjberger/indigo/transform"

type Spinner struct {
	Axis     transform.Vec3
	Speed    float32
	Rotation transform.Quat
}
