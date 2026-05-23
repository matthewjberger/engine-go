package render

import "github.com/matthewjberger/indigo/ecs"

// PickProxy redirects a GPU-pick hit on the carrying entity to a
// different target entity. Used so editor gizmos (camera, light)
// can hand off selection to their owning entity instead of the
// invisible proxy mesh stamped at their position.
type PickProxy struct {
	Target ecs.Entity
	Mesh   uint32
}
