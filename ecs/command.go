package ecs

func (w *World) Queue(command func(*World)) {
	w.commandBuffer = append(w.commandBuffer, command)
}

func (w *World) QueueSpawn(mask Mask) {
	w.commandBuffer = append(w.commandBuffer, func(world *World) {
		world.Spawn(mask)
	})
}

func (w *World) QueueDespawn(entity Entity) {
	w.commandBuffer = append(w.commandBuffer, func(world *World) {
		world.Despawn(entity)
	})
}

func (w *World) QueueAddComponents(entity Entity, mask Mask) {
	w.commandBuffer = append(w.commandBuffer, func(world *World) {
		world.AddComponents(entity, mask)
	})
}

func (w *World) QueueRemoveComponents(entity Entity, mask Mask) {
	w.commandBuffer = append(w.commandBuffer, func(world *World) {
		world.RemoveComponents(entity, mask)
	})
}

func QueueSet[T any](world *World, entity Entity, value T) {
	world.commandBuffer = append(world.commandBuffer, func(w *World) {
		Set(w, entity, value)
	})
}

func QueueAdd[T any](world *World, entity Entity) {
	world.commandBuffer = append(world.commandBuffer, func(w *World) {
		Add[T](w, entity)
	})
}

func QueueRemove[T any](world *World, entity Entity) {
	world.commandBuffer = append(world.commandBuffer, func(w *World) {
		Remove[T](w, entity)
	})
}

func QueueAddTag[T any](world *World, entity Entity) {
	world.commandBuffer = append(world.commandBuffer, func(w *World) {
		AddTag[T](w, entity)
	})
}

func QueueRemoveTag[T any](world *World, entity Entity) {
	world.commandBuffer = append(world.commandBuffer, func(w *World) {
		RemoveTag[T](w, entity)
	})
}

func (w *World) ApplyCommands() {
	pending := w.commandBuffer
	w.commandBuffer = nil
	for _, command := range pending {
		command(w)
	}
}

func (w *World) CommandCount() int { return len(w.commandBuffer) }

func (w *World) ClearCommands() { w.commandBuffer = nil }
