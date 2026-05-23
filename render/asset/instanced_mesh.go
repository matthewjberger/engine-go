package asset

import "github.com/go-gl/mathgl/mgl32"

type InstancedMesh struct {
	Mesh      MeshHandle
	Instances []mgl32.Mat4
}
