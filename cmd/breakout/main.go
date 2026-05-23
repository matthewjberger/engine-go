//go:build !js

package main

import (
	"errors"
	"log"
	"runtime"
	"time"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/cogentcore/webgpu/wgpuglfw"
	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/matthewjberger/indigo/app"
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/render"
	"github.com/matthewjberger/indigo/window"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	setupLogging()

	if err := glfw.Init(); err != nil {
		log.Fatal(err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfwWindow, err := glfw.CreateWindow(1280, 720, "breakout", nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer glfwWindow.Destroy()

	instance := wgpu.CreateInstance(nil)
	defer instance.Release()

	surface := instance.CreateSurface(wgpuglfw.GetSurfaceDescriptor(glfwWindow))

	width, height := glfwWindow.GetSize()
	renderer, err := render.NewRenderer(instance, surface, uint32(width), uint32(height))
	if err != nil {
		log.Fatal(err)
	}
	defer renderer.Release()

	worlds, demo := buildWorlds(renderer)
	installInputCallbacks(glfwWindow, worlds.Engine)

	glfwWindow.SetSizeCallback(func(_ *glfw.Window, w, h int) {
		if w <= 0 || h <= 0 {
			return
		}
		if err := renderer.Resize(uint32(w), uint32(h)); err != nil {
			log.Printf("resize error: %v", err)
		}
		viewport := window.ViewportSize{Width: uint32(w), Height: uint32(h)}
		ecs.MustResource[window.Window](worlds.Engine).Viewport = viewport
		if worlds.UI != nil {
			ecs.MustResource[window.Window](worlds.UI).Viewport = viewport
		}
	})

	last := time.Now()
	for !glfwWindow.ShouldClose() {
		glfw.PollEvents()

		now := time.Now()
		delta := float32(now.Sub(last).Seconds())
		last = now

		syncBreakoutUiPointer(worlds)

		app.TickFrame(worlds, demo, delta)
		handleBreakoutUiClicks(worlds)
		updateBreakoutHud(worlds, delta)

		glfwWindow.SetTitle(titleForState(ecs.MustResource[GameState](worlds.Game)))

		switch err := render.RenderFrame(renderer, worlds.Engine); {
		case err == nil:
		case errors.Is(err, render.ErrSurfaceLost):
			renderer.Reconfigure()
		default:
			log.Fatal(err)
		}

		app.PostFrame(worlds)
	}
}

func installInputCallbacks(glfwWindow *glfw.Window, engine *ecs.World) {
	glfwWindow.SetCursorPosCallback(func(_ *glfw.Window, x, y float64) {
		input := ecs.MustResource[render.Input](engine)
		input.MousePosition[0] = float32(x)
		input.MousePosition[1] = float32(y)
	})

	glfwWindow.SetMouseButtonCallback(func(_ *glfw.Window, button glfw.MouseButton, action glfw.Action, _ glfw.ModifierKey) {
		input := ecs.MustResource[render.Input](engine)
		pressed := action == glfw.Press
		switch button {
		case glfw.MouseButtonLeft:
			input.LeftDown = pressed
		case glfw.MouseButtonRight:
			input.RightDown = pressed
		case glfw.MouseButtonMiddle:
			input.MiddleDown = pressed
		}
	})

	glfwWindow.SetScrollCallback(func(_ *glfw.Window, _, yOffset float64) {
		input := ecs.MustResource[render.Input](engine)
		input.Wheel += float32(yOffset)
	})

	glfwWindow.SetKeyCallback(func(w *glfw.Window, key glfw.Key, _ int, action glfw.Action, _ glfw.ModifierKey) {
		if key == glfw.KeyEscape && action == glfw.Press {
			w.SetShouldClose(true)
			return
		}
		r, ok := glfwKeyRune(key)
		if !ok {
			return
		}
		input := ecs.MustResource[render.Input](engine)
		switch action {
		case glfw.Press:
			render.InputMarkKeyDown(input, r)
		case glfw.Release:
			render.InputMarkKeyUp(input, r)
		}
	})
}

func glfwKeyRune(key glfw.Key) (rune, bool) {
	switch {
	case key >= glfw.KeyA && key <= glfw.KeyZ:
		return rune(key), true
	case key == glfw.KeySpace:
		return ' ', true
	}
	return 0, false
}
