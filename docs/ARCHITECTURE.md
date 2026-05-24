# Architecture

indigo is a data-oriented Go game engine. Everything in the engine is either a plain struct of data, a free function over that data, or a typed handle into a table of those structs. Nothing in the engine is "an object with methods that own business logic." This document explains how the pieces fit, what trade-offs each piece is making, and what each piece refuses to do.

## Storage

The `ecs` package is an archetype Entity Component System. Components are plain structs registered against a world. Entities are 64-bit generational handles. The storage lays out each component type as its own contiguous slice and groups entities into tables keyed by the exact set of component types they carry. Iteration walks the table directly through typed columns, which is a packed-memory loop the hardware prefetcher likes.

Adding or removing a component on an entity migrates that entity to a different table. Migrations are O(component-count-on-the-entity) and rare next to per-frame iteration, so the design pays the migration cost to keep the iteration cost flat. The ceiling is 64 component types per world, which is plenty if you split your data across more than one world.

Resources are world-scoped singletons keyed by Go type. `type DeltaTime float32` and `type GameTime float32` are distinct slots even though they share the same underlying numeric type. Two flavors of lookup exist for the standard reasons: `Resource[T]` returns `(*T, bool)` for defensive code, `MustResource[T]` panics for code that knows the resource is set.

Generics carry the typed accessors: `Get[T]`, `Set[T]`, `Resource[T]`, `Iter1..6[T1..T6]`. Everything monomorphizes at compile time, so the iteration body sees a concrete `*Position` and a concrete `*Velocity`, not an `interface{}`. The `Iter` family is templated through a `go:generate` step rather than written by hand.

## Worlds

The engine ships three worlds, not one.

The **engine world** holds rendering data. Transforms, mesh handles, materials, lights, the camera, input, the renderer handle itself, graphics settings, the window's viewport and timing. Anything a render pass reads lives here.

The **game world** holds gameplay data. The breakout demo's paddle, ball, bricks, and game state live in the game world. The editor's spinner system lives there too. Gameplay code reads and writes the game world freely; cross-world reach goes through named sync helpers like `app.SyncEngineTransform`, not through scattered `EngineRef.World.Mutate(...)` calls. The discipline is "go through a named bridge or don't reach."

The **UI world** is a third instance reserved for retained-UI state: nodes, colors, text, parents, hit-test rects, focus, text-input buffers. The layout system and the hit-test system are systems on this world; the renderer's UI passes iterate this world's components like any other render pass iterates the engine world's. Treating the UI as a regular ECS world means it gets every property the rest of the engine has — query iteration, change detection, command buffers — without a parallel scene graph.

Cross-world bridges are typed resources installed at construction time. `ui.WorldRef` on the engine world points the renderer at the UI world. `app.EngineRef` on the game world points sync helpers at the engine world. `app.NewWorlds(renderer)` constructs all three together and installs both bridges, so a hand-built app cannot silently forget one.

## Render graph

The renderer is a declarative pass DAG, not a hardcoded sequence of draws.

A pass is a struct of function values plus a slot manifest:

```go
type Pass struct {
    Name     string
    Reads    []string
    Writes   []string
    State    any
    Prepare  func(state any, ctx *PassContext) error
    Execute  func(state any, ctx *PassContext) error
    Release  func(state any)
    InvalidateBindGroups func(state any)
}
```

Apps build passes (`render.AddSkyPass`, `render.AddMeshPass`, ...) and bind their `Reads` / `Writes` slot names to typed resource IDs. `Compile` topologically sorts the passes by their dependencies, allocates any transient textures whose handles are still empty, decides clear-vs-load by who writes first, and freezes an execution order. `RenderFrame` then walks the sorted list every frame and invokes each pass's `Prepare` then `Execute`.

The slot indirection means a pass's source code references "color" and "depth" without baking in which concrete textures they map to. A new app can construct its own slot wiring without touching the pass implementation. Adding a pass is one function call plus a slot manifest.

Bind groups are version-tracked. Resources stamp a counter every time their underlying handle changes (transient reallocations on resize, external view replacement each frame). A pass records the version it cached its bind groups against; on a mismatch the pass's `InvalidateBindGroups` runs and rebuilds them. Write-only attachments don't trigger invalidation, so a pass that writes to the swapchain doesn't rebuild on every present.

Pass state is `any`. The pass implementation casts at the top of each function. This is the Go-shaped way to type-erase heterogeneous pass states stored in a slice; the cast costs one type assertion per call and keeps the data fully separate from the behavior.

## Systems

A system is a free function with signature `func(*ecs.World)`. Systems read and write components and resources through the typed generics, never through method calls on the world. They are ordered through a named schedule:

```go
worlds.EngineSchedule.Push("graphics_toggles", render.UpdateGraphicsToggles)
worlds.EngineSchedule.Push("pan_orbit_camera", render.UpdatePanOrbitCamera)
worlds.EngineSchedule.Push("transform_propagation", transform.UpdateGlobalTransforms)
```

The schedule runs the systems in insertion order each frame. Inter-system data flows through the world's resources (a system writes a resource, the next system reads it) rather than through arguments, which keeps every system's signature identical and trivial to test in isolation.

There are no method overrides, no virtual dispatch through interfaces in the systems layer, and no "system manager." A schedule is a slice of named function values plus a `Run(*World)` method. The named entries exist for tracing and for the schedule to refuse duplicates.

## Asset pipeline

Mesh, texture, and glTF loading sit on three typed caches on the engine world.

`MeshAssets` registers vertex buffers and returns `MeshHandle` indices. Every registered mesh carries its local-space AABB so the bounding-volume overlay and any spatial query can read bounds without re-walking vertices.

`TextureCache` registers GPU textures with full mip chains. Callers pick `TextureSRGB` or `TextureLinear` at register time so base color and emissive maps land in sRGB while normal, metallic-roughness, and occlusion maps stay linear. A 1x1 white texture is registered by default and bound to every mesh handle that hasn't been given a real texture, so the shader's `textureSample` call always has a valid binding.

`LoadGltfReader` parses a `.glb` or `.gltf` document, uploads every primitive into the mesh cache, decodes embedded PNG / JPEG images into the texture cache (with auto-classification by sRGB vs linear, based on which material slot references each texture), and returns a `LoadedScene` that mirrors the glTF node hierarchy. `SpawnLoadedScene` materializes the scene as ECS entities with `transform.Parent` links so the engine's transform propagation handles world matrices automatically.

The load step is pure CPU work that produces a `LoadedScene`. The spawn step takes that scene and writes ECS entities. The split means an app can inspect or transform the loaded data before any entity is spawned, or swap in its own spawner with different defaults.

## Frame

A frame is procedural, written out in the platform layer. There is no `engine.Run()` that takes control.

```
glfw.PollEvents
compute delta
sync UI pointer state from the raw input snapshot
refresh HUD layout, consume tree-scroll wheel

TickFrame(worlds, app, delta)
  window.Advance on each world's timing
  UI schedule, then game schedule, then engine schedule
  app.RunSystems, app.PreRender (optional)
  ApplyCommands on each world

handle right-click, drive text inputs, handle UI clicks
refresh per-frame HUD state (menus, hovers, tree, inspector, caret)

renderer.RenderFrame(engine)
  set swapchain external resource
  execute compiled graph passes in topo order
  present

ProcessPickingReadback, handle any pending pick result
PostFrame
  Step on each world (rolls events + change-detection watermarks)
  Input.BeginFrame (clears per-frame deltas)
```

Reading the file tells you what runs each frame, in what order, without inheriting from a base class.

## Tradeoffs the design refuses

The engine deliberately stays narrow in places it could grow:

- Transient textures aren't aliased. Each transient is its own GPU texture today. A pooling layer would save memory on large graphs but isn't load-bearing for the size of graphs the engine runs.
- There's no tangent generation. The glTF loader reads a primitive's TANGENT attribute when it has one and otherwise falls back to a constant tangent, so normal maps on meshes authored without tangents render wrong until the asset is reexported with them.
- Skins are capped at 128 joints (`MaxJointsPerSkin`). A skin that needs more is rejected when its `Skin` is constructed rather than silently truncated.

These are scope decisions, not regressions. The point of writing them down is so the engine's shape stays honest about what it does and doesn't promise.
