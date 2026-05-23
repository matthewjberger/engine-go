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

	srgbNext    uint32
	linearNext  uint32
	layerByName map[string]uint32
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
		LayerSize:   layerSize,
		MaxLayers:   maxLayers,
		MipLevels:   mips,
		layerByName: make(map[string]uint32, 64),
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

func (m *MaterialTextureArrays) Upload(queue *wgpu.Queue, name string, space TextureColorSpace, pixels []byte) (uint32, error) {
	key := name + colorSpaceTag(space)
	if layer, ok := m.layerByName[key]; ok {
		return layer, nil
	}
	expected := int(m.LayerSize * m.LayerSize * 4)
	if len(pixels) != expected {
		return 0, fmt.Errorf("material arrays: %q: expected %d bytes (%dx%d RGBA8), got %d", name, expected, m.LayerSize, m.LayerSize, len(pixels))
	}
	var (
		target *wgpu.Texture
		layer  uint32
	)
	if space == TextureSRGB {
		if m.srgbNext >= m.MaxLayers {
			return 0, fmt.Errorf("material arrays: sRGB array full (%d layers)", m.MaxLayers)
		}
		target = m.SRGB
		layer = m.srgbNext
		m.srgbNext++
	} else {
		if m.linearNext >= m.MaxLayers {
			return 0, fmt.Errorf("material arrays: linear array full (%d layers)", m.MaxLayers)
		}
		target = m.Linear
		layer = m.linearNext
		m.linearNext++
	}

	level := pixels
	w := m.LayerSize
	h := m.LayerSize
	for mip := uint32(0); mip < m.MipLevels; mip++ {
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
		if mip+1 >= m.MipLevels {
			break
		}
		level, w, h = downsampleRGBA(level, w, h)
	}

	m.layerByName[key] = layer
	return layer, nil
}

func colorSpaceTag(space TextureColorSpace) string {
	if space == TextureSRGB {
		return "|srgb"
	}
	return "|linear"
}
