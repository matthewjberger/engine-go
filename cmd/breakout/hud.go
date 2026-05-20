package main

import (
	"math"
	"strconv"

	"indigo/app"
	"indigo/ecs"
	"indigo/render"
	"indigo/ui"
)

// BreakoutHud holds the UI entities that need per-frame mutation
// from game state. Stored as a resource on the engine world so the
// per-frame helper can update score/lives text + flash the win/lose
// banner without rebuilding the tree.
type BreakoutHud struct {
	ScorePanel    ecs.Entity
	ScoreLabel    ecs.Entity
	LivesPanel    ecs.Entity
	LivesLabel    ecs.Entity
	HintLabel     ecs.Entity
	BannerPanel   ecs.Entity
	BannerLabel   ecs.Entity
	RestartButton ecs.Entity

	MarqueePhase float32
}

// buildBreakoutHud lays out the in-game HUD: a score panel anchored
// to the top-left, a lives panel mirrored to the top-right, a launch
// hint pinned to the bottom-center, and a centered banner that flips
// on at win/lose with a color-cycling label plus a Restart button.
// All entities exist from frame zero; update_hud below toggles their
// visibility by writing into their Color / Text components.
func buildBreakoutHud(world *ecs.World) BreakoutHud {
	b := ui.NewBuilder(world)

	scorePanel := b.Node(ui.Node{
		X: 16, Y: 16, Width: 160, Height: 40,
		Anchor: ui.AnchorTopLeft, Padding: 8, Layout: ui.LayoutColumn,
	}).Color(ui.Color{RGBA: [4]float32{0.08, 0.09, 0.12, 0.8}}).Entity()
	b.Push(scorePanel)
	scoreLabel := b.Node(ui.Node{Width: 144, Height: 20}).Text(ui.Text{
		Content: "SCORE 0",
		Color:   [4]float32{0.95, 0.96, 0.98, 1},
		Scale:   2.0,
	}).Entity()
	b.Pop()

	livesPanel := b.Node(ui.Node{
		X: 16, Y: 16, Width: 160, Height: 40,
		Anchor: ui.AnchorTopRight, Padding: 8, Layout: ui.LayoutColumn,
	}).Color(ui.Color{RGBA: [4]float32{0.08, 0.09, 0.12, 0.8}}).Entity()
	b.Push(livesPanel)
	livesLabel := b.Node(ui.Node{Width: 144, Height: 20}).Text(ui.Text{
		Content: "LIVES 3",
		Color:   [4]float32{0.95, 0.96, 0.98, 1},
		Scale:   2.0,
	}).Entity()
	b.Pop()

	hintLabel := b.Node(ui.Node{
		X: 0, Y: 48, Width: 360, Height: 24,
		Anchor: ui.AnchorBottomLeft,
	}).Text(ui.Text{
		Content: "A and D to move, space to launch",
		Color:   [4]float32{0.85, 0.88, 0.95, 0.85},
		Scale:   1.6,
	}).Entity()

	bannerPanel := b.Node(ui.Node{
		X: 0, Y: -40, Width: 460, Height: 120,
		Anchor: ui.AnchorCenter, Padding: 16, Layout: ui.LayoutColumn, Spacing: 10,
	}).Color(ui.Color{RGBA: [4]float32{0, 0, 0, 0}}).Entity()
	b.Push(bannerPanel)
	bannerLabel := b.Node(ui.Node{Width: 428, Height: 36}).Text(ui.Text{
		Content: "",
		Color:   [4]float32{1, 1, 1, 0},
		Scale:   4.5,
	}).Entity()
	restartButton := b.Node(ui.Node{Width: 200, Height: 32}).
		Color(ui.Color{RGBA: [4]float32{0, 0, 0, 0}}).
		Interactive().
		Text(ui.Text{
			Content: "",
			Color:   [4]float32{1, 1, 1, 0},
			Scale:   2.0,
		}).Entity()
	b.Pop()

	return BreakoutHud{
		ScorePanel:    scorePanel,
		ScoreLabel:    scoreLabel,
		LivesPanel:    livesPanel,
		LivesLabel:    livesLabel,
		HintLabel:     hintLabel,
		BannerPanel:   bannerPanel,
		BannerLabel:   bannerLabel,
		RestartButton: restartButton,
	}
}

// updateBreakoutHud rewrites the HUD's per-frame state from the
// current GameState: score + lives text on the corner panels, hint
// fades out once play starts, banner becomes visible when the round
// ends with VICTORY / GAME OVER and a Restart button, and the banner
// label color cycles via a sine-wave marquee.
func updateBreakoutHud(worlds app.Worlds, delta float32) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[BreakoutHud](worlds.Engine)
	state := ecs.Resource[GameState](worlds.Game)

	if label, ok := ecs.GetMut[ui.Text](worlds.UI, hud.ScoreLabel); ok {
		label.Content = "SCORE " + strconv.Itoa(state.Score)
	}
	if label, ok := ecs.GetMut[ui.Text](worlds.UI, hud.LivesLabel); ok {
		label.Content = "LIVES " + strconv.Itoa(state.Lives)
	}
	if hint, ok := ecs.GetMut[ui.Text](worlds.UI, hud.HintLabel); ok {
		if state.Started {
			hint.Color[3] = 0
		} else {
			hint.Color[3] = 0.85
		}
	}

	roundOver := state.Won || state.Lost
	if panel, ok := ecs.GetMut[ui.Color](worlds.UI, hud.BannerPanel); ok {
		if roundOver {
			panel.RGBA = [4]float32{0.06, 0.07, 0.1, 0.92}
		} else {
			panel.RGBA = [4]float32{0, 0, 0, 0}
		}
	}
	if button, ok := ecs.GetMut[ui.Color](worlds.UI, hud.RestartButton); ok {
		if roundOver {
			button.RGBA = [4]float32{0.2, 0.48, 0.86, 1}
		} else {
			button.RGBA = [4]float32{0, 0, 0, 0}
		}
	}
	if label, ok := ecs.GetMut[ui.Text](worlds.UI, hud.RestartButton); ok {
		if roundOver {
			label.Content = "RESTART"
			label.Color = [4]float32{0.98, 0.98, 1, 1}
		} else {
			label.Content = ""
			label.Color[3] = 0
		}
	}
	if label, ok := ecs.GetMut[ui.Text](worlds.UI, hud.BannerLabel); ok {
		if state.Won {
			label.Content = "VICTORY!"
			label.Color = marqueeColor(hud.MarqueePhase, 1)
		} else if state.Lost {
			label.Content = "GAME OVER"
			label.Color = marqueeColor(hud.MarqueePhase, 1)
		} else {
			label.Content = ""
			label.Color[3] = 0
		}
	}

	if roundOver {
		hud.MarqueePhase += delta * 3.5
	} else {
		hud.MarqueePhase = 0
	}
}

// marqueeColor returns an RGB color cycling through the hue wheel
// at the given phase. The three channels are sine waves with 120°
// phase offsets so the result sweeps red -> yellow -> green ->
// cyan -> blue -> magenta -> red over a 2*pi-phase period.
func marqueeColor(phase, alpha float32) [4]float32 {
	const twoPiOver3 = 2.0943951
	r := 0.5 + 0.5*float32(math.Sin(float64(phase)))
	g := 0.5 + 0.5*float32(math.Sin(float64(phase+twoPiOver3)))
	bb := 0.5 + 0.5*float32(math.Sin(float64(phase+2*twoPiOver3)))
	return [4]float32{r, g, bb, alpha}
}

// handleBreakoutUiClicks drains UI EntityClicked events from the UI
// world and dispatches them. Today the only interactive widget is
// the Restart button, which flips [GameState.RequestReset] so the
// next frame's breakoutResetSystem rebuilds the wall.
func handleBreakoutUiClicks(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[BreakoutHud](worlds.Engine)
	state := ecs.Resource[GameState](worlds.Game)
	for _, evt := range ecs.DrainEvents[ui.EntityClicked](worlds.UI) {
		if evt.Entity == hud.RestartButton && (state.Won || state.Lost) {
			state.RequestReset = true
		}
	}
}

// syncBreakoutUiPointer mirrors editor's helper: copies the engine
// mouse + left button edges into the UI world's pointer state so the
// interaction system sees this frame's input.
func syncBreakoutUiPointer(worlds app.Worlds) {
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
