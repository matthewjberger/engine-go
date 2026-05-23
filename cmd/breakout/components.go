package main

import "github.com/matthewjberger/indigo/transform"

// Paddle is the player's paddle: a wide flat box near the bottom of
// the field. Position is stored game-side; the sync system writes it
// onto the linked engine entity's [transform.LocalTransform].
type Paddle struct {
	Position transform.Vec3
	HalfSize transform.Vec3
	Speed    float32
}

// Ball is the ball: a small cube that moves linearly and bounces off
// surfaces. Launched=false means it's resting on the paddle.
type Ball struct {
	Position transform.Vec3
	Velocity transform.Vec3
	Radius   float32
	Launched bool
}

// Brick is a single brick in the wall. Score is awarded on destroy.
type Brick struct {
	Position transform.Vec3
	HalfSize transform.Vec3
	Score    int
}

// GameState is the singleton resource carrying score, lives, the
// win/lose/started flags, and a reset-requested flag the input
// system sets when the player presses R after the round ends.
type GameState struct {
	Score        int
	Lives        int
	Started      bool
	Won          bool
	Lost         bool
	RequestReset bool
}

// FieldBounds defines the playable region in world XZ. The paddle and
// ball clamp/bounce against these bounds.
type FieldBounds struct {
	MinX, MaxX float32
	MinZ, MaxZ float32
}
