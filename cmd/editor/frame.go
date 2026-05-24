package main

import (
	"github.com/matthewjberger/indigo/app"
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/internal/render/pass"
	"github.com/matthewjberger/indigo/render"
)

// updateEditorFrame runs the per-frame editor update shared by the native and
// wasm loops: drain pending downloads, sync UI input, tick the worlds, and
// refresh the HUD. It returns whether the user requested the editor to exit.
func updateEditorFrame(worlds app.Worlds, renderer *render.Renderer, demo *app.App, delta float32) bool {
	drainKhronosPending(worlds, renderer)
	syncUiPointer(worlds)
	ctx := newHudContext(worlds)
	ctx.refreshHudLayout()
	ctx.updateTreeScroll()

	app.TickFrame(worlds, demo, delta)
	handleRightClick(worlds)
	driveTextInputs(worlds)
	handleUiClicks(worlds)
	ctx.refreshFps()
	ctx.refreshModeButtons()
	ctx.refreshMenuPopups()
	ctx.refreshInteractiveHovers()
	ctx.refreshEntityTree()
	ctx.refreshInspector()
	ctx.updateInspectorCaret()
	ctx.refreshKhronosBrowser()

	return ctx.Hud.RequestExit
}

// finishEditorFrame runs the post-render tail shared by both loops: process the
// picking readback, apply any pick result, and advance to the next frame.
func finishEditorFrame(worlds app.Worlds, renderer *render.Renderer) {
	pass.ProcessPickingReadback(renderer, worlds.Engine)
	if picking := ecs.MustResource[*pass.Picking](worlds.Engine); (*picking).Result != nil {
		result := (*picking).Result
		(*picking).Result = nil
		handlePickResult(worlds, result.EntityID)
	}
	app.PostFrame(worlds)
}
