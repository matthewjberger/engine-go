//go:build js

package pass

import "indigo/render"

// dispatchCullPasses is a no-op on wasm because the cogentcore wgpu
// binding doesn't expose DrawIndirect. The draw path falls back to
// plain Draw with the full per-bucket instance count and reads
// visible_indices that the CPU pre-filled with the identity mapping
// in meshPrepare. Running the cull compute here would overwrite the
// leading K entries with culled indices but leave the [K..N) tail
// holding stale identity values, causing the wasm Draw to render
// duplicated instances (visible as merged bricks + striped sprites).
func dispatchCullPasses(_ *meshPassState, _ *render.PassContext) {}
