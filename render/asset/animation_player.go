package asset

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/transform"
	"github.com/matthewjberger/indigo/window"
)

type AnimationPlayer struct {
	Clips             []AnimationClip
	CurrentClip       int
	Time              float32
	Speed             float32
	Looping           bool
	Playing           bool
	NodeIndexToEntity map[int]ecs.Entity
}

func NewAnimationPlayer(clips []AnimationClip, nodeIndexToEntity map[int]ecs.Entity) AnimationPlayer {
	player := AnimationPlayer{
		Clips:             clips,
		CurrentClip:       -1,
		Speed:             1,
		Looping:           true,
		NodeIndexToEntity: nodeIndexToEntity,
	}
	if len(clips) > 0 {
		player.CurrentClip = 0
		player.Playing = true
	}
	return player
}

func UpdateAnimationPlayers(world *ecs.World) {
	w, ok := ecs.Resource[window.Window](world)
	if !ok {
		return
	}
	delta := w.Timing.DeltaSeconds

	var dirty []ecs.Entity
	seen := make(map[ecs.Entity]struct{}, 16)

	mask := ecs.MustMaskOf[AnimationPlayer](world)
	world.ForEach(mask, 0, func(entity ecs.Entity, table *ecs.Archetype, index int) {
		players, _ := ecs.Column[AnimationPlayer](world, table)
		p := &players[index]
		advancePlayer(p, delta)
		applyPlayer(world, p, func(target ecs.Entity) {
			if _, ok := seen[target]; ok {
				return
			}
			seen[target] = struct{}{}
			dirty = append(dirty, target)
		})
	})

	for _, target := range dirty {
		transform.MarkDirty(world, target)
	}
}

func advancePlayer(p *AnimationPlayer, delta float32) {
	if !p.Playing || p.CurrentClip < 0 || p.CurrentClip >= len(p.Clips) {
		return
	}
	clip := &p.Clips[p.CurrentClip]
	p.Time += delta * p.Speed
	if clip.Duration <= 0 {
		return
	}
	if p.Looping {
		p.Time = float32(math.Mod(float64(p.Time), float64(clip.Duration)))
		if p.Time < 0 {
			p.Time += clip.Duration
		}
	} else if p.Time > clip.Duration {
		p.Time = clip.Duration
		p.Playing = false
	}
}

func applyPlayer(world *ecs.World, p *AnimationPlayer, markDirty func(ecs.Entity)) {
	if p.CurrentClip < 0 || p.CurrentClip >= len(p.Clips) {
		return
	}
	clip := &p.Clips[p.CurrentClip]
	for i := range clip.Channels {
		channel := &clip.Channels[i]
		target, ok := p.NodeIndexToEntity[channel.TargetNode]
		if !ok {
			continue
		}
		if sampleChannelInto(world, target, channel, p.Time) {
			markDirty(target)
		}
	}
}

func sampleChannelInto(world *ecs.World, target ecs.Entity, channel *AnimationChannel, t float32) bool {
	keyA, keyB, factor := findKeyframe(channel.Sampler.Inputs, t)
	local, ok := ecs.GetMut[transform.LocalTransform](world, target)
	if !ok {
		return false
	}
	switch channel.Property {
	case AnimationTranslation:
		if len(channel.Sampler.Vec3Outputs) > 0 {
			local.Translation = sampleVec3(channel.Sampler.Vec3Outputs, keyA, keyB, factor, channel.Sampler.Interpolation)
			return true
		}
	case AnimationScale:
		if len(channel.Sampler.Vec3Outputs) > 0 {
			local.Scale = sampleVec3(channel.Sampler.Vec3Outputs, keyA, keyB, factor, channel.Sampler.Interpolation)
			return true
		}
	case AnimationRotation:
		if len(channel.Sampler.Vec4Outputs) > 0 {
			local.Rotation = sampleQuat(channel.Sampler.Vec4Outputs, keyA, keyB, factor, channel.Sampler.Interpolation)
			return true
		}
	}
	return false
}

func findKeyframe(times []float32, t float32) (int, int, float32) {
	if len(times) == 0 {
		return 0, 0, 0
	}
	if t <= times[0] || len(times) == 1 {
		return 0, 0, 0
	}
	last := len(times) - 1
	if t >= times[last] {
		return last, last, 0
	}
	low, high := 0, last
	for low < high {
		mid := (low + high + 1) / 2
		if times[mid] <= t {
			low = mid
		} else {
			high = mid - 1
		}
	}
	span := times[low+1] - times[low]
	if span <= 0 {
		return low, low, 0
	}
	return low, low + 1, (t - times[low]) / span
}

func sampleVec3(outputs [][3]float32, a, b int, factor float32, mode AnimationInterpolation) transform.Vec3 {
	va := outputs[a]
	vb := outputs[b]
	if mode == InterpolationStep || a == b {
		return transform.Vec3{va[0], va[1], va[2]}
	}
	return transform.Vec3{
		va[0] + (vb[0]-va[0])*factor,
		va[1] + (vb[1]-va[1])*factor,
		va[2] + (vb[2]-va[2])*factor,
	}
}

func sampleQuat(outputs [][4]float32, a, b int, factor float32, mode AnimationInterpolation) transform.Quat {
	va := outputs[a]
	vb := outputs[b]
	if mode == InterpolationStep || a == b {
		return transform.Quat{W: va[3], V: mgl32.Vec3{va[0], va[1], va[2]}}
	}
	qa := mgl32.Quat{W: va[3], V: mgl32.Vec3{va[0], va[1], va[2]}}
	qb := mgl32.Quat{W: vb[3], V: mgl32.Vec3{vb[0], vb[1], vb[2]}}
	return mgl32.QuatSlerp(qa, qb, factor)
}
