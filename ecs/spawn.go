package ecs

import "math/bits"

func (w *World) Spawn(mask Mask) Entity {
	w.guardStructuralMutation("Spawn")
	entity := w.allocator.allocate()
	w.placeEntityInArchetype(entity, mask)
	return entity
}

func (w *World) SpawnEntityInto(entity Entity, mask Mask) {
	w.guardStructuralMutation("SpawnEntityInto")
	if _, _, ok := w.entityLocs.get(entity); ok {
		panic("freecs: SpawnEntityInto called for an entity that already has a row in this world; use AddComponents/RemoveComponents to change its archetype")
	}
	w.placeEntityInArchetype(entity, mask)
}

func (w *World) placeEntityInArchetype(entity Entity, mask Mask) {
	tableIndex := w.getOrCreateTable(mask)
	tick := w.currentTick
	table := w.tables[tableIndex]
	arrayIndex := len(table.Entities)
	table.Entities = append(table.Entities, entity)
	for bit := uint8(0); bit < w.registry.nextBit; bit++ {
		if mask&(Mask(1)<<bit) == 0 {
			continue
		}
		table.columns[bit].pushZero(tick)
	}
	w.entityLocs.set(entity, tableIndex, arrayIndex)
}

func (w *World) SpawnBatch(mask Mask, count int, init func(table *Archetype, index int)) []Entity {
	w.guardStructuralMutation("SpawnBatch")
	if count <= 0 {
		return nil
	}
	tableIndex := w.getOrCreateTable(mask)
	tick := w.currentTick
	table := w.tables[tableIndex]
	entities := make([]Entity, count)
	startIndex := len(table.Entities)
	for slot := 0; slot < count; slot++ {
		entity := w.allocator.allocate()
		entities[slot] = entity
		arrayIndex := startIndex + slot
		table.Entities = append(table.Entities, entity)
		for bit := uint8(0); bit < w.registry.nextBit; bit++ {
			if mask&(Mask(1)<<bit) == 0 {
				continue
			}
			table.columns[bit].pushZero(tick)
		}
		w.entityLocs.set(entity, tableIndex, arrayIndex)
		if init != nil {
			init(table, arrayIndex)
		}
	}
	return entities
}

func (w *World) Despawn(entity Entity) bool {
	w.guardStructuralMutation("Despawn")
	if !despawnFromArchetype(w, entity) {
		return false
	}
	w.allocator.deallocate(entity)
	return true
}

func despawnFromArchetype(world *World, entity Entity) bool {
	tableIndex, arrayIndex, ok := world.entityLocs.get(entity)
	if !ok {
		return false
	}
	world.entityLocs.markDeallocated(entity.ID)
	Send(world, EntityDespawned{Entity: entity})

	for _, set := range world.tagSets {
		delete(set, entity)
	}

	table := world.tables[tableIndex]
	lastIndex := len(table.Entities) - 1
	var swapped Entity
	hasSwap := arrayIndex < lastIndex
	if hasSwap {
		swapped = table.Entities[lastIndex]
	}

	if hasSwap {
		table.Entities[arrayIndex] = table.Entities[lastIndex]
	}
	table.Entities = table.Entities[:lastIndex]

	for bit := uint8(0); bit < world.registry.nextBit; bit++ {
		if table.Mask&(Mask(1)<<bit) == 0 {
			continue
		}
		table.columns[bit].swapRemove(arrayIndex)
	}

	if hasSwap {
		world.entityLocs.set(swapped, tableIndex, arrayIndex)
	}
	return true
}

func (w *World) AddComponents(entity Entity, mask Mask) bool {
	tableIndex, arrayIndex, ok := w.entityLocs.get(entity)
	if !ok {
		return false
	}
	currentMask := w.tables[tableIndex].Mask
	if currentMask&mask == mask {
		return true
	}
	w.guardStructuralMutation("AddComponents")

	var destTableIndex int
	if bits.OnesCount64(uint64(mask)) == 1 {
		bit := bits.TrailingZeros64(uint64(mask))
		cached := w.tableEdges[tableIndex].add[bit]
		if cached >= 0 {
			destTableIndex = int(cached)
		} else {
			destTableIndex = w.getOrCreateTable(currentMask | mask)
		}
	} else {
		destTableIndex = w.getOrCreateTable(currentMask | mask)
	}
	moveEntity(w, entity, tableIndex, arrayIndex, destTableIndex)
	return true
}

func (w *World) RemoveComponents(entity Entity, mask Mask) bool {
	tableIndex, arrayIndex, ok := w.entityLocs.get(entity)
	if !ok {
		return false
	}
	currentMask := w.tables[tableIndex].Mask
	if currentMask&mask == 0 {
		return true
	}
	w.guardStructuralMutation("RemoveComponents")

	var destTableIndex int
	if bits.OnesCount64(uint64(mask)) == 1 {
		bit := bits.TrailingZeros64(uint64(mask))
		cached := w.tableEdges[tableIndex].remove[bit]
		if cached >= 0 {
			destTableIndex = int(cached)
		} else {
			destTableIndex = w.getOrCreateTable(currentMask &^ mask)
		}
	} else {
		destTableIndex = w.getOrCreateTable(currentMask &^ mask)
	}
	moveEntity(w, entity, tableIndex, arrayIndex, destTableIndex)
	return true
}

func moveEntity(world *World, entity Entity, fromTableIndex, fromArrayIndex, toTableIndex int) {
	if fromTableIndex == toTableIndex {
		return
	}
	tick := world.currentTick
	fromTable := world.tables[fromTableIndex]
	toTable := world.tables[toTableIndex]

	toArrayIndex := len(toTable.Entities)
	toTable.Entities = append(toTable.Entities, entity)

	for bit := uint8(0); bit < world.registry.nextBit; bit++ {
		bitMask := Mask(1) << bit
		if toTable.Mask&bitMask == 0 {
			continue
		}
		if fromTable.Mask&bitMask != 0 {
			toTable.columns[bit].migrateFrom(fromTable.columns[bit], fromArrayIndex, tick)
		} else {
			toTable.columns[bit].pushZero(tick)
		}
	}

	world.entityLocs.set(entity, toTableIndex, toArrayIndex)

	lastIndex := len(fromTable.Entities) - 1
	var swapped Entity
	hasSwap := fromArrayIndex < lastIndex
	if hasSwap {
		swapped = fromTable.Entities[lastIndex]
		fromTable.Entities[fromArrayIndex] = fromTable.Entities[lastIndex]
	}
	fromTable.Entities = fromTable.Entities[:lastIndex]

	for bit := uint8(0); bit < world.registry.nextBit; bit++ {
		if fromTable.Mask&(Mask(1)<<bit) == 0 {
			continue
		}
		fromTable.columns[bit].swapRemove(fromArrayIndex)
	}

	if hasSwap {
		world.entityLocs.set(swapped, fromTableIndex, fromArrayIndex)
	}
}
