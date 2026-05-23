package app

import (
	"github.com/matthewjberger/indigo/ecs"
	"github.com/matthewjberger/indigo/transform"
)

// SyncEngineTransform writes a full LocalTransform onto the engine
// entity linked by an [EngineEntity] and marks it dirty so the
// transform propagation system rebuilds the GlobalTransform.
//
// The point of the named helper is convention: game-side systems
// should never reach into engine [transform.LocalTransform]
// directly; they always go through this API. The function is a
// no-op for stale links.
func SyncEngineTransform(engine *ecs.World, link EngineEntity, local transform.LocalTransform) {
	existing, ok := ecs.GetMut[transform.LocalTransform](engine, link.Entity)
	if !ok {
		return
	}
	*existing = local
	transform.MarkDirty(engine, link.Entity)
}

// SyncEngineTranslation writes only the translation field of the
// engine entity's LocalTransform. Useful for "this game thing moved"
// without overwriting rotation or scale.
func SyncEngineTranslation(engine *ecs.World, link EngineEntity, translation transform.Vec3) {
	existing, ok := ecs.GetMut[transform.LocalTransform](engine, link.Entity)
	if !ok {
		return
	}
	existing.Translation = translation
	transform.MarkDirty(engine, link.Entity)
}

// SyncEngineRotation writes only the rotation field of the engine
// entity's LocalTransform.
func SyncEngineRotation(engine *ecs.World, link EngineEntity, rotation transform.Quat) {
	existing, ok := ecs.GetMut[transform.LocalTransform](engine, link.Entity)
	if !ok {
		return
	}
	existing.Rotation = rotation
	transform.MarkDirty(engine, link.Entity)
}

// DespawnLinked despawns both the game entity and its linked engine
// entity (if any).
func DespawnLinked(engine, game *ecs.World, gameEntity ecs.Entity) {
	if link, ok := ecs.Get[EngineEntity](game, gameEntity); ok {
		engine.Despawn(link.Entity)
	}
	game.Despawn(gameEntity)
}
