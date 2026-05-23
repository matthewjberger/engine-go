package app

import (
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/render"
)

type App struct {
	Initialize           func(world *ecs.World)
	ConfigureRenderGraph func(world *ecs.World, renderer *render.Renderer)
	RunSystems           func(world *ecs.World)
	PreRender            func(world *ecs.World)
}
