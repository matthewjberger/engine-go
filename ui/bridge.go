package ui

import "github.com/matthewjberger/indigo/ecs"

type WorldRef struct {
	World *ecs.World
}

func HasUI(engine *ecs.World) bool {
	return ecs.HasResource[WorldRef](engine)
}
