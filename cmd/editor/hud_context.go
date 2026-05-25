package main

import (
	"fmt"

	"github.com/matthewjberger/indigo/app"
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/render"
	"github.com/matthewjberger/indigo/transform"
	"github.com/matthewjberger/indigo/ui"
)

type HudContext struct {
	Worlds  app.Worlds
	Engine  *ecs.World
	UI      *ecs.World
	Input   *render.Input
	Hud     *HudHandles
	Pointer *ui.PointerState
	Gizmo   *render.Gizmos
}

func newHudContext(worlds app.Worlds) *HudContext {
	if worlds.UI == nil {
		return nil
	}
	return &HudContext{
		Worlds:  worlds,
		Engine:  worlds.Engine,
		UI:      worlds.UI,
		Input:   ecs.MustResource[render.Input](worlds.Engine),
		Hud:     ecs.MustResource[HudHandles](worlds.Engine),
		Pointer: ecs.MustResource[ui.PointerState](worlds.UI),
		Gizmo:   *ecs.MustResource[*render.Gizmos](worlds.Engine),
	}
}

func (c *HudContext) setText(entity ecs.Entity, content string) {
	if t, ok := ecs.GetMut[ui.Text](c.UI, entity); ok {
		t.Content = content
	}
}

func (c *HudContext) setColor(entity ecs.Entity, rgba [4]float32) {
	if col, ok := ecs.GetMut[ui.Color](c.UI, entity); ok {
		col.RGBA = rgba
	}
}

const caretBlinkPeriodSeconds = 0.5

func caretVisible(uptime float32) bool {
	return int(uptime/caretBlinkPeriodSeconds)%2 == 0
}

func formatVec3(v transform.Vec3) string {
	return fmt.Sprintf("%.2f %.2f %.2f", v[0], v[1], v[2])
}

func formatQuat(q transform.Quat) string {
	return fmt.Sprintf("%.2f %.2f %.2f %.2f", q.V.X(), q.V.Y(), q.V.Z(), q.W)
}

func formatRGBA(c [4]float32) string {
	return fmt.Sprintf("%.2f %.2f %.2f", c[0], c[1], c[2])
}
