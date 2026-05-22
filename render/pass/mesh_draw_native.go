//go:build !js

package pass

import "github.com/cogentcore/webgpu/wgpu"

// drawHandle issues the per-handle draw call. On native targets
// this is an indirect draw — the GPU reads vertex_count and
// instance_count from bucket.indirectBuffer. The fallback Draw
// args go unused here; they exist for the wasm build path where
// the wgpu binding doesn't expose DrawIndirect yet.
func drawHandle(pass *wgpu.RenderPassEncoder, bucket *handleInstances, _, _ uint32) {
	pass.DrawIndirect(bucket.indirectBuffer, 0)
}
