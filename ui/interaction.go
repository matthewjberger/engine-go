package ui

import (
	"github.com/matthewjberger/indigo/ecs"
)

type PointerState struct {
	X, Y          float32
	LeftDown      bool
	LeftJustDown  bool
	LeftJustUp    bool
	RightDown     bool
	RightJustDown bool
	RightJustUp   bool
	OverUI        bool

	FocusedEntity ecs.Entity

	pressedEntity ecs.Entity
	hasPressed    bool
}

func InteractionSystem(world *ecs.World) {
	pointer := ecs.MustResource[PointerState](world)

	mask := ecs.MustMaskOf[Node](world) | ecs.MustMaskOf[Interactive](world)

	var hit ecs.Entity
	hasHit := false
	bestArea := float32(0)
	world.ForEach(mask, 0, func(entity ecs.Entity, _ *ecs.Archetype, _ int) {
		node, _ := ecs.Get[Node](world, entity)
		if !node.Resolved.Contains(pointer.X, pointer.Y) {
			return
		}

		if color, ok := ecs.Get[Color](world, entity); ok {
			if color.RGBA[3] <= 0.001 {
				return
			}
		}

		area := node.Resolved.Width * node.Resolved.Height
		if !hasHit || area < bestArea {
			hit = entity
			hasHit = true
			bestArea = area
		}
	})
	pointer.OverUI = hasHit

	var clearList, hoverList, pressList []ecs.Entity
	world.ForEach(mask, 0, func(entity ecs.Entity, _ *ecs.Archetype, _ int) {
		if hasHit && entity == hit {
			hoverList = append(hoverList, entity)
			if pointer.LeftDown {
				pressList = append(pressList, entity)
			}
		} else {
			clearList = append(clearList, entity)
		}
	})

	for _, e := range clearList {
		if i, ok := ecs.GetMut[Interactive](world, e); ok {
			i.Hovered = false
			i.Pressed = false
		}
	}
	for _, e := range hoverList {
		if i, ok := ecs.GetMut[Interactive](world, e); ok {
			i.Hovered = true
			i.Pressed = pointer.LeftDown
		}
	}
	_ = pressList

	if pointer.LeftJustDown {
		if hasHit {
			pointer.pressedEntity = hit
			pointer.hasPressed = true
			pointer.FocusedEntity = hit
		} else {
			pointer.FocusedEntity = ecs.Entity{}
		}
	}
	if pointer.LeftJustUp {
		if pointer.hasPressed && hasHit && pointer.pressedEntity == hit {
			ecs.Send(world, EntityClicked{Entity: hit})
		}
		pointer.hasPressed = false
	}
}
