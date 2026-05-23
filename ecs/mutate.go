package ecs

func Get[T any](world *World, entity Entity) (*T, bool) {
	info, ok := componentInfoFor[T](world)
	if !ok {
		return nil, false
	}
	tableIndex, arrayIndex, ok := world.entityLocs.get(entity)
	if !ok {
		return nil, false
	}
	table := world.tables[tableIndex]
	if table.Mask&info.mask == 0 {
		return nil, false
	}
	column := table.columns[info.bitIndex]
	return (*T)(column.at(arrayIndex)), true
}

func GetMut[T any](world *World, entity Entity) (*T, bool) {
	info, ok := componentInfoFor[T](world)
	if !ok {
		return nil, false
	}
	tableIndex, arrayIndex, ok := world.entityLocs.get(entity)
	if !ok {
		return nil, false
	}
	table := world.tables[tableIndex]
	if table.Mask&info.mask == 0 {
		return nil, false
	}
	column := table.columns[info.bitIndex]
	column.markChanged(arrayIndex, world.currentTick)
	return (*T)(column.at(arrayIndex)), true
}

func Has[T any](world *World, entity Entity) bool {
	info, ok := componentInfoFor[T](world)
	if !ok {
		return false
	}
	tableIndex, _, ok := world.entityLocs.get(entity)
	if !ok {
		return false
	}
	return world.tables[tableIndex].Mask&info.mask != 0
}

func (w *World) HasComponents(entity Entity, mask Mask) bool {
	tableIndex, _, ok := w.entityLocs.get(entity)
	if !ok {
		return false
	}
	return w.tables[tableIndex].Mask&mask == mask
}

func (w *World) ComponentMask(entity Entity) (Mask, bool) {
	tableIndex, _, ok := w.entityLocs.get(entity)
	if !ok {
		return 0, false
	}
	return w.tables[tableIndex].Mask, true
}

func Set[T any](world *World, entity Entity, value T) {
	info := mustComponentInfo[T](world)
	tableIndex, arrayIndex, ok := world.entityLocs.get(entity)
	if !ok {
		return
	}
	table := world.tables[tableIndex]
	if table.Mask&info.mask != 0 {
		column := table.columns[info.bitIndex]
		*(*T)(column.at(arrayIndex)) = value
		column.markChanged(arrayIndex, world.currentTick)
		return
	}
	world.AddComponents(entity, info.mask)
	tableIndex, arrayIndex, ok = world.entityLocs.get(entity)
	if !ok {
		return
	}
	table = world.tables[tableIndex]
	column := table.columns[info.bitIndex]
	*(*T)(column.at(arrayIndex)) = value
	column.markChanged(arrayIndex, world.currentTick)
}

func Add[T any](world *World, entity Entity) {
	info := mustComponentInfo[T](world)
	world.AddComponents(entity, info.mask)
}

func Remove[T any](world *World, entity Entity) {
	info := mustComponentInfo[T](world)
	world.RemoveComponents(entity, info.mask)
}
