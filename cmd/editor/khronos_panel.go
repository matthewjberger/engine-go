package main

import (
	"fmt"
	"strings"

	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/ui"
)

const (
	khronosPanelZ      int32   = 60
	khronosContentZ    int32   = 61
	khronosRowHeight   float32 = 22
	khronosPanelX      float32 = hudLeftPanelWidth + 16
	khronosPanelY      float32 = hudTopBarHeight + 16
	khronosPanelWidth  float32 = 360
	khronosPanelHeight float32 = 520
	khronosListHeight  float32 = 420
)

var (
	khronosTitleColor  = [4]float32{0.72, 0.78, 0.88, 1}
	khronosStatusColor = [4]float32{0.62, 0.66, 0.74, 1}
	khronosRowColor    = [4]float32{0.9, 0.92, 0.96, 1}
	khronosRowFill     = [4]float32{0.10, 0.11, 0.14, 1}
	khronosPanelFill   = [4]float32{0.07, 0.08, 0.11, 1}
	khronosSearchFill  = [4]float32{0.12, 0.13, 0.16, 1}
)

type khronosHandles struct {
	Panel       ecs.Entity
	Title       ecs.Entity
	SearchField ecs.Entity
	SearchCaret ecs.Entity
	StatusLabel ecs.Entity
	Scroll      ui.ScrollArea
	Rows        []ecs.Entity
	RowToEntry  []int
	LastFilter  string
	LastCount   int
	Built       bool
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
		Hidden: true,
	}).Color(ui.Color{RGBA: khronosPanelFill}).Interactive().Entity()

	b.Push(h.Panel)
	h.Title = b.Node(ui.Node{Width: contentWidth, Height: 22, ZIndex: khronosContentZ}).
		Text(ui.Text{Content: "KHRONOS SAMPLE ASSETS", Color: khronosTitleColor, Scale: 1.5}).Entity()

	h.SearchField = b.Node(ui.Node{Width: contentWidth, Height: 24, ZIndex: khronosContentZ}).
		Color(ui.Color{RGBA: khronosSearchFill}).
		Interactive().
		Text(ui.Text{Content: "", Color: khronosRowColor, Scale: 1.4}).Entity()
	ecs.Set(b.World(), h.SearchField, ui.TextInput{})
	b.Push(h.SearchField)
	h.SearchCaret = b.Node(ui.Node{
		X: 0, Y: 0, Width: 2, Height: 14,
		Anchor: ui.AnchorTopLeft,
		ZIndex: khronosContentZ + 10,
	}).Color(ui.Color{RGBA: [4]float32{0, 0, 0, 0}}).Entity()
	b.Pop()

	h.StatusLabel = b.Node(ui.Node{Width: contentWidth, Height: 18, ZIndex: khronosContentZ}).
		Text(ui.Text{Content: "", Color: khronosStatusColor, Scale: 1.3}).Entity()

	h.Scroll = ui.NewScrollArea(b, contentWidth, khronosListHeight)
	b.Pop()

	return h
}

func (c *HudContext) refreshKhronosBrowser() {
	h := &c.Hud.Khronos
	ui.SetVisible(c.UI, h.Panel, c.Hud.KhronosOpen)
	if !c.Hud.KhronosOpen {
		return
	}

	c.updateCaret(h.SearchField, h.SearchCaret)

	browser := *ecs.MustResource[*KhronosBrowser](c.Engine)
	browser.EnsureLoaded()

	needle := ""
	if ti, ok := ecs.Get[ui.TextInput](c.UI, h.SearchField); ok {
		needle = strings.ToLower(ti.Buffer)
	}

	entries := browser.Entries()
	if entries == nil {
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

	if !h.Built || needle != h.LastFilter || len(entries) != h.LastCount {
		c.rebuildKhronosRows(h, entries, filtered)
		h.LastFilter = needle
		h.LastCount = len(entries)
		h.Built = true
	}

	contentHeight := float32(len(h.Rows)) * khronosRowHeight
	if h.Scroll.Update(c.UI, c.Pointer.X, c.Pointer.Y, c.Input.Wheel, contentHeight) {
		c.Input.Wheel = 0
	}

	if loading := browser.LoadingStatus(); loading != "" {
		setLabel(c.UI, h.StatusLabel, "LOADING "+strings.ToUpper(loading)+"...", khronosStatusColor)
	} else {
		setLabel(c.UI, h.StatusLabel, fmt.Sprintf("%d / %d ASSETS", len(filtered), len(entries)), khronosStatusColor)
	}
}

func (c *HudContext) rebuildKhronosRows(h *khronosHandles, entries []KhronosEntry, filtered []int) {
	for _, row := range h.Rows {
		c.UI.Despawn(row)
	}
	h.Rows = h.Rows[:0]
	h.RowToEntry = h.RowToEntry[:0]
	h.Scroll.Offset = 0

	contentWidth := khronosPanelWidth - 20
	b := ui.NewBuilder(c.UI)
	b.Push(h.Scroll.Content)
	for _, entryIndex := range filtered {
		row := b.Node(ui.Node{Width: contentWidth, Height: khronosRowHeight, ZIndex: khronosContentZ}).
			Color(ui.Color{RGBA: khronosRowFill}).
			Interactive().
			Text(ui.Text{Content: entries[entryIndex].Label, Color: khronosRowColor, Scale: 1.3}).
			Entity()
		h.Rows = append(h.Rows, row)
		h.RowToEntry = append(h.RowToEntry, entryIndex)
	}
	b.Pop()
	ui.MarkLayoutDirty(c.UI)
}

func setLabel(world *ecs.World, entity ecs.Entity, content string, color [4]float32) {
	if text, ok := ecs.GetMut[ui.Text](world, entity); ok {
		text.Content = content
		text.Color = color
	}
}
