//go:build js

package asset

// streamBudget caps image decodes per frame on wasm. Goroutines give no
// parallelism in the browser (single thread, GPU calls must stay on it), so
// decoding happens inline here and this directly bounds per-frame decode cost.
const streamBudget = 2

type decodeRequest struct {
	item      pendingTexture
	layerSize uint32
}

// textureDecoder holds pending work and decodes inline during poll, since the
// browser runtime is single-threaded.
type textureDecoder struct {
	pending []decodeRequest
}

func newTextureDecoder() *textureDecoder {
	return &textureDecoder{}
}

func (d *textureDecoder) submit(item pendingTexture, layerSize uint32) {
	d.pending = append(d.pending, decodeRequest{item: item, layerSize: layerSize})
}

func (d *textureDecoder) poll() (decodedLayer, bool) {
	if len(d.pending) == 0 {
		return decodedLayer{}, false
	}
	request := d.pending[0]
	d.pending = d.pending[1:]
	return decodeLayerJob(request.item, request.layerSize), true
}

func (d *textureDecoder) reset() {
	d.pending = nil
}
