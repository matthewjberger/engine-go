// Package app is the data-oriented application lifecycle for an
// indigo binary.
//
// Lifecycle hooks live as function-value fields on an [App] struct.
// Applications construct an [App] and hand it to the main loop.
//
// Conventions:
//   - Initialize runs once after the renderer is built.
//   - ConfigureRenderGraph runs once after Initialize. This is where the
//     application registers its passes against the engine's resources.
//   - RunSystems runs every frame before rendering.
//   - PreRender runs every frame after RunSystems, before [Renderer.RenderFrame].
//
// Every hook is optional; nil fields are skipped.
package app

import (
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/render"
)

// App is a bundle of lifecycle hooks the main loop invokes at
// fixed points each frame.
type App struct {
	Initialize           func(world *ecs.World)
	ConfigureRenderGraph func(world *ecs.World, renderer *render.Renderer)
	RunSystems           func(world *ecs.World)
	PreRender            func(world *ecs.World)
}
