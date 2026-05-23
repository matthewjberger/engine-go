package app

import (
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/transform"
)

func SyncEngineTransform(engine *ecs.World, link EngineEntity, local transform.LocalTransform) {
	existing, ok := ecs.GetMut[transform.LocalTransform](engine, link.Entity)
	if !ok {
		return
	}
	*existing = local
	transform.MarkDirty(engine, link.Entity)
}

func SyncEngineTranslation(engine *ecs.World, link EngineEntity, translation transform.Vec3) {
	existing, ok := ecs.GetMut[transform.LocalTransform](engine, link.Entity)
	if !ok {
		return
	}
	existing.Translation = translation
	transform.MarkDirty(engine, link.Entity)
}

func SyncEngineRotation(engine *ecs.World, link EngineEntity, rotation transform.Quat) {
	existing, ok := ecs.GetMut[transform.LocalTransform](engine, link.Entity)
	if !ok {
		return
	}
	existing.Rotation = rotation
	transform.MarkDirty(engine, link.Entity)
}

func DespawnLinked(engine, game *ecs.World, gameEntity ecs.Entity) {
	if link, ok := ecs.Get[EngineEntity](game, gameEntity); ok {
		engine.Despawn(link.Entity)
	}
	game.Despawn(gameEntity)
}
