package main

import "github.com/matthewjberger/indigo/transform"

// Spinner is the game-side component for "this entity rotates around
// Axis by Speed radians per second." The game world holds its own
// Rotation accumulator (source of truth); the sync system writes the
// resulting rotation into the linked engine entity's
// [transform.LocalTransform] via the [app] sync API.
type Spinner struct {
	Axis     transform.Vec3
	Speed    float32
	Rotation transform.Quat
}
