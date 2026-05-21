package main

import (
	"fmt"
	"sort"

	"indigo/app"
	"indigo/ecs"
	"indigo/render"
	"indigo/transform"
	"indigo/ui"
	"indigo/window"
)

func syncUiPointer(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	input := ecs.Resource[render.Input](worlds.Engine)
	pointer := ecs.Resource[ui.PointerState](worlds.UI)
	prevLeft := pointer.LeftDown
	prevRight := pointer.RightDown
	pointer.X = input.MousePosition[0]
	pointer.Y = input.MousePosition[1]
	pointer.LeftDown = input.LeftDown
	pointer.LeftJustDown = input.LeftDown && !prevLeft
	pointer.LeftJustUp = !input.LeftDown && prevLeft
	pointer.RightDown = input.RightDown
	pointer.RightJustDown = input.RightDown && !prevRight
	pointer.RightJustUp = !input.RightDown && prevRight
}

func driveTextInputs(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	input := ecs.Resource[render.Input](worlds.Engine)
	hud := ecs.Resource[HudHandles](worlds.Engine)
	pointer := ecs.Resource[ui.PointerState](worlds.UI)

	focusedNow := pointer.FocusedEntity == hud.InspectorName

	if focusedNow && pointer.LeftJustDown {
		placeCaretAtPointer(worlds.UI, hud.InspectorName, pointer.X)
	}

	backspace := false
	enter := false
	left := false
	right := false
	for _, k := range input.KeysJustDown {
		switch k {
		case '\b':
			backspace = true
		case '\n', '\r':
			enter = true
		case '\x01':
			left = true
		case '\x02':
			right = true
		}
	}
	ui.AdvanceTextInputs(worlds.UI, input.Chars, backspace, enter)

	if focusedNow && (left || right) {
		if ti, ok := ecs.GetMut[ui.TextInput](worlds.UI, hud.InspectorName); ok {
			runes := []rune(ti.Buffer)
			if left && ti.Caret > 0 {
				ti.Caret--
			}
			if right && ti.Caret < len(runes) {
				ti.Caret++
			}
		}
	}

	for _, evt := range ecs.DrainEvents[ui.TextCommitted](worlds.UI) {
		if evt.Entity != hud.InspectorName {
			continue
		}
		commitInspectorName(worlds, evt.Value)
	}

	if hud.NameFocusedPrev && !focusedNow {
		if ti, ok := ecs.Get[ui.TextInput](worlds.UI, hud.InspectorName); ok {
			commitInspectorName(worlds, ti.Buffer)
		}
	}
	hud.NameFocusedPrev = focusedNow
}

func commitInspectorName(worlds app.Worlds, value string) {
	target, ok := render.SelectedTarget(worlds.Engine)
	if !ok {
		return
	}
	if name, ok := ecs.GetMut[app.Name](worlds.Engine, target); ok {
		name.Value = value
	}
}

func placeCaretAtPointer(world *ecs.World, field ecs.Entity, pointerX float32) {
	node, ok := ecs.Get[ui.Node](world, field)
	if !ok {
		return
	}
	ti, ok := ecs.GetMut[ui.TextInput](world, field)
	if !ok {
		return
	}
	label, ok := ecs.Get[ui.Text](world, field)
	if !ok {
		return
	}
	scale := label.Scale
	if scale <= 0 {
		scale = 1
	}
	advance := float32(ui.FontGlyphWidth+1) * scale
	runes := []rune(ti.Buffer)
	labelWidth := float32(len(runes)) * advance
	originX := node.Resolved.X + (node.Resolved.Width-labelWidth)*0.5
	rel := pointerX - originX
	idx := int((rel + advance*0.5) / advance)
	if idx < 0 {
		idx = 0
	}
	if idx > len(runes) {
		idx = len(runes)
	}
	ti.Caret = idx
}

func handleRightClick(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	pointer := ecs.Resource[ui.PointerState](worlds.UI)
	if !pointer.RightJustDown {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	for i, row := range hud.TreeRows {
		if row.ID == 0 {
			continue
		}
		nodeRef, ok := ecs.Get[ui.Node](worlds.UI, row)
		if !ok {
			continue
		}
		if !nodeRef.Resolved.Contains(pointer.X, pointer.Y) {
			continue
		}
		target := hud.TreeRowToEngine[i]
		if target.ID == 0 {
			return
		}
		hud.ContextTarget = target
		hud.OpenMenu = menuContextOpen
		moveContextMenu(worlds.UI, hud.ContextMenu, pointer.X, pointer.Y)
		return
	}
}

func moveContextMenu(world *ecs.World, menu menuPopup, x, y float32) {
	if node, ok := ecs.GetMut[ui.Node](world, menu.Panel); ok {
		node.X = x
		node.Y = y
	}
	ui.MarkLayoutDirty(world)
}

func handleUiClicks(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	gizmo := *ecs.Resource[*render.Gizmos](worlds.Engine)

	clickHandled := false
	for _, evt := range ecs.DrainEvents[ui.EntityClicked](worlds.UI) {
		clickHandled = true
		switch evt.Entity {
		case hud.TranslateButton:
			gizmo.Mode = render.GizmoTranslate
			hud.OpenMenu = menuClosed
		case hud.RotateButton:
			gizmo.Mode = render.GizmoRotate
			hud.OpenMenu = menuClosed
		case hud.ScaleButton:
			gizmo.Mode = render.GizmoScale
			hud.OpenMenu = menuClosed
		case hud.FileButton:
			hud.OpenMenu = toggleMenu(hud.OpenMenu, menuFileOpen)
		case hud.EditButton:
			hud.OpenMenu = toggleMenu(hud.OpenMenu, menuEditOpen)
		case hud.ViewButton:
			hud.OpenMenu = toggleMenu(hud.OpenMenu, menuViewOpen)
		default:
			if handleMenuItem(worlds, hud, evt.Entity) {
				hud.OpenMenu = menuClosed
				break
			}
			for i, row := range hud.TreeRows {
				if evt.Entity == row {
					target := hud.TreeRowToEngine[i]
					if target.ID != 0 {
						applyEntitySelection(worlds.Engine, target)
					}
					hud.OpenMenu = menuClosed
					break
				}
			}
		}
	}

	if hud.OpenMenu == menuClosed {
		return
	}

	pointer := ecs.Resource[ui.PointerState](worlds.UI)
	if !pointer.LeftJustDown || clickHandled {
		return
	}
	if pointerOverMenuGeometry(worlds.UI, hud) {
		return
	}
	hud.OpenMenu = menuClosed
}

func toggleMenu(current, target int) int {
	if current == target {
		return menuClosed
	}
	return target
}

func handleMenuItem(worlds app.Worlds, hud *HudHandles, entity ecs.Entity) bool {
	if idx := matchItem(hud.FileMenu, entity); idx >= 0 {
		if idx == 3 {
			hud.RequestExit = true
		}
		return true
	}
	if idx := matchItem(hud.EditMenu, entity); idx >= 0 {
		if idx == 2 {
			applySelection(worlds.Engine, 0)
		}
		return true
	}
	if idx := matchItem(hud.ViewMenu, entity); idx >= 0 {
		switch idx {
		case 0:
			controller := ecs.Resource[render.PanOrbitController](worlds.Engine)
			defaults := render.DefaultPanOrbitController()
			controller.TargetYaw = defaults.TargetYaw
			controller.TargetPitch = defaults.TargetPitch
			controller.TargetRadius = defaults.TargetRadius
			controller.TargetFocus = defaults.TargetFocus
		case 1:
			settings := ecs.Resource[render.GraphicsSettings](worlds.Engine)
			settings.ShowGrid = !settings.ShowGrid
		case 2:
			settings := ecs.Resource[render.GraphicsSettings](worlds.Engine)
			settings.ShowSky = !settings.ShowSky
		}
		return true
	}
	if idx := matchItem(hud.ContextMenu, entity); idx >= 0 {
		switch idx {
		case 0:
			target := hud.ContextTarget
			if target.ID != 0 {
				worlds.Engine.Despawn(target)
			}
			hud.ContextTarget = ecs.Entity{}
		}
		return true
	}
	return false
}

func matchItem(menu menuPopup, entity ecs.Entity) int {
	for i := 0; i < menu.Count; i++ {
		if menu.Items[i] == entity {
			return i
		}
	}
	return -1
}

func pointerOverMenuGeometry(uiWorld *ecs.World, hud *HudHandles) bool {
	pointer := ecs.Resource[ui.PointerState](uiWorld)
	var menu menuPopup
	var button ecs.Entity
	switch hud.OpenMenu {
	case menuFileOpen:
		menu, button = hud.FileMenu, hud.FileButton
	case menuEditOpen:
		menu, button = hud.EditMenu, hud.EditButton
	case menuViewOpen:
		menu, button = hud.ViewMenu, hud.ViewButton
	case menuContextOpen:
		menu = hud.ContextMenu
	default:
		return false
	}
	if button.ID != 0 && nodeContains(uiWorld, button, pointer.X, pointer.Y) {
		return true
	}
	if nodeContains(uiWorld, menu.Panel, pointer.X, pointer.Y) {
		return true
	}
	for i := 0; i < menu.Count; i++ {
		if nodeContains(uiWorld, menu.Items[i], pointer.X, pointer.Y) {
			return true
		}
	}
	return false
}

func nodeContains(world *ecs.World, entity ecs.Entity, x, y float32) bool {
	node, ok := ecs.Get[ui.Node](world, entity)
	if !ok {
		return false
	}
	return node.Resolved.Contains(x, y)
}

func refreshMenuPopups(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	setMenuVisible(worlds.UI, hud.FileMenu, hud.OpenMenu == menuFileOpen)
	setMenuVisible(worlds.UI, hud.EditMenu, hud.OpenMenu == menuEditOpen)
	setMenuVisible(worlds.UI, hud.ViewMenu, hud.OpenMenu == menuViewOpen)
	setMenuVisible(worlds.UI, hud.ContextMenu, hud.OpenMenu == menuContextOpen)

	occludeTreeTextBehindPopup(worlds.UI, hud)
}

func occludeTreeTextBehindPopup(world *ecs.World, hud *HudHandles) {
	var popup *menuPopup
	switch hud.OpenMenu {
	case menuFileOpen:
		popup = &hud.FileMenu
	case menuEditOpen:
		popup = &hud.EditMenu
	case menuViewOpen:
		popup = &hud.ViewMenu
	}
	var popupRect ui.Rect
	hasPopup := false
	if popup != nil {
		if node, ok := ecs.Get[ui.Node](world, popup.Panel); ok {
			popupRect = node.Resolved
			hasPopup = popupRect.Width > 0 && popupRect.Height > 0
		}
	}
	apply := func(entity ecs.Entity) {
		if entity.ID == 0 {
			return
		}
		nodeRef, ok := ecs.Get[ui.Node](world, entity)
		if !ok {
			return
		}
		text, ok := ecs.GetMut[ui.Text](world, entity)
		if !ok {
			return
		}
		hidden := hasPopup &&
			nodeRef.Resolved.Y < popupRect.Y+popupRect.Height &&
			nodeRef.Resolved.Y+nodeRef.Resolved.Height > popupRect.Y
		if hidden {
			text.Color[3] = 0
		} else {
			text.Color[3] = 1
		}
	}
	apply(hud.TreeTitle)
	for _, row := range hud.TreeRows {
		apply(row)
	}
}

func refreshInteractiveHovers(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	idle := [4]float32{0.14, 0.16, 0.20, 1}
	hover := [4]float32{0.32, 0.62, 0.98, 1}
	for _, m := range []menuPopup{hud.FileMenu, hud.EditMenu, hud.ViewMenu, hud.ContextMenu} {
		for i := 0; i < m.Count; i++ {
			paintHover(worlds.UI, m.Items[i], idle, hover)
		}
	}
	for _, b := range []ecs.Entity{hud.FileButton, hud.EditButton, hud.ViewButton} {
		paintHover(worlds.UI, b, idle, hover)
	}

	selected, hasSelected := render.SelectedTarget(worlds.Engine)
	for i, row := range hud.TreeRows {
		if row.ID == 0 {
			continue
		}
		target := hud.TreeRowToEngine[i]
		if hasSelected && target == selected {
			continue
		}
		interactive, ok := ecs.Get[ui.Interactive](worlds.UI, row)
		if !ok {
			continue
		}
		color, ok := ecs.GetMut[ui.Color](worlds.UI, row)
		if !ok {
			continue
		}
		if interactive.Hovered && color.RGBA[3] > 0.001 {
			color.RGBA = [4]float32{0.22, 0.32, 0.55, 1}
		}
	}
}

func paintHover(world *ecs.World, entity ecs.Entity, idle, hover [4]float32) {
	color, ok := ecs.GetMut[ui.Color](world, entity)
	if !ok || color.RGBA[3] <= 0.001 {
		return
	}
	interactive, ok := ecs.Get[ui.Interactive](world, entity)
	if !ok {
		return
	}
	if interactive.Hovered {
		color.RGBA = hover
	} else {
		color.RGBA = idle
	}
}

func setMenuVisible(world *ecs.World, menu menuPopup, visible bool) {
	var panelAlpha float32
	var textAlpha float32
	if visible {
		panelAlpha = 1
		textAlpha = 1
	}
	if color, ok := ecs.GetMut[ui.Color](world, menu.Panel); ok {
		color.RGBA = [4]float32{0.10, 0.11, 0.14, panelAlpha}
	}
	for i := 0; i < menu.Count; i++ {
		if color, ok := ecs.GetMut[ui.Color](world, menu.Items[i]); ok {
			if visible {
				color.RGBA = [4]float32{0.14, 0.16, 0.20, 1}
			} else {
				color.RGBA = [4]float32{0, 0, 0, 0}
			}
		}
		if text, ok := ecs.GetMut[ui.Text](world, menu.Items[i]); ok {
			text.Color[3] = textAlpha
		}
	}
}

func refreshHudLayout(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	w := ecs.Resource[window.Window](worlds.UI).Viewport
	width := float32(w.Width)
	height := float32(w.Height)
	changed := false
	if node, ok := ecs.GetMut[ui.Node](worlds.UI, hud.TopBar); ok && node.Width != width {
		node.Width = width
		changed = true
	}
	panelHeight := height - hudTopBarHeight
	if node, ok := ecs.GetMut[ui.Node](worlds.UI, hud.LeftPanel); ok && node.Height != panelHeight {
		node.Height = panelHeight
		changed = true
	}
	if node, ok := ecs.GetMut[ui.Node](worlds.UI, hud.RightPanel); ok && node.Height != panelHeight {
		node.Height = panelHeight
		changed = true
	}
	if changed {
		ui.MarkLayoutDirty(worlds.UI)
	}
}

func refreshModeButtons(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	gizmo := *ecs.Resource[*render.Gizmos](worlds.Engine)
	setModeColor(worlds.UI, hud.TranslateButton, gizmo.Mode == render.GizmoTranslate)
	setModeColor(worlds.UI, hud.RotateButton, gizmo.Mode == render.GizmoRotate)
	setModeColor(worlds.UI, hud.ScaleButton, gizmo.Mode == render.GizmoScale)
}

func setModeColor(world *ecs.World, entity ecs.Entity, active bool) {
	color, ok := ecs.GetMut[ui.Color](world, entity)
	if !ok {
		return
	}
	if active {
		color.RGBA = [4]float32{0.22, 0.5, 0.86, 1}
	} else {
		color.RGBA = [4]float32{0.18, 0.21, 0.28, 1}
	}
}

type namedEntity struct {
	Entity ecs.Entity
	Name   string
}

func refreshEntityTree(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)

	selected, hasSelected := render.SelectedTarget(worlds.Engine)

	nameMask := ecs.MaskOf[app.Name](worlds.Engine)
	var entries []namedEntity
	worlds.Engine.ForEach(nameMask, 0, func(entity ecs.Entity, table *ecs.Archetype, columnIndex int) {
		names, _ := ecs.Column[app.Name](worlds.Engine, table)
		entries = append(entries, namedEntity{Entity: entity, Name: names[columnIndex].Value})
	})
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Entity.ID < entries[j].Entity.ID
	})

	const rowStride float32 = hudTreeRowHeight + 4
	maxScroll := len(entries) - hudTreeRowCount
	if maxScroll < 0 {
		maxScroll = 0
	}
	maxScrollPixels := float32(maxScroll) * rowStride
	if hud.TreeScrollPixels > maxScrollPixels {
		hud.TreeScrollPixels = maxScrollPixels
	}
	if hud.TreeScrollPixels < 0 {
		hud.TreeScrollPixels = 0
	}
	hud.TreeScrollIndex = int(hud.TreeScrollPixels / rowStride)
	if hud.TreeScrollIndex > maxScroll {
		hud.TreeScrollIndex = maxScroll
	}
	offset := hud.TreeScrollIndex

	for i := 0; i < hudTreeRowCount; i++ {
		row := hud.TreeRows[i]
		idx := i + offset
		if idx < len(entries) {
			hud.TreeRowToEngine[i] = entries[idx].Entity
			if label, ok := ecs.GetMut[ui.Text](worlds.UI, row); ok {
				label.Content = entries[idx].Name
			}
			if color, ok := ecs.GetMut[ui.Color](worlds.UI, row); ok {
				if hasSelected && entries[idx].Entity == selected {
					color.RGBA = [4]float32{0.22, 0.5, 0.86, 1}
				} else {
					color.RGBA = [4]float32{0.10, 0.11, 0.14, 1}
				}
			}
		} else {
			hud.TreeRowToEngine[i] = ecs.Entity{}
			if label, ok := ecs.GetMut[ui.Text](worlds.UI, row); ok {
				label.Content = ""
			}
			if color, ok := ecs.GetMut[ui.Color](worlds.UI, row); ok {
				color.RGBA = [4]float32{0, 0, 0, 0}
			}
		}
	}
}

func updateTreeScroll(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	input := ecs.Resource[render.Input](worlds.Engine)
	if input.Wheel != 0 {
		leftNode, ok := ecs.Get[ui.Node](worlds.UI, hud.LeftPanel)
		if ok && leftNode.Resolved.Contains(input.MousePosition[0], input.MousePosition[1]) {
			hud.TreeScrollPixels -= input.Wheel * 40
			input.Wheel = 0
		}
	}
	const rowStride float32 = hudTreeRowHeight + 4
	if hud.TreeScrollPixels < 0 {
		hud.TreeScrollPixels = 0
	}
	hud.TreeScrollIndex = int(hud.TreeScrollPixels / rowStride)
}

func refreshInspector(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	target, hasTarget := render.SelectedTarget(worlds.Engine)
	pointer := ecs.Resource[ui.PointerState](worlds.UI)
	editing := pointer.FocusedEntity == hud.InspectorName

	if !hasTarget {
		syncInspectorName(worlds.UI, hud.InspectorName, "No entity selected", false)
		setInspectorText(worlds.UI, hud.InspectorID, "")
		setInspectorText(worlds.UI, hud.TranslationLabel, "")
		setInspectorText(worlds.UI, hud.RotationLabel, "")
		setInspectorText(worlds.UI, hud.ScaleLabel, "")
		setInspectorText(worlds.UI, hud.MaterialLabel, "")
		return
	}

	name := "(unnamed)"
	if n, ok := ecs.Get[app.Name](worlds.Engine, target); ok {
		name = n.Value
	}
	syncInspectorName(worlds.UI, hud.InspectorName, name, editing)
	setInspectorText(worlds.UI, hud.InspectorID, fmt.Sprintf("ID   %d", target.ID))

	if local, ok := ecs.Get[transform.LocalTransform](worlds.Engine, target); ok {
		setInspectorText(worlds.UI, hud.TranslationLabel, fmt.Sprintf("POS  %s", formatVec3(local.Translation)))
		setInspectorText(worlds.UI, hud.RotationLabel, fmt.Sprintf("ROT  %s", formatQuat(local.Rotation)))
		setInspectorText(worlds.UI, hud.ScaleLabel, fmt.Sprintf("SCL  %s", formatVec3(local.Scale)))
	} else {
		setInspectorText(worlds.UI, hud.TranslationLabel, "")
		setInspectorText(worlds.UI, hud.RotationLabel, "")
		setInspectorText(worlds.UI, hud.ScaleLabel, "")
	}

	if mat, ok := ecs.Get[render.Material](worlds.Engine, target); ok {
		setInspectorText(worlds.UI, hud.MaterialLabel, fmt.Sprintf("MAT  %s", formatRGBA(mat.BaseColor)))
	} else {
		setInspectorText(worlds.UI, hud.MaterialLabel, "")
	}
}

func setInspectorText(world *ecs.World, entity ecs.Entity, content string) {
	if label, ok := ecs.GetMut[ui.Text](world, entity); ok {
		label.Content = content
	}
}

func syncInspectorName(world *ecs.World, entity ecs.Entity, name string, editing bool) {
	if !editing {
		if ti, ok := ecs.GetMut[ui.TextInput](world, entity); ok {
			ti.Buffer = name
			if ti.Caret > len(name) {
				ti.Caret = len(name)
			}
		}
		if label, ok := ecs.GetMut[ui.Text](world, entity); ok {
			label.Content = name
		}
	}
	if color, ok := ecs.GetMut[ui.Color](world, entity); ok {
		if editing {
			color.RGBA = [4]float32{0.18, 0.30, 0.55, 1}
		} else {
			color.RGBA = [4]float32{0.12, 0.13, 0.16, 1}
		}
	}
}

func updateInspectorCaret(worlds app.Worlds) {
	if worlds.UI == nil {
		return
	}
	hud := ecs.Resource[HudHandles](worlds.Engine)
	pointer := ecs.Resource[ui.PointerState](worlds.UI)
	editing := pointer.FocusedEntity == hud.InspectorName

	caretColor, ok := ecs.GetMut[ui.Color](worlds.UI, hud.InspectorCaret)
	if !ok {
		return
	}
	if !editing {
		caretColor.RGBA[3] = 0
		return
	}
	field, ok := ecs.Get[ui.Node](worlds.UI, hud.InspectorName)
	if !ok {
		return
	}
	ti, ok := ecs.Get[ui.TextInput](worlds.UI, hud.InspectorName)
	if !ok {
		return
	}
	label, ok := ecs.Get[ui.Text](worlds.UI, hud.InspectorName)
	if !ok {
		return
	}
	scale := label.Scale
	if scale <= 0 {
		scale = 1
	}
	advance := float32(ui.FontGlyphWidth+1) * scale
	glyphH := float32(ui.FontGlyphHeight) * scale
	bufferRunes := []rune(ti.Buffer)
	caret := ti.Caret
	if caret > len(bufferRunes) {
		caret = len(bufferRunes)
	}
	labelWidth := float32(len(bufferRunes)) * advance
	originX := field.Resolved.X + (field.Resolved.Width-labelWidth)*0.5
	caretX := originX + float32(caret)*advance
	caretY := field.Resolved.Y + (field.Resolved.Height-glyphH)*0.5

	caretNode, ok := ecs.GetMut[ui.Node](worlds.UI, hud.InspectorCaret)
	if !ok {
		return
	}
	caretNode.X = caretX
	caretNode.Y = caretY
	caretNode.Height = glyphH
	caretNode.Resolved = ui.Rect{X: caretX, Y: caretY, Width: caretNode.Width, Height: glyphH}
	caretColor.RGBA = [4]float32{0.95, 0.96, 0.98, 1}
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
