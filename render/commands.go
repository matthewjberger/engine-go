package render

import "github.com/matthewjberger/indigo/ecs"

type RenderCommand interface {
	Apply(world *ecs.World, renderer *Renderer)
}

type RenderCommandQueueResource struct {
	commands []RenderCommand
}

func QueueRenderCommand(world *ecs.World, command RenderCommand) {
	queue := ecs.MustResource[RenderCommandQueueResource](world)
	queue.commands = append(queue.commands, command)
}

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
