package render

import (
	"fmt"

	"github.com/cogentcore/webgpu/wgpu"

	"github.com/matthewjberger/indigo/ecs"
)

const DepthFormat = wgpu.TextureFormatDepth32Float

const HdrFormat = wgpu.TextureFormatRGBA16Float

type Renderer struct {
	Surface       *wgpu.Surface
	Adapter       *wgpu.Adapter
	Device        *wgpu.Device
	Queue         *wgpu.Queue
	Config        *wgpu.SurfaceConfiguration
	SurfaceFormat wgpu.TextureFormat

	Graph           *Graph
	SwapchainID     ResourceID
	SceneColorID    ResourceID
	LdrColorID      ResourceID
	DepthID         ResourceID
	EntityIdID      ResourceID
	SelectionMaskID ResourceID
	ViewNormalsID   ResourceID
	OitAccumID      ResourceID
	OitRevealID     ResourceID
}

func NewRenderer(instance *wgpu.Instance, surface *wgpu.Surface, width, height uint32) (*Renderer, error) {
	renderer := &Renderer{Surface: surface}

	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		CompatibleSurface: surface,
	})
	if err != nil {
		return nil, fmt.Errorf("render: request adapter: %w", err)
	}
	renderer.Adapter = adapter

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		adapter.Release()
		return nil, fmt.Errorf("render: request device: %w", err)
	}
	renderer.Device = device
	renderer.Queue = device.GetQueue()

	caps := surface.GetCapabilities(adapter)
	renderer.SurfaceFormat = caps.Formats[0]
	for _, format := range caps.Formats {
		if !isSrgb(format) {
			renderer.SurfaceFormat = format
			break
		}
	}

	renderer.Config = &wgpu.SurfaceConfiguration{
		Usage:       wgpu.TextureUsageRenderAttachment,
		Format:      renderer.SurfaceFormat,
		Width:       width,
		Height:      height,
		PresentMode: caps.PresentModes[0],
		AlphaMode:   caps.AlphaModes[0],
	}
	surface.Configure(adapter, device, renderer.Config)

	renderer.Graph = defaultGraph(renderer.SurfaceFormat, width, height)
	renderer.SwapchainID = renderer.Graph.ResourceByName("swapchain")
	renderer.SceneColorID = renderer.Graph.ResourceByName("scene_color")
	renderer.LdrColorID = renderer.Graph.ResourceByName("ldr_color")
	renderer.DepthID = renderer.Graph.ResourceByName("depth")
	renderer.EntityIdID = renderer.Graph.ResourceByName("entity_id")
	renderer.SelectionMaskID = renderer.Graph.ResourceByName("selection_mask")
	renderer.ViewNormalsID = renderer.Graph.ResourceByName("view_normals")
	renderer.OitAccumID = renderer.Graph.ResourceByName("oit_accum")
	renderer.OitRevealID = renderer.Graph.ResourceByName("oit_reveal")

	return renderer, nil
}

func defaultGraph(surfaceFormat wgpu.TextureFormat, width, height uint32) *Graph {
	graph := NewGraph()
	clearColor := wgpu.Color{R: 0.19, G: 0.24, B: 0.42, A: 1.0}
	clearDepth := float32(1.0)
	graph.AddColorTexture(ResourceDescriptor{
		Name: "scene_color",
		Kind: ResourceKindTransientColor,
		Texture: TextureDescriptor{
			Format: HdrFormat,
			Width:  width,
			Height: height,
			Usage:  wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopySrc,
		},
		ClearColor: &clearColor,
	})
	clearLdr := wgpu.Color{R: 0.19, G: 0.24, B: 0.42, A: 1.0}
	graph.AddColorTexture(ResourceDescriptor{
		Name: "ldr_color",
		Kind: ResourceKindTransientColor,
		Texture: TextureDescriptor{
			Format: surfaceFormat,
			Width:  width,
			Height: height,
			Usage:  wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopySrc,
		},
		ClearColor: &clearLdr,
	})
	graph.AddDepthTexture(ResourceDescriptor{
		Name: "depth",
		Kind: ResourceKindTransientDepth,
		Texture: TextureDescriptor{
			Format: DepthFormat,
			Width:  width,
			Height: height,
			Usage:  wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
		},
		ClearDepth: &clearDepth,
	})
	clearEntityID := wgpu.Color{R: 0, G: 0, B: 0, A: 0}
	graph.AddColorTexture(ResourceDescriptor{
		Name: "entity_id",
		Kind: ResourceKindTransientColor,
		Texture: TextureDescriptor{
			Format: EntityIdFormat,
			Width:  width,
			Height: height,
			Usage:  wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopySrc,
		},
		ClearColor: &clearEntityID,
	})
	clearMask := wgpu.Color{R: 0, G: 0, B: 0, A: 0}
	graph.AddColorTexture(ResourceDescriptor{
		Name: "selection_mask",
		Kind: ResourceKindTransientColor,
		Texture: TextureDescriptor{
			Format: SelectionMaskFormat,
			Width:  width,
			Height: height,
			Usage:  wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
		},
		ClearColor: &clearMask,
	})
	clearViewNormals := wgpu.Color{R: 0, G: 0, B: 0, A: 0}
	graph.AddColorTexture(ResourceDescriptor{
		Name: "view_normals",
		Kind: ResourceKindTransientColor,
		Texture: TextureDescriptor{
			Format: HdrFormat,
			Width:  width,
			Height: height,
			Usage:  wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
		},
		ClearColor: &clearViewNormals,
	})
	clearOitAccum := wgpu.Color{R: 0, G: 0, B: 0, A: 0}
	graph.AddColorTexture(ResourceDescriptor{
		Name: "oit_accum",
		Kind: ResourceKindTransientColor,
		Texture: TextureDescriptor{
			Format: HdrFormat,
			Width:  width,
			Height: height,
			Usage:  wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
		},
		ClearColor: &clearOitAccum,
	})

	clearOitReveal := wgpu.Color{R: 1, G: 1, B: 1, A: 1}
	graph.AddColorTexture(ResourceDescriptor{
		Name: "oit_reveal",
		Kind: ResourceKindTransientColor,
		Texture: TextureDescriptor{
			Format: wgpu.TextureFormatR8Unorm,
			Width:  width,
			Height: height,
			Usage:  wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
		},
		ClearColor: &clearOitReveal,
	})
	graph.AddColorTexture(ResourceDescriptor{
		Name: "swapchain",
		Kind: ResourceKindExternalColor,
	})
	return graph
}

func (r *Renderer) AspectRatio() float32 {
	height := r.Config.Height
	if height < 1 {
		height = 1
	}
	return float32(r.Config.Width) / float32(height)
}

func (r *Renderer) Resize(width, height uint32) error {
	r.Config.Width = width
	r.Config.Height = height
	r.Surface.Configure(r.Adapter, r.Device, r.Config)
	return r.Graph.ResizeTransients(r.Device, width, height)
}

func (r *Renderer) Reconfigure() {
	r.Surface.Configure(r.Adapter, r.Device, r.Config)
}

func RenderFrame(r *Renderer, world *ecs.World) error {
	DrainCommands(world, r)

	surfaceTexture, err := r.Surface.GetCurrentTexture()
	if err != nil {
		return wrapSurfaceErr(err)
	}

	view, err := surfaceTexture.CreateView(nil)
	if err != nil {
		return err
	}
	defer view.Release()

	r.Graph.Resources.SetExternalTexture(r.SwapchainID, view, r.Config.Width, r.Config.Height)

	encoder, err := r.Device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "frame"})
	if err != nil {
		return err
	}
	defer encoder.Release()

	if err := r.Graph.Execute(r.Device, r.Queue, world, encoder); err != nil {
		return err
	}

	cmd, err := encoder.Finish(nil)
	if err != nil {
		return err
	}
	defer cmd.Release()

	r.Queue.Submit(cmd)
	r.Surface.Present()
	return nil
}

type RendererResource struct {
	Renderer *Renderer
}

func (r *Renderer) Release() {
	if r.Graph != nil {
		r.Graph.Release()
	}
	if r.Queue != nil {
		r.Queue.Release()
	}
	if r.Device != nil {
		r.Device.Release()
	}
	if r.Adapter != nil {
		r.Adapter.Release()
	}
	if r.Surface != nil {
		r.Surface.Release()
	}
}

func isSrgb(f wgpu.TextureFormat) bool {
	switch f {
	case wgpu.TextureFormatRGBA8UnormSrgb,
		wgpu.TextureFormatBGRA8UnormSrgb:
		return true
	}
	return false
}
