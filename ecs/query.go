package ecs

import (
	"iter"
	"unsafe"
)

func (w *World) Query(include, exclude Mask) iter.Seq[Entity] {
	return func(yield func(Entity) bool) {
		for _, tableIndex := range w.cachedTables(include) {
			table := w.tables[tableIndex]
			if table.Mask&exclude != 0 {
				continue
			}
			for _, entity := range table.Entities {
				if !yield(entity) {
					return
				}
			}
		}
	}
}

func (w *World) QueryFirst(include, exclude Mask) (Entity, bool) {
	for _, tableIndex := range w.cachedTables(include) {
		table := w.tables[tableIndex]
		if table.Mask&exclude != 0 {
			continue
		}
		if len(table.Entities) > 0 {
			return table.Entities[0], true
		}
	}
	return Entity{}, false
}

func (w *World) CountQuery(include, exclude Mask) int {
	total := 0
	for _, tableIndex := range w.cachedTables(include) {
		table := w.tables[tableIndex]
		if table.Mask&exclude != 0 {
			continue
		}
		total += len(table.Entities)
	}
	return total
}

func (w *World) ForEach(include, exclude Mask, callback func(entity Entity, table *Archetype, index int)) {
	w.enterIter()
	defer w.leaveIter()
	for _, tableIndex := range w.cachedTables(include) {
		table := w.tables[tableIndex]
		if table.Mask&exclude != 0 {
			continue
		}
		for arrayIndex := 0; arrayIndex < len(table.Entities); arrayIndex++ {
			callback(table.Entities[arrayIndex], table, arrayIndex)
		}
	}
}

func MarkChanged[T any](world *World, entity Entity) {
	info := mustComponentInfo[T](world)
	tableIndex, arrayIndex, ok := world.entityLocs.get(entity)
	if !ok {
		return
	}
	table := world.tables[tableIndex]
	if table.Mask&info.mask == 0 {
		return
	}
	table.columns[info.bitIndex].markChanged(arrayIndex, world.currentTick)
}

func Column[T any](world *World, table *Archetype) ([]T, bool) {
	info, present := componentInfoFor[T](world)
	if !present {
		return nil, false
	}
	if table.Mask&info.mask == 0 {
		return nil, false
	}
	column := table.columns[info.bitIndex]
	if column.length == 0 {
		return nil, true
	}
	return unsafe.Slice((*T)(column.dataPtr), column.length), true
}
