package render

import "github.com/matthewjberger/indigo/transform"

type Input struct {
	MousePosition transform.Vec2
	MouseDelta    transform.Vec2
	Wheel         float32

	LeftDown   bool
	RightDown  bool
	MiddleDown bool

	KeysDown     map[rune]struct{}
	KeysJustDown []rune

	Chars []rune
}

func NewInput() Input {
	return Input{KeysDown: make(map[rune]struct{}, 16)}
}

func InputMarkKeyDown(i *Input, key rune) {
	if i.KeysDown == nil {
		i.KeysDown = make(map[rune]struct{}, 16)
	}
	i.KeysDown[key] = struct{}{}
	i.KeysJustDown = append(i.KeysJustDown, key)
}

func InputMarkKeyUp(i *Input, key rune) {
	delete(i.KeysDown, key)
}

func InputIsKeyDown(i *Input, key rune) bool {
	_, ok := i.KeysDown[key]
	return ok
}

func InputBeginFrame(i *Input) {
	i.MouseDelta = transform.Vec2{0, 0}
	i.Wheel = 0
	i.KeysJustDown = i.KeysJustDown[:0]
	i.Chars = i.Chars[:0]
}
