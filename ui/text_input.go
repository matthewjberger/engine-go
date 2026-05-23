package ui

import (
	"github.com/matthewjberger/indigo/ecs"
)

// AnyTextInputFocused reports whether the focused entity carries a
// [TextInput] component. Engine systems that read keyboard input
// should gate on this so typing in a text field doesn't fire global
// hotkeys.
func AnyTextInputFocused(world *ecs.World) bool {
	if world == nil {
		return false
	}
	if !ecs.HasResource[PointerState](world) {
		return false
	}
	pointer := ecs.MustResource[PointerState](world)
	if pointer.FocusedEntity.ID == 0 {
		return false
	}
	_, ok := ecs.Get[TextInput](world, pointer.FocusedEntity)
	return ok
}

// AdvanceTextInputs applies a frame's typed characters to the
// focused [TextInput]. Enter emits [TextCommitted].
func AdvanceTextInputs(world *ecs.World, chars []rune, backspace, enter bool) {
	pointer := ecs.MustResource[PointerState](world)
	target := pointer.FocusedEntity
	if target.ID == 0 {
		return
	}
	input, ok := ecs.GetMut[TextInput](world, target)
	if !ok {
		return
	}
	for _, c := range chars {
		if c < 0x20 || c == 0x7f {
			continue
		}
		if input.Caret < 0 || input.Caret > len(input.Buffer) {
			input.Caret = len(input.Buffer)
		}
		input.Buffer = input.Buffer[:input.Caret] + string(c) + input.Buffer[input.Caret:]
		input.Caret++
	}
	if backspace && input.Caret > 0 && len(input.Buffer) > 0 {
		input.Buffer = input.Buffer[:input.Caret-1] + input.Buffer[input.Caret:]
		input.Caret--
	}
	if text, ok := ecs.GetMut[Text](world, target); ok {
		text.Content = input.Buffer
	}
	if enter {
		ecs.Send(world, TextCommitted{Entity: target, Value: input.Buffer})
	}
}
