package main

import (
	"strconv"

	"indigo/ecs"
	"indigo/render"
)

// pickTitle formats the window title to reflect the latest pick
// result. Used by both native and wasm entry points.
func pickTitle(result *render.PickResult) string {
	if result.EntityID == 0 {
		return "indigo editor, picked: (nothing)"
	}
	return "indigo editor, picked entity " + strconv.FormatUint(uint64(result.EntityID), 10)
}

// applySelection deselects every currently-selected entity and adds
// the Selected tag to the entity matching pickedID (if any). The
// outline post-process picks this up next frame.
func applySelection(engine *ecs.World, pickedID uint32) {
	selectedMask := ecs.MaskOf[render.Selected](engine)
	var toDeselect []ecs.Entity
	engine.ForEach(selectedMask, 0, func(e ecs.Entity, _ *ecs.Archetype, _ int) {
		toDeselect = append(toDeselect, e)
	})
	for _, e := range toDeselect {
		ecs.Remove[render.Selected](engine, e)
	}

	if pickedID == 0 {
		return
	}
	renderMask := ecs.MaskOf[render.RenderMesh](engine)
	var picked ecs.Entity
	found := false
	engine.ForEach(renderMask, 0, func(e ecs.Entity, _ *ecs.Archetype, _ int) {
		if !found && e.ID == pickedID {
			picked = e
			found = true
		}
	})
	if found {
		ecs.Add[render.Selected](engine, picked)
	}
}
