//go:build !js

package pass

import (
	"github.com/cogentcore/webgpu/wgpu"

	"indigo/render"
)

// dispatchCullPasses runs the reset + cull compute dispatches that
// build each bucket's indirect command buffer and write the K
// surviving slot indices into visible_indices. The native draw path
// reads back instance_count from indirect_buffer via DrawIndirect,
// so reading only the leading K visible_indices entries is correct.
//
// The wasm build skips this entirely; see mesh_cull_dispatch_wasm.go.
func dispatchCullPasses(state *meshPassState, context *render.PassContext) {
	if len(state.sortedHandles) == 0 {
		return
	}

	resetPass := context.Encoder.BeginComputePass(&wgpu.ComputePassDescriptor{})
	resetPass.SetBindGroup(0, state.meshCulling.frameBindGroup, nil)
	resetPass.SetPipeline(state.meshCulling.resetPipeline)
	for _, handle := range state.sortedHandles {
		bucket := state.perHandle[handle]
		if bucket.cullBindGroup == nil {
			continue
		}
		resetPass.SetBindGroup(1, bucket.cullBindGroup, nil)
		resetPass.DispatchWorkgroups(1, 1, 1)
	}
	resetPass.End()
	resetPass.Release()

	cullPass := context.Encoder.BeginComputePass(&wgpu.ComputePassDescriptor{})
	cullPass.SetBindGroup(0, state.meshCulling.frameBindGroup, nil)
	cullPass.SetPipeline(state.meshCulling.cullPipeline)
	for _, handle := range state.sortedHandles {
		bucket := state.perHandle[handle]
		if bucket.cullBindGroup == nil {
			continue
		}
		count := uint32(len(bucket.slotEntity))
		groups := (count + 63) / 64
		if groups == 0 {
			continue
		}
		cullPass.SetBindGroup(1, bucket.cullBindGroup, nil)
		cullPass.DispatchWorkgroups(groups, 1, 1)
	}
	cullPass.End()
	cullPass.Release()
}
