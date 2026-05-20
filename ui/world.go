package ui

import (
	"indigo/ecs"
	"indigo/window"
)

// NewWorld returns a fresh ECS world configured for UI: every UI
// component is registered, the layout scratch and pointer state are
// installed as resources, and the window resource is initialized
// with the supplied viewport size so the layout system has something
// to anchor against on the first frame.
//
// Layout policy: this is the only world the [LayoutSystem] /
// [InteractionSystem] systems operate on.
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

// NewSchedule returns a [ecs.Schedule] pre-loaded with the standard
// UI systems in the right order: interaction first (so widgets react
// to the click that happened this frame), then layout (so the
// renderer sees the latest Resolved rectangles).
func NewSchedule() *ecs.Schedule {
	sched := ecs.NewSchedule()
	sched.Push("ui_interaction", InteractionSystem)
	sched.Push("ui_layout", LayoutSystem)
	return sched
}
