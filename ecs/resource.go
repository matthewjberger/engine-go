package ecs

import "reflect"

func resourceKeyFor[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func SetResource[T any](world *World, value T) {
	key := resourceKeyFor[T]()
	if existing, ok := world.resources[key]; ok {
		*(existing.(*T)) = value
		return
	}
	world.resources[key] = &value
}

func Resource[T any](world *World) (*T, bool) {
	value, ok := world.resources[resourceKeyFor[T]()]
	if !ok {
		return nil, false
	}
	return value.(*T), true
}

func MustResource[T any](world *World) *T {
	r, ok := Resource[T](world)
	if !ok {
		panic("freecs: resource " + resourceKeyFor[T]().String() + " is not set, call SetResource first")
	}
	return r
}

func HasResource[T any](world *World) bool {
	_, ok := world.resources[resourceKeyFor[T]()]
	return ok
}

func RemoveResource[T any](world *World) bool {
	key := resourceKeyFor[T]()
	if _, ok := world.resources[key]; !ok {
		return false
	}
	delete(world.resources, key)
	return true
}
