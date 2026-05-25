//go:build js

package pass

import "github.com/cogentcore/webgpu/wgpu"

func writeBuffer(device *wgpu.Device, _ *wgpu.Queue, encoder *wgpu.CommandEncoder, buffer *wgpu.Buffer, offset uint64, data []byte) {
	if len(data) == 0 {
		return
	}
	staging, err := device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "write_buffer staging",
		Contents: data,
		Usage:    wgpu.BufferUsageCopySrc,
	})
	if err != nil {
		return
	}
	encoder.CopyBufferToBuffer(staging, 0, buffer, offset, uint64(len(data)))
	staging.Release()
}

// writeBufferStandalone writes a buffer outside of a frame's command encoder
// (e.g. one-time init writes). queue.WriteBuffer views the Go heap and crashes
// once it grows, so copy through a staging buffer with a one-shot submit.
func writeBufferStandalone(device *wgpu.Device, queue *wgpu.Queue, buffer *wgpu.Buffer, offset uint64, data []byte) {
	if len(data) == 0 {
		return
	}
	staging, err := device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "standalone write staging",
		Contents: data,
		Usage:    wgpu.BufferUsageCopySrc,
	})
	if err != nil {
		return
	}
	defer staging.Release()
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "standalone write"})
	if err != nil {
		return
	}
	defer encoder.Release()
	encoder.CopyBufferToBuffer(staging, 0, buffer, offset, uint64(len(data)))
	cmd, err := encoder.Finish(nil)
	if err != nil {
		return
	}
	defer cmd.Release()
	queue.Submit(cmd)
}
