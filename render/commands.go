package render

import "github.com/matthewjberger/indigo/ecs"

// RenderCommand is the sealed interface for deferred GPU-side work
// queued during ECS systems and drained by the renderer at frame
// setup, before any pass runs. Typical commands: load a glTF
// scene, upload an HDR skybox, capture a screenshot, rebuild IBL
// from a different atmosphere.
//
// The renderer calls [DrainRenderCommands] inside
// [RenderFrame] before any pass starts so commands can issue
// staging copies, texture uploads, or buffer reallocations
// without colliding with mid-frame rendering work.
type RenderCommand interface {
	Apply(world *ecs.World, renderer *Renderer)
}

// RenderCommandQueueResource owns the per-frame render-command
// queue. One instance is registered on the engine world at setup.
type RenderCommandQueueResource struct {
	commands []RenderCommand
}

// QueueRenderCommand appends a command to the world's render
// queue. The command applies on the next frame's setup pass.
func QueueRenderCommand(world *ecs.World, command RenderCommand) {
	queue := ecs.MustResource[RenderCommandQueueResource](world)
	queue.commands = append(queue.commands, command)
}

// DrainRenderCommands applies every queued render command in order
// and clears the queue. Called by [RenderFrame] at frame setup.
func DrainRenderCommands(world *ecs.World, renderer *Renderer) {
	queue, ok := ecs.Resource[RenderCommandQueueResource](world)
	if !ok {
		return
	}
	pending := queue.commands
	queue.commands = queue.commands[:0]
	for _, cmd := range pending {
		cmd.Apply(world, renderer)
	}
}
