package asset

import (
	"bytes"
	"log"

	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/render"
)

type LoadGltfBytes struct {
	Label    string
	Bytes    []byte
	OnLoaded func(world *ecs.World, scene *LoadedScene)
}

func (c LoadGltfBytes) Apply(world *ecs.World, renderer *render.Renderer) {
	assets := ecs.MustResource[MeshAssetsResource](world).Assets
	skinnedAssets := ecs.MustResource[SkinnedMeshAssetsResource](world).Assets
	arrays := ecs.MustResource[MaterialTextureArraysResource](world).Arrays
	scene, err := LoadGltfReader(renderer.Device, renderer.Queue, assets, skinnedAssets, arrays, c.Label, bytes.NewReader(c.Bytes))
	if err != nil {
		log.Printf("load_gltf_bytes: %s: %v", c.Label, err)
		return
	}
	if c.OnLoaded != nil {
		c.OnLoaded(world, scene)
	}
}
