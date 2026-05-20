package main

import (
	"indigo/app"
	"indigo/ecs"
	"indigo/render"
	"indigo/ui"
)

// syncUiPointer copies the engine-side mouse position + left button
// edges into the UI world's [ui.PointerState] before TickFrame runs,
// so the UI interaction system sees this frame's pointer. Called by
// both the native and wasm entry points.
func syncUiPointer(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	input := ecs.Resource[render.Input](worlds.Engine)
	pointer := ecs.Resource[ui.PointerState](worlds.UI)
	prevDown := pointer.LeftDown
	pointer.X = input.MousePosition[0]
	pointer.Y = input.MousePosition[1]
	pointer.LeftDown = input.LeftDown
	pointer.LeftJustDown = input.LeftDown && !prevDown
	pointer.LeftJustUp = !input.LeftDown && prevDown
}

// handleUiClicks drains EntityClicked events from the UI world and
// dispatches them. The editor's only interactive widget today is the
// Clear button, which deselects whatever entity was picked.
func handleUiClicks(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	for _, evt := range ecs.DrainEvents[ui.EntityClicked](worlds.UI) {
		if evt.Entity == hud.ClearButton {
			applySelection(worlds.Engine, 0)
			refreshHudLabel(worlds, 0)
		}
	}
}

// refreshHudLabel rewrites the status label's Text.Content to match
// the latest pick result. Passes nothing when entityID==0.
func refreshHudLabel(worlds app.Worlds, entityID uint32) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	label, ok := ecs.GetMut[ui.Text](worlds.UI, hud.StatusLabel)
	if !ok {
		return
	}
	if entityID == 0 {
		label.Content = "Pick a cube"
	} else {
		label.Content = "Selected " + strconvUint(entityID)
	}
}

func strconvUint(v uint32) string {
	if v == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
