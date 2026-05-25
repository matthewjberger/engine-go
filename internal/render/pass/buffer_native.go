//go:build !js

package pass

import "github.com/cogentcore/webgpu/wgpu"

func writeBuffer(_ *wgpu.Device, queue *wgpu.Queue, _ *wgpu.CommandEncoder, buffer *wgpu.Buffer, offset uint64, data []byte) {
	queue.WriteBuffer(buffer, offset, data)
}

// writeBufferStandalone writes a buffer outside of a frame's command encoder
// (e.g. one-time init writes). On native this is a direct queue write.
func writeBufferStandalone(_ *wgpu.Device, queue *wgpu.Queue, buffer *wgpu.Buffer, offset uint64, data []byte) {
	queue.WriteBuffer(buffer, offset, data)
}
