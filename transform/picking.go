package transform

import "github.com/matthewjberger/indigo/ecs"

func ChainFromRootToLeaf(world *ecs.World, root, leaf ecs.Entity) []ecs.Entity {
	if root == leaf {
		return []ecs.Entity{root}
	}
	reverse := []ecs.Entity{leaf}
	current := leaf
	for current != root {
		parent, ok := ecs.Get[Parent](world, current)
		if !ok {
			return nil
		}
		current = parent.Entity
		reverse = append(reverse, current)
		if len(reverse) > MaxHierarchyDepth {
			return nil
		}
	}
	for i, j := 0, len(reverse)-1; i < j; i, j = i+1, j-1 {
		reverse[i], reverse[j] = reverse[j], reverse[i]
	}
	return reverse
}
