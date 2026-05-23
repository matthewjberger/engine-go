package ecs

import (
	"iter"
	"reflect"
)

func tagSetFor[T any](world *World) map[Entity]struct{} {
	key := reflect.TypeOf((*T)(nil)).Elem()
	if set, ok := world.tagSets[key]; ok {
		return set
	}
	set := make(map[Entity]struct{})
	world.tagSets[key] = set
	return set
}

func AddTag[T any](world *World, entity Entity) {
	if _, _, ok := world.entityLocs.get(entity); !ok {
		return
	}
	tagSetFor[T](world)[entity] = struct{}{}
}

func RemoveTag[T any](world *World, entity Entity) bool {
	set := tagSetFor[T](world)
	if _, ok := set[entity]; !ok {
		return false
	}
	delete(set, entity)
	return true
}

func HasTag[T any](world *World, entity Entity) bool {
	_, ok := tagSetFor[T](world)[entity]
	return ok
}

func QueryTag[T any](world *World) iter.Seq[Entity] {
	return func(yield func(Entity) bool) {
		for entity := range tagSetFor[T](world) {
			if !yield(entity) {
				return
			}
		}
	}
}

func CountTag[T any](world *World) int {
	return len(tagSetFor[T](world))
}
