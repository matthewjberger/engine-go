package main

import "github.com/matthewjberger/indigo/transform"

type Paddle struct {
	Position transform.Vec3
	HalfSize transform.Vec3
	Speed    float32
}

type Ball struct {
	Position transform.Vec3
	Velocity transform.Vec3
	Radius   float32
	Launched bool
}

type Brick struct {
	Position transform.Vec3
	HalfSize transform.Vec3
	Score    int
}

type GameState struct {
	Score        int
	Lives        int
	Started      bool
	Won          bool
	Lost         bool
	RequestReset bool
}

type FieldBounds struct {
	MinX, MaxX float32
	MinZ, MaxZ float32
}
