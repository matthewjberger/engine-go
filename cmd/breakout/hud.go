package main

import (
	"math"
	"strconv"

	"github.com/matthewjberger/indigo/app"
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/render"
	"github.com/matthewjberger/indigo/ui"
)

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

	restartButton := b.Node(ui.Node{X: 114, Width: 200, Height: 32}).
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

func updateBreakoutHud(worlds app.Worlds, delta float32) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.MustResource[BreakoutHud](worlds.Engine)
	state := ecs.MustResource[GameState](worlds.Game)

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

func marqueeColor(phase, alpha float32) [4]float32 {
	const twoPiOver3 = 2.0943951
	r := 0.5 + 0.5*float32(math.Sin(float64(phase)))
	g := 0.5 + 0.5*float32(math.Sin(float64(phase+twoPiOver3)))
	bb := 0.5 + 0.5*float32(math.Sin(float64(phase+2*twoPiOver3)))
	return [4]float32{r, g, bb, alpha}
}

func handleBreakoutUiClicks(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.MustResource[BreakoutHud](worlds.Engine)
	state := ecs.MustResource[GameState](worlds.Game)
	for _, evt := range ecs.DrainEvents[ui.EntityClicked](worlds.UI) {
		if evt.Entity == hud.RestartButton && (state.Won || state.Lost) {
			state.RequestReset = true
		}
	}
}

func syncBreakoutUiPointer(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	input := ecs.MustResource[render.Input](worlds.Engine)
	pointer := ecs.MustResource[ui.PointerState](worlds.UI)
	prevDown := pointer.LeftDown
	pointer.X = input.MousePosition[0]
	pointer.Y = input.MousePosition[1]
	pointer.LeftDown = input.LeftDown
	pointer.LeftJustDown = input.LeftDown && !prevDown
	pointer.LeftJustUp = !input.LeftDown && prevDown
}
