package main

import (
	"log"
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"github.com/matthewjberger/indigo/app"
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/render"
	"github.com/matthewjberger/indigo/render/asset"
	"github.com/matthewjberger/indigo/transform"
)

func spawnSkinnedDemo(worlds app.Worlds) {
	assetsResource, ok := ecs.Resource[asset.SkinnedMeshAssetsResource](worlds.Engine)
	if !ok || assetsResource == nil {
		return
	}
	rendererResource, ok := ecs.Resource[render.RendererResource](worlds.Engine)
	if !ok || rendererResource == nil || rendererResource.Renderer == nil {
		return
	}
	device := rendererResource.Renderer.Device

	handle, err := assetsResource.Assets.Register(device, "skinned_bend_plane", buildBendPlane())
	if err != nil {
		log.Printf("skinned demo: register mesh: %v", err)
		return
	}
	skin, err := asset.NewSkin(2)
	if err != nil {
		log.Printf("skinned demo: skin: %v", err)
		return
	}

	hingeAngle := float32(math.Pi / 3.5)
	jointRoot := spawnSkinJoint(worlds, "Skin Root", transform.Vec3{3, 0, -2}, transform.QuatIdentity())
	jointHinge := spawnSkinJoint(worlds, "Skin Hinge", transform.Vec3{3, 0.5, -2}, transform.QuatFromAxisAngle(hingeAngle, transform.Vec3{1, 0, 0}))

	skin.Joints[0] = jointRoot
	skin.Joints[1] = jointHinge
	skin.InverseBindMatrices[0] = mgl32.Ident4()
	skin.InverseBindMatrices[1] = mgl32.Translate3D(0, -0.5, 0)

	entityMask := ecs.MustMaskOf[transform.LocalTransform](worlds.Engine) |
		ecs.MustMaskOf[transform.GlobalTransform](worlds.Engine) |
		ecs.MustMaskOf[transform.LocalTransformDirty](worlds.Engine) |
		ecs.MustMaskOf[asset.SkinnedMesh](worlds.Engine) |
		ecs.MustMaskOf[asset.Material](worlds.Engine) |
		ecs.MustMaskOf[app.Name](worlds.Engine)
	entity := worlds.Engine.Spawn(entityMask)
	local := transform.IdentityLocalTransform()
	local.Translation = transform.Vec3{3, 0, -2}
	ecs.Set(worlds.Engine, entity, local)
	ecs.Set(worlds.Engine, entity, transform.IdentityGlobalTransform())
	ecs.Set(worlds.Engine, entity, asset.SkinnedMesh{Mesh: handle, Skin: skin})

	mat := asset.DefaultMaterial()
	mat.BaseColor = [4]float32{0.4, 0.7, 1.0, 0.55}
	mat.AlphaMode = asset.AlphaModeBlend
	ecs.Set(worlds.Engine, entity, mat)
	ecs.Set(worlds.Engine, entity, app.Name{Value: "Skinned Plane"})
}

func spawnSkinJoint(worlds app.Worlds, name string, translation transform.Vec3, rotation transform.Quat) ecs.Entity {
	mask := ecs.MustMaskOf[transform.LocalTransform](worlds.Engine) |
		ecs.MustMaskOf[transform.GlobalTransform](worlds.Engine) |
		ecs.MustMaskOf[transform.LocalTransformDirty](worlds.Engine) |
		ecs.MustMaskOf[app.Name](worlds.Engine)
	entity := worlds.Engine.Spawn(mask)
	local := transform.IdentityLocalTransform()
	local.Translation = translation
	local.Rotation = rotation
	ecs.Set(worlds.Engine, entity, local)
	ecs.Set(worlds.Engine, entity, transform.IdentityGlobalTransform())
	ecs.Set(worlds.Engine, entity, app.Name{Value: name})
	return entity
}

func buildBendPlane() []asset.SkinnedMeshVertex {
	return []asset.SkinnedMeshVertex{
		{Position: [4]float32{-0.5, 0, 0, 1}, Normal: [4]float32{0, 0, 1, 0}, Tangent: [4]float32{1, 0, 0, 1}, UV: [4]float32{0, 1, 0, 0}, Color: [4]float32{0.6, 0.7, 0.95, 1}, JointIndices: [4]uint32{0, 0, 0, 0}, JointWeights: [4]float32{1, 0, 0, 0}},
		{Position: [4]float32{0.5, 0, 0, 1}, Normal: [4]float32{0, 0, 1, 0}, Tangent: [4]float32{1, 0, 0, 1}, UV: [4]float32{1, 1, 0, 0}, Color: [4]float32{0.6, 0.7, 0.95, 1}, JointIndices: [4]uint32{0, 0, 0, 0}, JointWeights: [4]float32{1, 0, 0, 0}},
		{Position: [4]float32{0.5, 1, 0, 1}, Normal: [4]float32{0, 0, 1, 0}, Tangent: [4]float32{1, 0, 0, 1}, UV: [4]float32{1, 0, 0, 0}, Color: [4]float32{0.4, 0.55, 0.9, 1}, JointIndices: [4]uint32{1, 0, 0, 0}, JointWeights: [4]float32{1, 0, 0, 0}},
		{Position: [4]float32{0.5, 1, 0, 1}, Normal: [4]float32{0, 0, 1, 0}, Tangent: [4]float32{1, 0, 0, 1}, UV: [4]float32{1, 0, 0, 0}, Color: [4]float32{0.4, 0.55, 0.9, 1}, JointIndices: [4]uint32{1, 0, 0, 0}, JointWeights: [4]float32{1, 0, 0, 0}},
		{Position: [4]float32{-0.5, 1, 0, 1}, Normal: [4]float32{0, 0, 1, 0}, Tangent: [4]float32{1, 0, 0, 1}, UV: [4]float32{0, 0, 0, 0}, Color: [4]float32{0.4, 0.55, 0.9, 1}, JointIndices: [4]uint32{1, 0, 0, 0}, JointWeights: [4]float32{1, 0, 0, 0}},
		{Position: [4]float32{-0.5, 0, 0, 1}, Normal: [4]float32{0, 0, 1, 0}, Tangent: [4]float32{1, 0, 0, 1}, UV: [4]float32{0, 1, 0, 0}, Color: [4]float32{0.6, 0.7, 0.95, 1}, JointIndices: [4]uint32{0, 0, 0, 0}, JointWeights: [4]float32{1, 0, 0, 0}},
	}
}
