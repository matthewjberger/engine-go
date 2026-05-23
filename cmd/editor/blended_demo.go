package main

import (
	"fmt"

	"github.com/matthewjberger/indigo/app"
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/render/asset"
	"github.com/matthewjberger/indigo/transform"
)

func spawnBlendedSpheres(worlds app.Worlds, orbMesh asset.MeshHandle) {
	mask := ecs.MustMaskOf[transform.LocalTransform](worlds.Engine) |
		ecs.MustMaskOf[transform.GlobalTransform](worlds.Engine) |
		ecs.MustMaskOf[transform.LocalTransformDirty](worlds.Engine) |
		ecs.MustMaskOf[asset.RenderMesh](worlds.Engine) |
		ecs.MustMaskOf[asset.Material](worlds.Engine) |
		ecs.MustMaskOf[app.Name](worlds.Engine)

	colors := [][4]float32{
		{1.0, 0.1, 0.1, 0.85},
		{0.1, 1.0, 0.2, 0.55},
		{0.1, 0.3, 1.0, 0.35},
		{1.0, 0.8, 0.1, 0.20},
	}
	for index, color := range colors {
		entity := worlds.Engine.Spawn(mask)
		local := transform.IdentityLocalTransform()
		local.Translation = transform.Vec3{-3, 0.8, float32(index)*1.4 - 2.5}
		local.Scale = transform.Vec3{0.6, 0.6, 0.6}
		ecs.Set(worlds.Engine, entity, local)
		ecs.Set(worlds.Engine, entity, transform.IdentityGlobalTransform())
		ecs.Set(worlds.Engine, entity, asset.RenderMesh{Mesh: orbMesh})
		ecs.Set(worlds.Engine, entity, asset.Material{
			BaseColor:              color,
			BaseColorLayer:         asset.NoTextureLayer,
			NormalLayer:            asset.NoTextureLayer,
			MetallicRoughnessLayer: asset.NoTextureLayer,
			OcclusionLayer:         asset.NoTextureLayer,
			EmissiveLayer:          asset.NoTextureLayer,
			MetallicFactor:         0.0,
			RoughnessFactor:        0.5,
			EmissiveFactor:         [3]float32{0, 0, 0},
			EmissiveStrength:       1.0,
			NormalScale:            1.0,
			OcclusionStrength:      1.0,
			AlphaMode:              asset.AlphaModeBlend,
			AlphaCutoff:            0.5,
			IOR:                    1.5,
		})
		ecs.Set(worlds.Engine, entity, app.Name{Value: fmt.Sprintf("Blended Sphere %d", index)})
	}
}
