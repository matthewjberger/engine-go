package ecs

// The empty Lock/Unlock make go vet's copylocks analyzer flag copies of any struct embedding noCopy; removing them silently disables that protection.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
