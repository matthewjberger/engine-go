package app

import "github.com/matthewjberger/indigo/ecs"

// EngineEntity is the component a game-side entity uses to point at
// its engine-side counterpart. In the dual-world pattern, a game
// world holds game state (AI, inventory, spinner, behaviours) and
// an engine world holds rendering / physics / transforms; they are
// linked by this component.
//
// Game systems read this off a game entity to find the engine entity
// they should sync into (transform writes, despawn cascades, etc.).
type EngineEntity struct {
	Entity ecs.Entity
}
