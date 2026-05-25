package asset

import (
	"fmt"
	"unsafe"

	"github.com/cogentcore/webgpu/wgpu"

	"github.com/matthewjberger/indigo/ecs"
)

type MaterialRegistry struct {
	device   *wgpu.Device
	buffer   *wgpu.Buffer
	capacity uint32
	count    uint32

	entityToID map[ecs.Entity]uint32
	idToEntity []ecs.Entity
}

type MaterialRegistryResource struct {
	Registry *MaterialRegistry
}

const minMaterialCapacity uint32 = 256

func NewMaterialRegistry(device *wgpu.Device) (*MaterialRegistry, error) {
	registry := &MaterialRegistry{
		device:     device,
		capacity:   minMaterialCapacity,
		entityToID: make(map[ecs.Entity]uint32),
	}
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "global material registry",
		Size:  uint64(registry.capacity) * MaterialGPUSize,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst | wgpu.BufferUsageCopySrc,
	})
	if err != nil {
		return nil, fmt.Errorf("material registry: buffer: %w", err)
	}
	registry.buffer = buffer
	return registry, nil
}

func (r *MaterialRegistry) Buffer() *wgpu.Buffer { return r.buffer }

func (r *MaterialRegistry) AssignID(entity ecs.Entity) (uint32, bool) {
	if id, ok := r.entityToID[entity]; ok {
		return id, false
	}
	id := r.count
	r.entityToID[entity] = id
	r.idToEntity = append(r.idToEntity, entity)
	r.count++
	return id, true
}

func (r *MaterialRegistry) Release(entity ecs.Entity) {
	id, ok := r.entityToID[entity]
	if !ok {
		return
	}
	delete(r.entityToID, entity)
	last := r.count - 1
	if id != last {
		moved := r.idToEntity[last]
		r.idToEntity[id] = moved
		r.entityToID[moved] = id
	}
	r.idToEntity = r.idToEntity[:last]
	r.count = last
}

func (r *MaterialRegistry) IDFor(entity ecs.Entity) (uint32, bool) {
	id, ok := r.entityToID[entity]
	return id, ok
}

func (r *MaterialRegistry) EntityFor(id uint32) (ecs.Entity, bool) {
	if id >= r.count {
		return ecs.Entity{}, false
	}
	return r.idToEntity[id], true
}

func (r *MaterialRegistry) Count() uint32 { return r.count }

func (r *MaterialRegistry) Capacity() uint32 { return r.capacity }

func (r *MaterialRegistry) EnsureCapacity(required uint32) (bool, error) {
	if r.capacity >= required {
		return false, nil
	}
	newCapacity := r.capacity
	if newCapacity == 0 {
		newCapacity = minMaterialCapacity
	}
	for newCapacity < required {
		newCapacity *= 2
	}
	newBuffer, err := r.device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "global material registry",
		Size:  uint64(newCapacity) * MaterialGPUSize,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst | wgpu.BufferUsageCopySrc,
	})
	if err != nil {
		return false, fmt.Errorf("material registry: grow: %w", err)
	}
	if r.buffer != nil {
		r.buffer.Release()
	}
	r.buffer = newBuffer
	r.capacity = newCapacity
	return true, nil
}

func (r *MaterialRegistry) Write(queue *wgpu.Queue, id uint32, gpu MaterialGPU) {
	r.writeAt(queue, uint64(id)*MaterialGPUSize, unsafe.Slice((*byte)(unsafe.Pointer(&gpu)), MaterialGPUSize))
}

func (r *MaterialRegistry) ReleaseResources() {
	if r.buffer != nil {
		r.buffer.Release()
		r.buffer = nil
	}
}
