package window

type ViewportSize struct {
	Width  uint32
	Height uint32
}

type Timing struct {
	DeltaSeconds  float32
	UptimeSeconds float32
	FramesPerSec  float32
	FrameCounter  uint64
}

type Window struct {
	Viewport ViewportSize
	Timing   Timing
}

func Advance(timing *Timing, delta float32) {
	timing.DeltaSeconds = delta
	timing.UptimeSeconds += delta
	timing.FrameCounter++
	if delta > 0 {
		timing.FramesPerSec = 1.0 / delta
	}
}
