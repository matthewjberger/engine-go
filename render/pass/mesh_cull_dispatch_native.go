//go:build !js

package pass

import (
	"github.com/cogentcore/webgpu/wgpu"

	"indigo/render"
)

// dispatchCullPasses runs a single cull compute dispatch per
// bucket. meshPrepare CPU-writes each bucket's DrawIndirect
// template (instance_count=0) into indirect_buffer right before
// this; the cull shader then atomicAdd's into instance_count and
// appends surviving slot ids into visible_indices. Native DrawIndirect
// reads the resulting instance_count, so the leading K visible_indices
// entries are exactly what gets drawn.
//
// The wasm build skips this entirely; see mesh_cull_dispatch_wasm.go.
func dispatchCullPasses(state *meshPassState, context *render.PassContext) {
	if len(state.sortedHandles) == 0 {
		return
	}

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
