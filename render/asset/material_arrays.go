package asset

import (
	"fmt"

	"github.com/cogentcore/webgpu/wgpu"
)

type MaterialTextureArrays struct {
	LayerSize uint32
	MaxLayers uint32
	MipLevels uint32

	SRGB       *wgpu.Texture
	Linear     *wgpu.Texture
	SRGBView   *wgpu.TextureView
	LinearView *wgpu.TextureView
	Sampler    *wgpu.Sampler

	srgbNext      uint32
	linearNext    uint32
	srgbDefault   int64
	linearDefault int64
	layerByName   map[string]uint32

	loading         *LoadingQueue
	placeholderMips [][]byte
}

type MaterialTextureArraysResource struct {
	Arrays *MaterialTextureArrays
}

const NoTextureLayer uint32 = 0xFFFFFFFF

type WrapMode uint8

const (
	WrapRepeat         WrapMode = 0
	WrapMirroredRepeat WrapMode = 1
	WrapClampToEdge    WrapMode = 2
)

func PackLayer(layer uint32, wrapU, wrapV WrapMode) uint32 {
	return (layer & 0xFFFF) | (uint32(wrapU) << 16) | (uint32(wrapV) << 18)
}

func NewMaterialTextureArrays(device *wgpu.Device) (*MaterialTextureArrays, error) {
	return NewMaterialTextureArraysWith(device, 1024, 256)
}

func NewMaterialTextureArraysWith(device *wgpu.Device, layerSize, maxLayers uint32) (*MaterialTextureArrays, error) {
	mips := mipLevelCount(layerSize, layerSize)
	arrays := &MaterialTextureArrays{
		LayerSize:     layerSize,
		MaxLayers:     maxLayers,
		MipLevels:     mips,
		srgbDefault:   -1,
		linearDefault: -1,
		layerByName:   make(map[string]uint32, 64),
	}

	srgb, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "material srgb texture array",
		Size: wgpu.Extent3D{
			Width:              layerSize,
			Height:             layerSize,
			DepthOrArrayLayers: maxLayers,
		},
		MipLevelCount: mips,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatRGBA8UnormSrgb,
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("material arrays: srgb texture: %w", err)
	}
	arrays.SRGB = srgb

	linear, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "material linear texture array",
		Size: wgpu.Extent3D{
			Width:              layerSize,
			Height:             layerSize,
			DepthOrArrayLayers: maxLayers,
		},
		MipLevelCount: mips,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatRGBA8Unorm,
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
	})
	if err != nil {
		srgb.Release()
		return nil, fmt.Errorf("material arrays: linear texture: %w", err)
	}
	arrays.Linear = linear

	arrayDim := wgpu.TextureViewDimension2DArray
	srgbView, err := srgb.CreateView(&wgpu.TextureViewDescriptor{
		Label:           "material srgb array view",
		Format:          wgpu.TextureFormatRGBA8UnormSrgb,
		Dimension:       arrayDim,
		BaseMipLevel:    0,
		MipLevelCount:   mips,
		BaseArrayLayer:  0,
		ArrayLayerCount: maxLayers,
		Aspect:          wgpu.TextureAspectAll,
	})
	if err != nil {
		srgb.Release()
		linear.Release()
		return nil, fmt.Errorf("material arrays: srgb view: %w", err)
	}
	arrays.SRGBView = srgbView

	linearView, err := linear.CreateView(&wgpu.TextureViewDescriptor{
		Label:           "material linear array view",
		Format:          wgpu.TextureFormatRGBA8Unorm,
		Dimension:       arrayDim,
		BaseMipLevel:    0,
		MipLevelCount:   mips,
		BaseArrayLayer:  0,
		ArrayLayerCount: maxLayers,
		Aspect:          wgpu.TextureAspectAll,
	})
	if err != nil {
		srgbView.Release()
		srgb.Release()
		linear.Release()
		return nil, fmt.Errorf("material arrays: linear view: %w", err)
	}
	arrays.LinearView = linearView

	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:         "material array sampler",
		AddressModeU:  wgpu.AddressModeClampToEdge,
		AddressModeV:  wgpu.AddressModeClampToEdge,
		AddressModeW:  wgpu.AddressModeClampToEdge,
		MagFilter:     wgpu.FilterModeLinear,
		MinFilter:     wgpu.FilterModeLinear,
		MipmapFilter:  wgpu.MipmapFilterModeLinear,
		LodMinClamp:   0,
		LodMaxClamp:   float32(mips),
		MaxAnisotropy: 16,
	})
	if err != nil {
		linearView.Release()
		srgbView.Release()
		linear.Release()
		srgb.Release()
		return nil, fmt.Errorf("material arrays: sampler: %w", err)
	}
	arrays.Sampler = sampler

	return arrays, nil
}

func (m *MaterialTextureArrays) Release() {
	if m.Sampler != nil {
		m.Sampler.Release()
		m.Sampler = nil
	}
	if m.SRGBView != nil {
		m.SRGBView.Release()
		m.SRGBView = nil
	}
	if m.LinearView != nil {
		m.LinearView.Release()
		m.LinearView = nil
	}
	if m.SRGB != nil {
		m.SRGB.Release()
		m.SRGB = nil
	}
	if m.Linear != nil {
		m.Linear.Release()
		m.Linear = nil
	}
}

func (m *MaterialTextureArrays) ReserveLayer(queue *wgpu.Queue, name string, srgb bool) (uint32, bool) {
	fallback := m.ensureDefaultLayer(queue, srgb)
	key := layerKey(name, srgb)
	if layer, ok := m.layerByName[key]; ok {
		return layer, false
	}
	layer, ok := m.allocateLayer(queue, srgb)
	if !ok {
		return fallback, false
	}
	m.layerByName[key] = layer
	return layer, true
}

func (m *MaterialTextureArrays) UploadLayer(queue *wgpu.Queue, layerIndex uint32, srgb bool, rgba []byte, width, height uint32) error {
	if width != m.LayerSize || height != m.LayerSize {
		return fmt.Errorf("material arrays: layer %d: expected %dx%d, got %dx%d", layerIndex, m.LayerSize, m.LayerSize, width, height)
	}
	expected := int(width * height * 4)
	if len(rgba) != expected {
		return fmt.Errorf("material arrays: layer %d: expected %d bytes (%dx%d RGBA8), got %d", layerIndex, expected, width, height, len(rgba))
	}
	target := m.Linear
	if srgb {
		target = m.SRGB
	}
	writeLayerMips(queue, target, layerIndex, rgba, width, height, m.MipLevels)
	return nil
}

func (m *MaterialTextureArrays) Reset() {
	m.srgbNext = 0
	m.linearNext = 0
	m.srgbDefault = -1
	m.linearDefault = -1
	clear(m.layerByName)
	if m.loading != nil {
		m.loading.Reset()
	}
}

func (m *MaterialTextureArrays) ensureDefaultLayer(queue *wgpu.Queue, srgb bool) uint32 {
	if srgb {
		if m.srgbDefault < 0 {
			if layer, ok := m.allocateLayer(queue, true); ok {
				m.srgbDefault = int64(layer)
			} else {
				m.srgbDefault = 0
			}
		}
		return uint32(m.srgbDefault)
	}
	if m.linearDefault < 0 {
		if layer, ok := m.allocateLayer(queue, false); ok {
			m.linearDefault = int64(layer)
		} else {
			m.linearDefault = 0
		}
	}
	return uint32(m.linearDefault)
}

func (m *MaterialTextureArrays) allocateLayer(queue *wgpu.Queue, srgb bool) (uint32, bool) {
	var (
		target *wgpu.Texture
		layer  uint32
	)
	if srgb {
		if m.srgbNext >= m.MaxLayers {
			return 0, false
		}
		target = m.SRGB
		layer = m.srgbNext
		m.srgbNext++
	} else {
		if m.linearNext >= m.MaxLayers {
			return 0, false
		}
		target = m.Linear
		layer = m.linearNext
		m.linearNext++
	}
	m.writePlaceholder(queue, target, layer)
	return layer, true
}

// writePlaceholder fills every mip of a freshly reserved layer with a flat
// neutral colour so geometry renders cleanly before the decoded texture streams
// in. The flat mip buffers are built once and reused across layers.
func (m *MaterialTextureArrays) writePlaceholder(queue *wgpu.Queue, target *wgpu.Texture, layer uint32) {
	if m.placeholderMips == nil {
		m.placeholderMips = make([][]byte, m.MipLevels)
		w, h := m.LayerSize, m.LayerSize
		for mip := uint32(0); mip < m.MipLevels; mip++ {
			buf := make([]byte, w*h*4)
			for i := range buf {
				buf[i] = 255
			}
			m.placeholderMips[mip] = buf
			w, h = nextMipSize(w), nextMipSize(h)
		}
	}
	w, h := m.LayerSize, m.LayerSize
	for mip := uint32(0); mip < m.MipLevels; mip++ {
		queue.WriteTexture(
			&wgpu.ImageCopyTexture{
				Texture:  target,
				MipLevel: mip,
				Origin:   wgpu.Origin3D{Z: layer},
				Aspect:   wgpu.TextureAspectAll,
			},
			m.placeholderMips[mip],
			&wgpu.TextureDataLayout{
				Offset:       0,
				BytesPerRow:  w * 4,
				RowsPerImage: h,
			},
			&wgpu.Extent3D{
				Width:              w,
				Height:             h,
				DepthOrArrayLayers: 1,
			},
		)
		w, h = nextMipSize(w), nextMipSize(h)
	}
}

func nextMipSize(x uint32) uint32 {
	if x > 1 {
		return x / 2
	}
	return 1
}

func writeLayerMips(queue *wgpu.Queue, target *wgpu.Texture, layer uint32, pixels []byte, width, height, mipLevels uint32) {
	level := pixels
	w := width
	h := height
	for mip := uint32(0); mip < mipLevels; mip++ {
		queue.WriteTexture(
			&wgpu.ImageCopyTexture{
				Texture:  target,
				MipLevel: mip,
				Origin: wgpu.Origin3D{
					Z: layer,
				},
				Aspect: wgpu.TextureAspectAll,
			},
			level,
			&wgpu.TextureDataLayout{
				Offset:       0,
				BytesPerRow:  w * 4,
				RowsPerImage: h,
			},
			&wgpu.Extent3D{
				Width:              w,
				Height:             h,
				DepthOrArrayLayers: 1,
			},
		)
		if mip+1 >= mipLevels {
			break
		}
		level, w, h = downsampleRGBA(level, w, h)
	}
}

func layerKey(name string, srgb bool) string {
	if srgb {
		return name + "|srgb"
	}
	return name + "|linear"
}
