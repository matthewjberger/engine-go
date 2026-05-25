//go:build js

package asset

import "github.com/cogentcore/webgpu/wgpu"

// writeAt updates a slice of the registry buffer through a staging buffer and a
// one-shot copy. The browser WebGPU binding rejects queue.WriteBuffer at a
// non-zero offset, so materials past index 0 must be uploaded this way. The copy
// is submitted before the frame's command buffer, so the registry is populated
// by the time the mesh pass samples it.
func (r *MaterialRegistry) writeAt(queue *wgpu.Queue, offset uint64, data []byte) {
	if len(data) == 0 {
		return
	}
	staging, err := r.device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "material registry staging",
		Contents: data,
		Usage:    wgpu.BufferUsageCopySrc,
	})
	if err != nil {
		return
	}
	defer staging.Release()
	encoder, err := r.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "material registry write"})
	if err != nil {
		return
	}
	defer encoder.Release()
	encoder.CopyBufferToBuffer(staging, 0, r.buffer, offset, uint64(len(data)))
	cmd, err := encoder.Finish(nil)
	if err != nil {
		return
	}
	defer cmd.Release()
	queue.Submit(cmd)
}
