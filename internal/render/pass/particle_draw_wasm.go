//go:build js

package pass

import "github.com/cogentcore/webgpu/wgpu"

func particleDraw(pass *wgpu.RenderPassEncoder, system *particleSystem) {
	_ = system
	pass.Draw(6, maxParticles, 0, 0)
}
