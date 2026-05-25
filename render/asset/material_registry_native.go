//go:build !js

package asset

import "github.com/cogentcore/webgpu/wgpu"

func (r *MaterialRegistry) writeAt(queue *wgpu.Queue, offset uint64, data []byte) {
	queue.WriteBuffer(r.buffer, offset, data)
}
