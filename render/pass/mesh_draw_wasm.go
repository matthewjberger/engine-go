//go:build js

package pass

import "github.com/cogentcore/webgpu/wgpu"

// drawHandle issues the per-handle draw call. The cogentcore wgpu
// binding doesn't expose DrawIndirect on its wasm target yet, so
// the wasm build falls back to a direct Draw with the same vertex
// + instance counts the CPU already wrote into bucket.indirectBuffer.
// When the binding gains DrawIndirect on wasm, this file collapses
// into the native one.
func drawHandle(pass *wgpu.RenderPassEncoder, bucket *handleInstances, vertexCount, instanceCount uint32) {
	_ = bucket
	pass.Draw(vertexCount, instanceCount, 0, 0)
}
