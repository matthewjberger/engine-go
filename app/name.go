package app

// Name is a human-readable label attached to an engine entity.
// The editor's entity tree and inspector use it to surface entities
// to the user; engine systems otherwise ignore it. Optional - any
// entity without a Name component is filtered out of the tree.
type Name struct {
	Value string
}
