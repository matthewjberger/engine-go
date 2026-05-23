package asset

import (
	"fmt"
	"unsafe"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/matthewjberger/indigo/ecs"
)

const MaxJointsPerSkin = 128

type SkinnedMeshVertex struct {
	Position     [4]float32
	Normal       [4]float32
	Tangent      [4]float32
	UV           [4]float32
	Color        [4]float32
	JointIndices [4]uint32
	JointWeights [4]float32
}

type SkinnedMeshHandle uint32

type Skin struct {
	Joints              []ecs.Entity
	InverseBindMatrices []mgl32.Mat4
}

func NewSkin(jointCount int) (*Skin, error) {
	if jointCount <= 0 || jointCount > MaxJointsPerSkin {
		return nil, fmt.Errorf("skin: joint count %d outside [1, %d]", jointCount, MaxJointsPerSkin)
	}
	return &Skin{
		Joints:              make([]ecs.Entity, jointCount),
		InverseBindMatrices: make([]mgl32.Mat4, jointCount),
	}, nil
}

type skinnedMeshEntry struct {
	Name        string
	Vertices    *wgpu.Buffer
	VertexCount uint32
	Bounds      BoundingVolume
	CpuVertices []SkinnedMeshVertex
}

type SkinnedMeshAssets struct {
	entries []skinnedMeshEntry
}

func NewSkinnedMeshAssets() *SkinnedMeshAssets {
	return &SkinnedMeshAssets{}
}

func (assets *SkinnedMeshAssets) Register(device *wgpu.Device, name string, vertices []SkinnedMeshVertex) (SkinnedMeshHandle, error) {
	if len(vertices) == 0 {
		return 0, fmt.Errorf("skinned mesh %q: empty vertex slice", name)
	}
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "skinned mesh vertex buffer: " + name,
		Size:  uint64(len(vertices)) * uint64(unsafe.Sizeof(SkinnedMeshVertex{})),
		Usage: wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return 0, fmt.Errorf("skinned mesh %q: buffer: %w", name, err)
	}
	device.GetQueue().WriteBuffer(buffer, 0, unsafe.Slice((*byte)(unsafe.Pointer(&vertices[0])), len(vertices)*int(unsafe.Sizeof(SkinnedMeshVertex{}))))
	cpu := make([]SkinnedMeshVertex, len(vertices))
	copy(cpu, vertices)
	bounds := computeSkinnedBounds(cpu)
	handle := SkinnedMeshHandle(len(assets.entries))
	assets.entries = append(assets.entries, skinnedMeshEntry{
		Name:        name,
		Vertices:    buffer,
		VertexCount: uint32(len(vertices)),
		Bounds:      bounds,
		CpuVertices: cpu,
	})
	return handle, nil
}

func (assets *SkinnedMeshAssets) Lookup(handle SkinnedMeshHandle) (*skinnedMeshEntry, bool) {
	if int(handle) >= len(assets.entries) {
		return nil, false
	}
	return &assets.entries[handle], true
}

func (assets *SkinnedMeshAssets) Release() {
	for index := range assets.entries {
		if assets.entries[index].Vertices != nil {
			assets.entries[index].Vertices.Release()
			assets.entries[index].Vertices = nil
		}
	}
	assets.entries = nil
}

type SkinnedMeshAssetsResource struct {
	Assets *SkinnedMeshAssets
}

type SkinnedMesh struct {
	Mesh SkinnedMeshHandle
	Skin *Skin
}

func computeSkinnedBounds(vertices []SkinnedMeshVertex) BoundingVolume {
	if len(vertices) == 0 {
		return BoundingVolume{}
	}
	minPt := [3]float32{vertices[0].Position[0], vertices[0].Position[1], vertices[0].Position[2]}
	maxPt := minPt
	for index := 1; index < len(vertices); index++ {
		for component := 0; component < 3; component++ {
			value := vertices[index].Position[component]
			if value < minPt[component] {
				minPt[component] = value
			}
			if value > maxPt[component] {
				maxPt[component] = value
			}
		}
	}
	return BoundingVolume{Min: minPt, Max: maxPt}
}
