package render

import (
	"indigo/ecs"
	"indigo/ui"
)

// GraphicsSettings is the engine's runtime toggle resource. Each pass
// reads the relevant bool in Prepare/Execute and skips its work when
// disabled. Ported from nightshade's `world.resources.graphics.*`
// fields, scaled down to the toggles our current passes care about.
type GraphicsSettings struct {
	ShowSky     bool
	ShowGrid    bool
	FxaaEnabled bool
	ShowBounds  bool
	ShowNormals bool

	// NormalLineLength scales the per-vertex normal line emitted
	// when ShowNormals is true. World units.
	NormalLineLength float32

	// NormalLineColor is the RGBA used for the debug normal lines.
	NormalLineColor [4]float32
}

// DefaultGraphicsSettings returns settings with everything enabled
// except the debug bounding-volume overlay.
func DefaultGraphicsSettings() GraphicsSettings {
	return GraphicsSettings{
		ShowSky:          true,
		ShowGrid:         true,
		FxaaEnabled:      true,
		ShowBounds:       false,
		ShowNormals:      false,
		NormalLineLength: 0.08,
		NormalLineColor:  [4]float32{1.0, 0.92, 0.2, 0.95},
	}
}

// UpdateGraphicsToggles is the engine system that flips graphics
// settings based on this frame's keyboard input. Press G to toggle the
// grid, S to toggle the sky, F to toggle FXAA. Reads the keys-just-
// down slice from [Input] and clears nothing; [Input.BeginFrame]
// resets the just-pressed slice each frame.
func UpdateGraphicsToggles(world *ecs.World) {
	if ecs.HasResource[ui.WorldRef](world) {
		if ui.AnyTextInputFocused(ecs.MustResource[ui.WorldRef](world).World) {
			return
		}
	}
	input := ecs.MustResource[Input](world)
	settings := ecs.MustResource[GraphicsSettings](world)
	for _, key := range input.KeysJustDown {
		switch key {
		case 'G':
			settings.ShowGrid = !settings.ShowGrid
		case 'S':
			settings.ShowSky = !settings.ShowSky
		case 'F':
			settings.FxaaEnabled = !settings.FxaaEnabled
		case 'B':
			settings.ShowBounds = !settings.ShowBounds
		case 'N':
			settings.ShowNormals = !settings.ShowNormals
		}
	}
}
