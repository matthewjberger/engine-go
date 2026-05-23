package ecs

func Changed[T any](world *World, entity Entity) bool {
	info, ok := componentInfoFor[T](world)
	if !ok {
		return false
	}
	tableIndex, arrayIndex, ok := world.entityLocs.get(entity)
	if !ok {
		return false
	}
	table := world.tables[tableIndex]
	if table.Mask&info.mask == 0 {
		return false
	}
	return table.columns[info.bitIndex].changed[arrayIndex] > world.lastTick
}
