package pass

import (
	"github.com/go-gl/mathgl/mgl32"

	"indigo/ecs"
	"indigo/render"
	"indigo/render/asset"
	"indigo/transform"
)

// UpdateNormalLines emits one world-space line per vertex of every
// drawn mesh when [render.GraphicsSettings.ShowNormals] is true.
// Each line starts at the vertex's world position and points along
// the vertex's world-space normal for
// [render.GraphicsSettings.NormalLineLength] units.
//
// Mirrors nightshade's sync_normal_data: the world normal uses the
// upper-3x3 of the model matrix as the normal matrix. That's
// accurate for uniform scale; non-uniform scale would need a true
// transpose-inverse normal matrix, but indigo's transform.Mat4
// only carries TRS so the approximation is fine for the scene.
//
// Push runs every frame so the lines pass picks the data up next
// draw. Frame-by-frame regeneration is fine for an interactive
// debug overlay — the cost scales with total visible vertex count
// (a few thousand for the helmet) which is well below interactive
// frame budget.
func UpdateNormalLines(world *ecs.World) {
	settings, ok := ecs.Resource[render.GraphicsSettings](world)
	if !ok || !settings.ShowNormals {
		return
	}
	linesRes, ok := ecs.Resource[LinesResource](world)
	if !ok {
		return
	}
	assetsRes, ok := ecs.Resource[asset.MeshAssetsResource](world)
	if !ok {
		return
	}
	assets := assetsRes.Assets
	lines := linesRes.Lines
	length := settings.NormalLineLength
	if length <= 0 {
		length = 0.05
	}
	color := settings.NormalLineColor

	meshMask := ecs.MustMaskOf[asset.RenderMesh](world)
	world.ForEach(meshMask, 0, func(entity ecs.Entity, _ *ecs.Archetype, _ int) {
		mesh, ok := ecs.Get[asset.RenderMesh](world, entity)
		if !ok {
			return
		}
		global, ok := ecs.Get[transform.GlobalTransform](world, entity)
		if !ok {
			return
		}
		vertices := assets.CpuVertices(mesh.Mesh)
		if len(vertices) == 0 {
			return
		}
		m := global.Matrix
		// Upper-3x3 doubles as the normal matrix for uniform scale.
		// Read once here; vertices then transform in-loop.
		for i := range vertices {
			v := &vertices[i]
			worldPos := m.Mul4x1(mgl32.Vec4{v.Position[0], v.Position[1], v.Position[2], 1})
			worldNormal := mgl32.Vec3{
				m[0]*v.Normal[0] + m[4]*v.Normal[1] + m[8]*v.Normal[2],
				m[1]*v.Normal[0] + m[5]*v.Normal[1] + m[9]*v.Normal[2],
				m[2]*v.Normal[0] + m[6]*v.Normal[1] + m[10]*v.Normal[2],
			}
			worldNormal = worldNormal.Normalize()
			start := [3]float32{worldPos.X(), worldPos.Y(), worldPos.Z()}
			end := [3]float32{
				start[0] + worldNormal.X()*length,
				start[1] + worldNormal.Y()*length,
				start[2] + worldNormal.Z()*length,
			}
			lines.AddSegment(start, end, color)
		}
	})
}
