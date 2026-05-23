package ecs

import "reflect"

type World struct {
	_             noCopy
	registry      *registry
	allocator     *allocator
	entityLocs    entityLocations
	tables        []*Archetype
	tableLookup   map[Mask]int
	tableEdges    []*tableEdges
	queryCache    map[Mask][]int
	currentTick   uint32
	lastTick      uint32
	eventQueues   []eventDriver
	eventByType   map[reflect.Type]int
	tagSets       map[reflect.Type]map[Entity]struct{}
	commandBuffer []func(*World)
	resources     map[reflect.Type]any
	iterDepth     int
}

func (w *World) enterIter() { w.iterDepth++ }
func (w *World) leaveIter() { w.iterDepth-- }

func (w *World) guardStructuralMutation(op string) {
	if w.iterDepth > 0 {
		panic("freecs: " + op + " called during Iter/ForEach; defer via the command buffer (Queue* helpers)")
	}
}

func New() *World {
	return newWorldWithAllocator(&allocator{})
}

func newWorldWithAllocator(shared *allocator) *World {
	return &World{
		registry:    newRegistry(),
		allocator:   shared,
		tableLookup: make(map[Mask]int),
		queryCache:  make(map[Mask][]int),
		eventByType: make(map[reflect.Type]int),
		tagSets:     make(map[reflect.Type]map[Entity]struct{}),
		resources:   make(map[reflect.Type]any),

		currentTick: 1,
	}
}

func (w *World) CurrentTick() uint32 { return w.currentTick }

func (w *World) LastTick() uint32 { return w.lastTick }

func (w *World) Step() {
	for _, queue := range w.eventQueues {
		queue.update()
	}
	w.lastTick = w.currentTick
	w.currentTick++
}

func (w *World) getOrCreateTable(mask Mask) int {
	if index, ok := w.tableLookup[mask]; ok {
		return index
	}
	newIndex := len(w.tables)
	w.tables = append(w.tables, newArchetype(mask, w.registry))
	w.tableEdges = append(w.tableEdges, newTableEdges())
	w.tableLookup[mask] = newIndex

	w.invalidateQueryCacheForNewTable(mask, newIndex)
	w.wireEdgesForNewTable(mask, newIndex)
	return newIndex
}

func (w *World) invalidateQueryCacheForNewTable(newMask Mask, newTableIndex int) {
	for queryMask, cached := range w.queryCache {
		if newMask&queryMask == queryMask {
			w.queryCache[queryMask] = append(cached, newTableIndex)
		}
	}
}

func (w *World) wireEdgesForNewTable(newMask Mask, newTableIndex int) {

	for bit := uint8(0); bit < w.registry.nextBit; bit++ {
		bitMask := Mask(1) << bit
		for existingIndex, existing := range w.tables {
			if existingIndex == newTableIndex {
				continue
			}
			if existing.Mask|bitMask == newMask {
				w.tableEdges[existingIndex].add[bit] = int32(newTableIndex)
			}
			if existing.Mask&^bitMask == newMask {
				w.tableEdges[existingIndex].remove[bit] = int32(newTableIndex)
			}
		}
	}

	for bit := uint8(0); bit < w.registry.nextBit; bit++ {
		bitMask := Mask(1) << bit
		if destIndex, ok := w.tableLookup[newMask|bitMask]; ok {
			w.tableEdges[newTableIndex].add[bit] = int32(destIndex)
		}
		if newMask&bitMask != 0 {
			if destIndex, ok := w.tableLookup[newMask&^bitMask]; ok {
				w.tableEdges[newTableIndex].remove[bit] = int32(destIndex)
			}
		}
	}
}

func (w *World) cachedTables(include Mask) []int {
	if cached, ok := w.queryCache[include]; ok {
		return cached
	}
	matching := make([]int, 0, len(w.tables))
	for index, table := range w.tables {
		if table.Mask&include == include {
			matching = append(matching, index)
		}
	}
	w.queryCache[include] = matching
	return matching
}
