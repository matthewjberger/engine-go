package main

import (
	"fmt"
	"strings"

	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/ui"
)

const (
	khronosRowCount            = 16
	khronosPanelZ      int32   = 60
	khronosContentZ    int32   = 61
	khronosRowHeight   float32 = 22
	khronosRowStride   float32 = khronosRowHeight + 4
	khronosPanelX      float32 = hudLeftPanelWidth + 16
	khronosPanelY      float32 = hudTopBarHeight + 16
	khronosPanelWidth  float32 = 360
	khronosPanelHeight float32 = 520
)

var (
	khronosTitleColor  = [4]float32{0.72, 0.78, 0.88, 1}
	khronosStatusColor = [4]float32{0.62, 0.66, 0.74, 1}
	khronosRowColor    = [4]float32{0.9, 0.92, 0.96, 1}
	khronosRowFill     = [4]float32{0.10, 0.11, 0.14, 1}
	khronosPanelFill   = [4]float32{0.07, 0.08, 0.11, 1}
	khronosSearchFill  = [4]float32{0.12, 0.13, 0.16, 1}
	khronosHidden      = [4]float32{0, 0, 0, 0}
)

type khronosHandles struct {
	Panel       ecs.Entity
	Title       ecs.Entity
	SearchField ecs.Entity
	StatusLabel ecs.Entity
	Rows        [khronosRowCount]ecs.Entity
	RowToEntry  [khronosRowCount]int
}

func buildKhronosPanel(b *ui.Builder) khronosHandles {
	var h khronosHandles
	contentWidth := khronosPanelWidth - 20

	h.Panel = b.Node(ui.Node{
		X: khronosPanelX, Y: khronosPanelY,
		Width: khronosPanelWidth, Height: khronosPanelHeight,
		Anchor:  ui.AnchorTopLeft,
		Layout:  ui.LayoutColumn,
		Padding: 10, Spacing: 4,
		ZIndex: khronosPanelZ,
	}).Color(ui.Color{RGBA: khronosHidden}).Interactive().Entity()

	b.Push(h.Panel)
	h.Title = b.Node(ui.Node{Width: contentWidth, Height: 22, ZIndex: khronosContentZ}).
		Text(ui.Text{Content: "KHRONOS SAMPLE ASSETS", Color: khronosHidden, Scale: 1.5}).Entity()

	h.SearchField = b.Node(ui.Node{Width: contentWidth, Height: 24, ZIndex: khronosContentZ}).
		Color(ui.Color{RGBA: khronosHidden}).
		Interactive().
		Text(ui.Text{Content: "", Color: khronosHidden, Scale: 1.4}).Entity()
	ecs.Set(b.World(), h.SearchField, ui.TextInput{})

	h.StatusLabel = b.Node(ui.Node{Width: contentWidth, Height: 18, ZIndex: khronosContentZ}).
		Text(ui.Text{Content: "", Color: khronosHidden, Scale: 1.3}).Entity()

	for i := 0; i < khronosRowCount; i++ {
		h.Rows[i] = b.Node(ui.Node{Width: contentWidth, Height: khronosRowHeight, ZIndex: khronosContentZ}).
			Color(ui.Color{RGBA: khronosHidden}).
			Interactive().
			Text(ui.Text{Content: "", Color: khronosHidden, Scale: 1.3}).Entity()
		h.RowToEntry[i] = -1
	}
	b.Pop()

	return h
}

func (c *HudContext) refreshKhronosBrowser() {
	h := &c.Hud.Khronos
	if !c.Hud.KhronosOpen {
		c.hideKhronosPanel()
		return
	}

	browser := *ecs.MustResource[*KhronosBrowser](c.Engine)
	browser.EnsureLoaded()

	c.setColor(h.Panel, khronosPanelFill)
	setLabel(c.UI, h.Title, "KHRONOS SAMPLE ASSETS", khronosTitleColor)
	c.setColor(h.SearchField, khronosSearchFill)
	if text, ok := ecs.GetMut[ui.Text](c.UI, h.SearchField); ok {
		text.Color = khronosRowColor
	}

	needle := ""
	if ti, ok := ecs.Get[ui.TextInput](c.UI, h.SearchField); ok {
		needle = strings.ToLower(ti.Buffer)
	}

	entries := browser.Entries()
	if entries == nil {
		c.clearKhronosRows()
		if message := browser.IndexError(); message != "" {
			setLabel(c.UI, h.StatusLabel, "FAILED: "+strings.ToUpper(message), khronosStatusColor)
		} else {
			setLabel(c.UI, h.StatusLabel, "LOADING INDEX...", khronosStatusColor)
		}
		return
	}

	filtered := make([]int, 0, len(entries))
	for index, entry := range entries {
		if needle == "" ||
			strings.Contains(strings.ToLower(entry.Label), needle) ||
			strings.Contains(strings.ToLower(entry.Name), needle) {
			filtered = append(filtered, index)
		}
	}

	offset := c.updateKhronosScroll(len(filtered))
	for i := 0; i < khronosRowCount; i++ {
		listIndex := i + offset
		if listIndex < len(filtered) {
			entryIndex := filtered[listIndex]
			c.setColor(h.Rows[i], khronosRowFill)
			setLabel(c.UI, h.Rows[i], entries[entryIndex].Label, khronosRowColor)
			h.RowToEntry[i] = entryIndex
		} else {
			c.setColor(h.Rows[i], khronosHidden)
			setLabel(c.UI, h.Rows[i], "", khronosHidden)
			h.RowToEntry[i] = -1
		}
	}

	if loading := browser.LoadingStatus(); loading != "" {
		setLabel(c.UI, h.StatusLabel, "LOADING "+strings.ToUpper(loading)+"...", khronosStatusColor)
	} else {
		setLabel(c.UI, h.StatusLabel, fmt.Sprintf("%d / %d ASSETS", len(filtered), len(entries)), khronosStatusColor)
	}
}

// updateKhronosScroll consumes wheel input while the pointer is over the panel
// and returns the first visible filtered index. The pan-orbit camera already
// ignores wheel input when the pointer is over UI, so consuming it here does
// not double-act as a camera zoom.
func (c *HudContext) updateKhronosScroll(itemCount int) int {
	if c.Input.Wheel != 0 {
		if node, ok := ecs.Get[ui.Node](c.UI, c.Hud.Khronos.Panel); ok &&
			node.Resolved.Contains(c.Pointer.X, c.Pointer.Y) {
			c.Hud.KhronosScrollPixels -= c.Input.Wheel * 40
			c.Input.Wheel = 0
		}
	}
	maxScroll := itemCount - khronosRowCount
	if maxScroll < 0 {
		maxScroll = 0
	}
	maxPixels := float32(maxScroll) * khronosRowStride
	if c.Hud.KhronosScrollPixels > maxPixels {
		c.Hud.KhronosScrollPixels = maxPixels
	}
	if c.Hud.KhronosScrollPixels < 0 {
		c.Hud.KhronosScrollPixels = 0
	}
	return int(c.Hud.KhronosScrollPixels / khronosRowStride)
}

func (c *HudContext) clearKhronosRows() {
	h := &c.Hud.Khronos
	for i := 0; i < khronosRowCount; i++ {
		c.setColor(h.Rows[i], khronosHidden)
		setLabel(c.UI, h.Rows[i], "", khronosHidden)
		h.RowToEntry[i] = -1
	}
}

func (c *HudContext) hideKhronosPanel() {
	h := &c.Hud.Khronos
	c.setColor(h.Panel, khronosHidden)
	c.setColor(h.SearchField, khronosHidden)
	fadeText(c.UI, h.Title)
	fadeText(c.UI, h.SearchField)
	fadeText(c.UI, h.StatusLabel)
	c.clearKhronosRows()
}

func setLabel(world *ecs.World, entity ecs.Entity, content string, color [4]float32) {
	if text, ok := ecs.GetMut[ui.Text](world, entity); ok {
		text.Content = content
		text.Color = color
	}
}

func fadeText(world *ecs.World, entity ecs.Entity) {
	if text, ok := ecs.GetMut[ui.Text](world, entity); ok {
		text.Color[3] = 0
	}
}
