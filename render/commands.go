package render

import "github.com/matthewjberger/indigo/ecs"

type Command interface {
	Apply(world *ecs.World, renderer *Renderer)
}

type CommandQueueResource struct {
	commands []Command
}

func QueueCommand(world *ecs.World, command Command) {
	queue := ecs.MustResource[CommandQueueResource](world)
	queue.commands = append(queue.commands, command)
}

func DrainCommands(world *ecs.World, renderer *Renderer) {
	queue, ok := ecs.Resource[CommandQueueResource](world)
	if !ok {
		return
	}
	pending := queue.commands
	queue.commands = queue.commands[:0]
	for _, cmd := range pending {
		cmd.Apply(world, renderer)
	}
}
