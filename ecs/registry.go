package ecs

import (
	"fmt"
	"reflect"
)

type Mask uint64

const maxComponents = 64

type componentInfo struct {
	bitIndex uint8
	mask     Mask
	elemType reflect.Type
}

type registry struct {
	byType  map[reflect.Type]*componentInfo
	byBit   [maxComponents]*componentInfo
	nextBit uint8
}

func newRegistry() *registry {
	return &registry{byType: make(map[reflect.Type]*componentInfo)}
}

func (r *registry) registerType(elemType reflect.Type) *componentInfo {
	if info, ok := r.byType[elemType]; ok {
		return info
	}
	if r.nextBit >= maxComponents {
		panic(fmt.Sprintf("freecs: cannot register more than %d component types per world", maxComponents))
	}
	bit := r.nextBit
	r.nextBit++
	info := &componentInfo{
		bitIndex: bit,
		mask:     Mask(1) << bit,
		elemType: elemType,
	}
	r.byType[elemType] = info
	r.byBit[bit] = info
	return info
}

func (r *registry) infoForType(elemType reflect.Type) (*componentInfo, bool) {
	info, ok := r.byType[elemType]
	return info, ok
}

func (r *registry) infoForBit(bit uint8) *componentInfo {
	return r.byBit[bit]
}

func Register[T any](world *World) Mask {
	elemType := reflect.TypeOf((*T)(nil)).Elem()
	info := world.registry.registerType(elemType)
	return info.mask
}

func MaskOf[T any](world *World) (Mask, bool) {
	elemType := reflect.TypeOf((*T)(nil)).Elem()
	info, ok := world.registry.infoForType(elemType)
	if !ok {
		return 0, false
	}
	return info.mask, true
}

func MustMaskOf[T any](world *World) Mask {
	m, ok := MaskOf[T](world)
	if !ok {
		elemType := reflect.TypeOf((*T)(nil)).Elem()
		panic(fmt.Sprintf("freecs: component %s is not registered, call freecs.Register[%s] first", elemType, elemType))
	}
	return m
}

func componentInfoFor[T any](world *World) (*componentInfo, bool) {
	elemType := reflect.TypeOf((*T)(nil)).Elem()
	return world.registry.infoForType(elemType)
}

func mustComponentInfo[T any](world *World) *componentInfo {
	elemType := reflect.TypeOf((*T)(nil)).Elem()
	info, ok := world.registry.infoForType(elemType)
	if !ok {
		panic(fmt.Sprintf("freecs: component %s is not registered, call freecs.Register[%s] first", elemType, elemType))
	}
	return info
}
