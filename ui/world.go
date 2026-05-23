package ui

import (
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/window"
)

func NewWorld(viewportWidth, viewportHeight uint32) *ecs.World {
	world := ecs.New()
	Register(world)
	ecs.SetResource(world, window.Window{
		Viewport: window.ViewportSize{Width: viewportWidth, Height: viewportHeight},
	})
	ecs.SetResource(world, NewLayoutState())
	ecs.SetResource(world, PointerState{})
	return world
}

func NewSchedule() *ecs.Schedule {
	sched := ecs.NewSchedule()
	sched.Push("ui_layout", LayoutSystem)
	sched.Push("ui_interaction", InteractionSystem)
	return sched
}
