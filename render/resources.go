package render

import "github.com/cogentcore/webgpu/wgpu"

type ResourceID uint32

type ResourceKind uint8

const (
	ResourceKindExternalColor ResourceKind = iota
	ResourceKindTransientColor
	ResourceKindExternalDepth
	ResourceKindTransientDepth
)

type TextureDescriptor struct {
	Format wgpu.TextureFormat
	Width  uint32
	Height uint32
	Usage  wgpu.TextureUsage
}

type ResourceDescriptor struct {
	Name string
	Kind ResourceKind

	Texture TextureDescriptor

	ClearColor *wgpu.Color

	ClearDepth *float32
}

type TextureHandle struct {
	Texture *wgpu.Texture
	View    *wgpu.TextureView
	Width   uint32
	Height  uint32

	Owned bool
}

type Resources struct {
	Descriptors []ResourceDescriptor
	Handles     []TextureHandle
	Versions    []uint64
}

func (r *Resources) Register(descriptor ResourceDescriptor) ResourceID {
	id := ResourceID(len(r.Descriptors))
	r.Descriptors = append(r.Descriptors, descriptor)
	r.Handles = append(r.Handles, TextureHandle{})
	r.Versions = append(r.Versions, 0)
	return id
}

func (r *Resources) SetExternalTexture(id ResourceID, view *wgpu.TextureView, width, height uint32) {
	r.Handles[id] = TextureHandle{View: view, Width: width, Height: height, Owned: false}
	r.Versions[id]++
}

func (r *Resources) Descriptor(id ResourceID) *ResourceDescriptor {
	return &r.Descriptors[id]
}

func (r *Resources) Handle(id ResourceID) *TextureHandle {
	return &r.Handles[id]
}

func (r *Resources) Version(id ResourceID) uint64 {
	return r.Versions[id]
}

func (r *Resources) ReleaseOwned() {
	for index := range r.Handles {
		handle := &r.Handles[index]
		if !handle.Owned {
			continue
		}
		if handle.View != nil {
			handle.View.Release()
		}
		if handle.Texture != nil {
			handle.Texture.Release()
		}
		*handle = TextureHandle{}
	}
}
