//go:build !js

package pass

import "github.com/cogentcore/webgpu/wgpu"

func particleDraw(pass *wgpu.RenderPassEncoder, system *particleSystem) {
	pass.DrawIndirect(system.drawIndirect, 0)
}
