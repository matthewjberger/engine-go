package ecs

type MultiWorld struct {
	_         noCopy
	allocator allocator
	worlds    []*World
	live      map[Entity]struct{}
}

func NewMultiWorld() *MultiWorld {
	return &MultiWorld{live: make(map[Entity]struct{})}
}

func (m *MultiWorld) NewWorld() *World {
	world := newWorldWithAllocator(&m.allocator)
	m.worlds = append(m.worlds, world)
	return world
}

func (m *MultiWorld) Worlds() []*World { return m.worlds }

func (m *MultiWorld) Spawn() Entity {
	entity := m.allocator.allocate()
	m.live[entity] = struct{}{}
	return entity
}

func (m *MultiWorld) Despawn(entity Entity) bool {
	if _, ok := m.live[entity]; !ok {
		return false
	}
	delete(m.live, entity)
	for _, world := range m.worlds {
		despawnFromArchetype(world, entity)
	}
	m.allocator.deallocate(entity)
	return true
}

func (m *MultiWorld) IsLive(entity Entity) bool {
	_, ok := m.live[entity]
	return ok
}

func (m *MultiWorld) Step() {
	for _, world := range m.worlds {
		world.Step()
	}
}
