//go:build !js

package asset

import "runtime"

// streamBudget caps GPU uploads per frame on native. Decoding runs in parallel
// worker goroutines, so this only throttles the cheap upload step.
const streamBudget = 8

type decodeRequest struct {
	item      pendingTexture
	layerSize uint32
}

// textureDecoder decodes images on a pool of worker goroutines for real
// parallelism across cores. The wgpu Device/Queue are never touched here;
// workers only produce decoded RGBA which the main thread uploads.
type textureDecoder struct {
	jobs    chan decodeRequest
	results chan decodedLayer
}

func newTextureDecoder() *textureDecoder {
	workers := max(min(runtime.NumCPU(), 8), 1)
	decoder := &textureDecoder{
		jobs:    make(chan decodeRequest, 1024),
		results: make(chan decodedLayer, 1024),
	}
	for range workers {
		go decoder.run()
	}
	return decoder
}

func (d *textureDecoder) run() {
	for request := range d.jobs {
		d.results <- decodeLayerJob(request.item, request.layerSize)
	}
}

func (d *textureDecoder) submit(item pendingTexture, layerSize uint32) {
	d.jobs <- decodeRequest{item: item, layerSize: layerSize}
}

func (d *textureDecoder) poll() (decodedLayer, bool) {
	select {
	case result := <-d.results:
		return result, true
	default:
		return decodedLayer{}, false
	}
}

func (d *textureDecoder) reset() {
	for {
		select {
		case <-d.results:
		default:
			return
		}
	}
}
