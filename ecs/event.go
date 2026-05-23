package ecs

import "reflect"

type eventDriver interface {
	update()
}

type eventQueue[T any] struct {
	current  []T
	previous []T
}

func (q *eventQueue[T]) update() {
	q.previous = q.previous[:0]
	q.current, q.previous = q.previous, q.current
}

func eventQueueFor[T any](world *World) *eventQueue[T] {
	key := reflect.TypeOf((*T)(nil)).Elem()
	if index, ok := world.eventByType[key]; ok {
		return world.eventQueues[index].(*eventQueue[T])
	}
	queue := &eventQueue[T]{}
	world.eventByType[key] = len(world.eventQueues)
	world.eventQueues = append(world.eventQueues, queue)
	return queue
}

func Send[T any](world *World, event T) {
	queue := eventQueueFor[T](world)
	queue.current = append(queue.current, event)
}

func ReadEvents[T any](world *World) []T {
	queue := eventQueueFor[T](world)
	if len(queue.previous) == 0 {
		return queue.current
	}
	if len(queue.current) == 0 {
		return queue.previous
	}
	out := make([]T, 0, len(queue.previous)+len(queue.current))
	out = append(out, queue.previous...)
	out = append(out, queue.current...)
	return out
}

func DrainEvents[T any](world *World) []T {
	queue := eventQueueFor[T](world)
	out := make([]T, 0, len(queue.previous)+len(queue.current))
	out = append(out, queue.previous...)
	out = append(out, queue.current...)
	queue.previous = queue.previous[:0]
	queue.current = queue.current[:0]
	return out
}

func ClearEvents[T any](world *World) {
	queue := eventQueueFor[T](world)
	queue.previous = queue.previous[:0]
	queue.current = queue.current[:0]
}

func LenEvents[T any](world *World) int {
	queue := eventQueueFor[T](world)
	return len(queue.previous) + len(queue.current)
}

func PeekEvent[T any](world *World) (T, bool) {
	queue := eventQueueFor[T](world)
	if len(queue.previous) > 0 {
		return queue.previous[0], true
	}
	if len(queue.current) > 0 {
		return queue.current[0], true
	}
	var zero T
	return zero, false
}
