package ecs

type EcsCommand interface {
	apply(world *World)
}

type DespawnEntity struct{ Entity Entity }

func (c DespawnEntity) apply(world *World) {
	world.Despawn(c.Entity)
}

type EcsCommandQueueResource struct {
	commands []EcsCommand
}

func QueueEcsCommand(world *World, command EcsCommand) {
	queue := MustResource[EcsCommandQueueResource](world)
	queue.commands = append(queue.commands, command)
}

func ProcessEcsCommands(world *World) {
	queue := MustResource[EcsCommandQueueResource](world)
	pending := queue.commands
	queue.commands = queue.commands[:0]
	for _, cmd := range pending {
		cmd.apply(world)
	}
}
