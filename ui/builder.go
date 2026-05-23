package ui

import (
	"github.com/matthewjberger/indigo/ecs"
)

type Builder struct {
	world  *ecs.World
	stack  []ecs.Entity
	cursor ecs.Entity
	hasCur bool
}

func NewBuilder(world *ecs.World) *Builder {
	return &Builder{world: world}
}

func (b *Builder) World() *ecs.World { return b.world }

func (b *Builder) Entity() ecs.Entity { return b.cursor }

func (b *Builder) Push(parent ecs.Entity) *Builder {
	b.stack = append(b.stack, parent)
	return b
}

func (b *Builder) Pop() *Builder {
	if n := len(b.stack); n > 0 {
		b.stack = b.stack[:n-1]
	}
	return b
}

func (b *Builder) Node(node Node) *Builder {
	mask := ecs.MustMaskOf[Node](b.world)
	if len(b.stack) > 0 {
		mask |= ecs.MustMaskOf[Parent](b.world)
	}
	entity := b.world.Spawn(mask)
	ecs.Set(b.world, entity, node)
	if len(b.stack) > 0 {
		ecs.Set(b.world, entity, Parent{Entity: b.stack[len(b.stack)-1]})
	}
	b.cursor = entity
	b.hasCur = true
	return b
}

func (b *Builder) Color(color Color) *Builder {
	if !b.hasCur {
		return b
	}
	ecs.Set(b.world, b.cursor, color)
	return b
}

func (b *Builder) Text(text Text) *Builder {
	if !b.hasCur {
		return b
	}
	ecs.Set(b.world, b.cursor, text)
	return b
}

func (b *Builder) Interactive() *Builder {
	if !b.hasCur {
		return b
	}
	ecs.Set(b.world, b.cursor, Interactive{})
	return b
}

func (b *Builder) Button(node Node, fill Color, label string, labelColor [4]float32) *Builder {
	b.Node(node).Color(fill).Interactive().Text(Text{
		Content: label,
		Color:   labelColor,
		Scale:   1,
	})
	return b
}
