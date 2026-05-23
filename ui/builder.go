package ui

import (
	"github.com/matthewjberger/indigo/ecs"
)

// Builder is a thin imperative DSL for assembling a UI tree on a UI
// world. Each method spawns at most one entity, attaches the
// supplied components, and tracks parent/child relationships so the
// caller can build trees with a fluent chain without manually
// passing Entity values.
//
// The entities Builder produces are ordinary ECS rows that any
// system can query.
//
// Usage:
//
//	b := ui.NewBuilder(world)
//	panel := b.Node(ui.Node{X: 20, Y: 20, Width: 240, Height: 80, Layout: ui.LayoutRow, Padding: 8, Spacing: 8}).
//	    Color(ui.Color{RGBA: [4]float32{0.1, 0.1, 0.12, 0.85}}).
//	    Entity()
//	b.Push(panel)
//	b.Button(ui.Node{Width: 96, Height: 32}, ui.Color{RGBA: [4]float32{0.18, 0.5, 0.8, 1}}, "Pick", ui.OutlineColor)
//	b.Pop()
type Builder struct {
	world  *ecs.World
	stack  []ecs.Entity
	cursor ecs.Entity
	hasCur bool
}

// NewBuilder returns a builder anchored to world.
func NewBuilder(world *ecs.World) *Builder {
	return &Builder{world: world}
}

// World returns the world the builder is operating on.
func (b *Builder) World() *ecs.World { return b.world }

// Entity returns the most-recently-created entity, or the zero
// Entity if Builder hasn't spawned anything yet. Useful right after
// a Node / Button / Text call.
func (b *Builder) Entity() ecs.Entity { return b.cursor }

// Push pushes parent onto the parent stack. Subsequent entities
// created without an explicit parent become children of parent.
func (b *Builder) Push(parent ecs.Entity) *Builder {
	b.stack = append(b.stack, parent)
	return b
}

// Pop discards the current parent. No-op if the stack is empty.
func (b *Builder) Pop() *Builder {
	if n := len(b.stack); n > 0 {
		b.stack = b.stack[:n-1]
	}
	return b
}

// Node spawns a bare UI entity carrying just a [Node] component and
// (if a parent is on the stack) a [Parent] link.
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

// Color attaches a [Color] component to the current entity.
func (b *Builder) Color(color Color) *Builder {
	if !b.hasCur {
		return b
	}
	ecs.Set(b.world, b.cursor, color)
	return b
}

// Text attaches a [Text] component to the current entity.
func (b *Builder) Text(text Text) *Builder {
	if !b.hasCur {
		return b
	}
	ecs.Set(b.world, b.cursor, text)
	return b
}

// Interactive attaches an [Interactive] component so the entity
// receives Hovered/Pressed updates and emits [EntityClicked]
// events.
func (b *Builder) Interactive() *Builder {
	if !b.hasCur {
		return b
	}
	ecs.Set(b.world, b.cursor, Interactive{})
	return b
}

// Button is a one-shot convenience that spawns a colored,
// interactive rectangle carrying a label centered inside it. Returns
// the builder so further calls can add siblings.
func (b *Builder) Button(node Node, fill Color, label string, labelColor [4]float32) *Builder {
	b.Node(node).Color(fill).Interactive().Text(Text{
		Content: label,
		Color:   labelColor,
		Scale:   1,
	})
	return b
}
