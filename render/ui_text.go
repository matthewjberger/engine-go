package render

import (
	_ "embed"
	"fmt"

	"github.com/cogentcore/webgpu/wgpu"

	"indigo/ecs"
	"indigo/ui"
)

//go:embed ui_text.wgsl
var uiTextShader string

const uiTextGlyphBytes = uint64(48)
const uiTextMinCapacity uint32 = 256

type uiTextGlyphInstance struct {
	Rect  [4]float32
	Color [4]float32
	Atlas [4]float32
}

type uiTextPassState struct {
	pipeline        *wgpu.RenderPipeline
	bindGroupLayout *wgpu.BindGroupLayout
	viewportBuffer  *wgpu.Buffer
	instanceBuffer  *wgpu.Buffer
	atlasTexture    *wgpu.Texture
	atlasView       *wgpu.TextureView
	atlasSampler    *wgpu.Sampler
	bindGroup       *wgpu.BindGroup
	capacity        uint32
	count           uint32
	scratch         []uiTextGlyphInstance

	atlas ui.FontAtlas
}

// NewUiTextPass builds the screen-space text pass that draws labels
// attached to UI entities via [ui.Text]. The hand-rolled bitmap font
// (see [ui.BuildFontAtlas]) is uploaded once at construction; each
// frame the pass walks the UI world for entities with both a Node
// and a Text, packs per-character glyph instances into a storage
// buffer, and renders alpha-blended quads sampling the atlas.
func NewUiTextPass(device *wgpu.Device, queue *wgpu.Queue, surfaceFormat wgpu.TextureFormat) (*Pass, error) {
	atlas := ui.BuildFontAtlas()
	state := &uiTextPassState{atlas: atlas}

	bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "ui_text bind group layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageVertex,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageVertex,
				Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage},
			},
			{
				Binding:    2,
				Visibility: wgpu.ShaderStageFragment,
				Texture: wgpu.TextureBindingLayout{
					SampleType:    wgpu.TextureSampleTypeFloat,
					ViewDimension: wgpu.TextureViewDimension2D,
				},
			},
			{
				Binding:    3,
				Visibility: wgpu.ShaderStageFragment,
				Sampler:    wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ui_text: bind group layout: %w", err)
	}
	state.bindGroupLayout = bindGroupLayout

	viewportBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "ui_text viewport",
		Size:  16,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("ui_text: viewport buffer: %w", err)
	}
	state.viewportBuffer = viewportBuffer

	atlasTex, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "ui_text atlas",
		Size: wgpu.Extent3D{
			Width:              atlas.Width,
			Height:             atlas.Height,
			DepthOrArrayLayers: 1,
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatR8Unorm,
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("ui_text: atlas texture: %w", err)
	}
	state.atlasTexture = atlasTex

	queue.WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture: atlasTex,
			Aspect:  wgpu.TextureAspectAll,
		},
		atlas.Pixels,
		&wgpu.TextureDataLayout{
			Offset:       0,
			BytesPerRow:  atlas.Width,
			RowsPerImage: atlas.Height,
		},
		&wgpu.Extent3D{
			Width:              atlas.Width,
			Height:             atlas.Height,
			DepthOrArrayLayers: 1,
		},
	)

	atlasView, err := atlasTex.CreateView(nil)
	if err != nil {
		return nil, fmt.Errorf("ui_text: atlas view: %w", err)
	}
	state.atlasView = atlasView

	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:         "ui_text sampler",
		AddressModeU:  wgpu.AddressModeClampToEdge,
		AddressModeV:  wgpu.AddressModeClampToEdge,
		MagFilter:     wgpu.FilterModeNearest,
		MinFilter:     wgpu.FilterModeNearest,
		MipmapFilter:  wgpu.MipmapFilterModeNearest,
		MaxAnisotropy: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("ui_text: sampler: %w", err)
	}
	state.atlasSampler = sampler

	if err := ensureUiTextCapacity(state, device, uiTextMinCapacity); err != nil {
		return nil, err
	}

	shader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "ui_text shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: uiTextShader},
	})
	if err != nil {
		return nil, fmt.Errorf("ui_text: shader: %w", err)
	}
	defer shader.Release()

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "ui_text pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("ui_text: pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()

	pipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "ui_text pipeline",
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{Module: shader, EntryPoint: "vertex_main"},
		Primitive: wgpu.PrimitiveState{
			Topology: wgpu.PrimitiveTopologyTriangleList,
			CullMode: wgpu.CullModeNone,
		},
		Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module:     shader,
			EntryPoint: "fragment_main",
			Targets: []wgpu.ColorTargetState{{
				Format:    surfaceFormat,
				WriteMask: wgpu.ColorWriteMaskAll,
				Blend: &wgpu.BlendState{
					Color: wgpu.BlendComponent{
						SrcFactor: wgpu.BlendFactorSrcAlpha,
						DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
						Operation: wgpu.BlendOperationAdd,
					},
					Alpha: wgpu.BlendComponent{
						SrcFactor: wgpu.BlendFactorOne,
						DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
						Operation: wgpu.BlendOperationAdd,
					},
				},
			}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ui_text: pipeline: %w", err)
	}
	state.pipeline = pipeline

	return &Pass{
		Name:    "ui_text",
		Writes:  []string{"color"},
		State:   state,
		Prepare: uiTextPrepare,
		Execute: uiTextExecute,
		Release: uiTextRelease,
	}, nil
}

func uiTextPrepare(s any, context *PassContext) error {
	state := s.(*uiTextPassState)
	state.count = 0
	state.scratch = state.scratch[:0]

	if !ui.HasUI(context.World) {
		return nil
	}
	uiWorld := ecs.Resource[ui.WorldRef](context.World).World

	mask := ecs.MaskOf[ui.Node](uiWorld) | ecs.MaskOf[ui.Text](uiWorld)
	atlasWidth := float32(state.atlas.Width)
	cellW := float32(state.atlas.GlyphWidth)
	cellH := float32(state.atlas.GlyphHeight)

	uiWorld.ForEach(mask, 0, func(entity ecs.Entity, table *ecs.Archetype, index int) {
		nodes, _ := ecs.Column[ui.Node](uiWorld, table)
		texts, _ := ecs.Column[ui.Text](uiWorld, table)
		node := &nodes[index]
		text := &texts[index]
		if text.Content == "" {
			return
		}
		scale := text.Scale
		if scale <= 0 {
			scale = 1
		}
		glyphW := cellW * scale
		glyphH := cellH * scale
		advance := (cellW + 1) * scale

		var pen float32
		for _, r := range text.Content {
			column := state.atlas.LookupGlyph(r)
			pen += advance
			if column == 0 && r != ' ' {
				continue
			}
			pen -= advance
			break
		}
		labelWidth := float32(len([]rune(text.Content))) * advance
		_ = pen

		originX := node.Resolved.X + (node.Resolved.Width-labelWidth)*0.5
		originY := node.Resolved.Y + (node.Resolved.Height-glyphH)*0.5

		cursor := originX
		for _, r := range text.Content {
			column := state.atlas.LookupGlyph(r)
			if r == ' ' || column == 0 {
				cursor += advance
				continue
			}
			atlasX := float32(column) * cellW
			state.scratch = append(state.scratch, uiTextGlyphInstance{
				Rect:  [4]float32{cursor, originY, glyphW, glyphH},
				Color: text.Color,
				Atlas: [4]float32{atlasX, cellW, cellW, atlasWidth},
			})
			cursor += advance
		}
	})

	state.count = uint32(len(state.scratch))
	if state.count == 0 {
		return nil
	}
	if err := ensureUiTextCapacity(state, context.Device, state.count); err != nil {
		return err
	}

	viewport := uiQuadViewport{
		Width:  float32(ecs.Resource[RendererResource](context.World).Renderer.Config.Width),
		Height: float32(ecs.Resource[RendererResource](context.World).Renderer.Config.Height),
	}
	writeBuffer(context.Device, context.Queue, context.Encoder, state.viewportBuffer, 0, bytesOf(&viewport))

	instanceBytes := uint64(state.count) * uiTextGlyphBytes
	writeBuffer(context.Device, context.Queue, context.Encoder, state.instanceBuffer, 0, bytesOfN(&state.scratch[0], instanceBytes))

	return nil
}

func uiTextExecute(s any, context *PassContext) error {
	state := s.(*uiTextPassState)
	if state.count == 0 {
		return nil
	}
	attachment, err := context.ColorAttachment("color")
	if err != nil {
		return err
	}
	pass := context.Encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label:            "ui_text",
		ColorAttachments: []wgpu.RenderPassColorAttachment{attachment},
	})
	pass.SetPipeline(state.pipeline)
	pass.SetBindGroup(0, state.bindGroup, nil)
	pass.Draw(6, state.count, 0, 0)
	pass.End()
	pass.Release()
	return nil
}

func uiTextRelease(s any) {
	state := s.(*uiTextPassState)
	if state.bindGroup != nil {
		state.bindGroup.Release()
	}
	if state.instanceBuffer != nil {
		state.instanceBuffer.Release()
	}
	if state.atlasSampler != nil {
		state.atlasSampler.Release()
	}
	if state.atlasView != nil {
		state.atlasView.Release()
	}
	if state.atlasTexture != nil {
		state.atlasTexture.Release()
	}
	if state.viewportBuffer != nil {
		state.viewportBuffer.Release()
	}
	if state.pipeline != nil {
		state.pipeline.Release()
	}
	if state.bindGroupLayout != nil {
		state.bindGroupLayout.Release()
	}
}

func ensureUiTextCapacity(state *uiTextPassState, device *wgpu.Device, required uint32) error {
	if state.capacity >= required && state.instanceBuffer != nil && state.bindGroup != nil {
		return nil
	}
	newCapacity := state.capacity
	if newCapacity == 0 {
		newCapacity = uiTextMinCapacity
	}
	for newCapacity < required {
		newCapacity *= 2
	}
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "ui_text instances",
		Size:  uint64(newCapacity) * uiTextGlyphBytes,
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("ui_text: instance buffer: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "ui_text bind group",
		Layout: state.bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: state.viewportBuffer, Offset: 0, Size: 16},
			{Binding: 1, Buffer: buffer, Offset: 0, Size: wgpu.WholeSize},
			{Binding: 2, TextureView: state.atlasView},
			{Binding: 3, Sampler: state.atlasSampler},
		},
	})
	if err != nil {
		buffer.Release()
		return fmt.Errorf("ui_text: bind group: %w", err)
	}
	if state.bindGroup != nil {
		state.bindGroup.Release()
	}
	if state.instanceBuffer != nil {
		state.instanceBuffer.Release()
	}
	state.instanceBuffer = buffer
	state.bindGroup = bindGroup
	state.capacity = newCapacity
	return nil
}
