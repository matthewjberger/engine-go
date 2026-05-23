package render

import (
	"fmt"
	"sort"

	"github.com/cogentcore/webgpu/wgpu"

	"github.com/matthewjberger/indigo/ecs"
)

// SlotBinding wires a pass-local slot name to a graph resource. The graph
// uses the pair to resolve resources during execute, compute the
// first-write set that drives clear-vs-load, and build the dependency
// DAG for the topological sort.
type SlotBinding struct {
	Slot       string
	ResourceID ResourceID
}

// passEntry is the graph's per-pass record: the user-supplied [Pass] plus
// the wiring from its slot names to graph resources and the per-frame
// read-slot version snapshot used to drive bind-group invalidation.
//
// Only Read-slot versions are tracked. Bind groups bind reads;
// attachments don't. Diffing every slot would invalidate present's
// bind group every frame because the swapchain (a write-only output)
// bumps its version on each frame's external-texture refresh.
type passEntry struct {
	pass         *Pass
	bindings     map[string]ResourceID
	lastVersions map[ResourceID]uint64
}

// Graph is the data-oriented render graph. It owns the resource table
// and a list of passes. [Compile] topologically sorts passes by their
// read/write dependencies, computes the first-write set for clear ops,
// and allocates any unallocated transient textures.
type Graph struct {
	Resources Resources

	passes         []passEntry
	executionOrder []int
	clearOps       map[ClearKey]struct{}
	compiled       bool
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{clearOps: make(map[ClearKey]struct{})}
}

// ResourceByName returns the id of the first resource whose descriptor
// has the given name, or [ResourceID](0) if there is none.
func (g *Graph) ResourceByName(name string) ResourceID {
	for index := range g.Resources.Descriptors {
		if g.Resources.Descriptors[index].Name == name {
			return ResourceID(index)
		}
	}
	return 0
}

// AddColorTexture registers a color texture in the graph and returns
// its id. Set kind to [ResourceKindExternalColor] for the swapchain
// (the caller refreshes the view each frame); use
// [ResourceKindTransientColor] for graph-allocated intermediates like
// scene_color. Transient textures registered before [Compile] are
// auto-allocated during compile; registering after compile requires
// re-compiling.
func (g *Graph) AddColorTexture(descriptor ResourceDescriptor) ResourceID {
	g.compiled = false
	return g.Resources.Register(descriptor)
}

// AddDepthTexture registers a depth texture.
func (g *Graph) AddDepthTexture(descriptor ResourceDescriptor) ResourceID {
	g.compiled = false
	return g.Resources.Register(descriptor)
}

// AddPass registers a pass with its slot bindings. The pass's declared
// [Pass.Reads] and [Pass.Writes] are slot names; every slot they
// reference must be bound here.
func (g *Graph) AddPass(pass *Pass, bindings []SlotBinding) error {
	indexed := make(map[string]ResourceID, len(bindings))
	for _, binding := range bindings {
		indexed[binding.Slot] = binding.ResourceID
	}
	for _, slot := range pass.Reads {
		if _, ok := indexed[slot]; !ok {
			return fmt.Errorf("render: pass %q reads slot %q with no binding", pass.Name, slot)
		}
	}
	for _, slot := range pass.Writes {
		if _, ok := indexed[slot]; !ok {
			return fmt.Errorf("render: pass %q writes slot %q with no binding", pass.Name, slot)
		}
	}
	g.passes = append(g.passes, passEntry{
		pass:         pass,
		bindings:     indexed,
		lastVersions: make(map[ResourceID]uint64, len(pass.Reads)),
	})
	g.compiled = false
	return nil
}

// Compile freezes the graph for execution:
//   - topologically sorts passes by their read/write dependencies
//   - records the first-write set that drives clear-vs-load
//   - allocates any unallocated transient textures against device
//
// The dependency rules: a pass that reads resource R depends on the
// most recent pass that wrote R; a pass that writes resource R after
// some other pass also wrote R depends on the previous writer
// (write-after-write ordering). Stable insertion-order tiebreak keeps
// the schedule predictable when passes are independent.
//
// Returns an error on a dependency cycle.
func (g *Graph) Compile(device *wgpu.Device) error {
	order, err := g.topoSort()
	if err != nil {
		return err
	}
	g.executionOrder = order

	clearOps := make(map[ClearKey]struct{}, len(g.passes))
	seen := make(map[ResourceID]struct{}, len(g.Resources.Descriptors))
	for _, passIndex := range order {
		entry := g.passes[passIndex]
		for _, slot := range entry.pass.Writes {
			id := entry.bindings[slot]
			if _, already := seen[id]; already {
				continue
			}
			seen[id] = struct{}{}
			descriptor := g.Resources.Descriptor(id)
			if descriptor.ClearColor != nil || descriptor.ClearDepth != nil {
				clearOps[ClearKey{PassIndex: passIndex, ResourceID: id}] = struct{}{}
			}
		}
	}
	g.clearOps = clearOps

	if err := g.allocateMissingTransients(device); err != nil {
		return err
	}

	g.compiled = true
	return nil
}

// topoSort builds a dependency DAG from the passes' Reads/Writes lists
// and returns the topological execution order (Kahn's algorithm with
// stable insertion-order tiebreak). Errors on a cycle.
func (g *Graph) topoSort() ([]int, error) {
	n := len(g.passes)
	edges := make([][]int, n)
	inDegree := make([]int, n)

	writers := make(map[ResourceID][]int, len(g.Resources.Descriptors))
	readers := make(map[ResourceID][]int, len(g.Resources.Descriptors))
	for i, entry := range g.passes {
		for _, slot := range entry.pass.Reads {
			id := entry.bindings[slot]
			readers[id] = append(readers[id], i)
		}
		for _, slot := range entry.pass.Writes {
			id := entry.bindings[slot]
			writers[id] = append(writers[id], i)
		}
	}

	addEdge := func(from, to int) {
		edges[from] = append(edges[from], to)
		inDegree[to]++
	}

	for _, ws := range writers {
		for k := 1; k < len(ws); k++ {
			addEdge(ws[k-1], ws[k])
		}
	}
	for id, rs := range readers {
		ws, ok := writers[id]
		if !ok || len(ws) == 0 {
			continue
		}
		lastWriter := ws[len(ws)-1]
		for _, r := range rs {
			if r == lastWriter {
				continue
			}
			addEdge(lastWriter, r)
		}
	}

	order := make([]int, 0, n)
	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		sort.Ints(queue)
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		for _, next := range edges[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(order) != n {
		return nil, fmt.Errorf("render: cycle in render graph dependencies")
	}
	return order, nil
}

// allocateMissingTransients creates GPU textures for any transient
// resources whose handle is still empty. Already-allocated transients
// are left alone, so calling [Compile] repeatedly does not churn
// device resources.
func (g *Graph) allocateMissingTransients(device *wgpu.Device) error {
	for index := range g.Resources.Descriptors {
		descriptor := &g.Resources.Descriptors[index]
		switch descriptor.Kind {
		case ResourceKindExternalColor, ResourceKindExternalDepth:
			continue
		case ResourceKindTransientColor, ResourceKindTransientDepth:
			if g.Resources.Handles[index].View != nil {
				continue
			}
			if err := g.createTransient(device, index); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *Graph) createTransient(device *wgpu.Device, index int) error {
	descriptor := &g.Resources.Descriptors[index]
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label: descriptor.Name,
		Size: wgpu.Extent3D{
			Width:              descriptor.Texture.Width,
			Height:             descriptor.Texture.Height,
			DepthOrArrayLayers: 1,
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        descriptor.Texture.Format,
		Usage:         descriptor.Texture.Usage,
	})
	if err != nil {
		return fmt.Errorf("render: create transient texture %q: %w", descriptor.Name, err)
	}
	view, err := texture.CreateView(nil)
	if err != nil {
		texture.Release()
		return fmt.Errorf("render: create view for %q: %w", descriptor.Name, err)
	}
	g.Resources.Handles[index] = TextureHandle{
		Texture: texture,
		View:    view,
		Width:   descriptor.Texture.Width,
		Height:  descriptor.Texture.Height,
		Owned:   true,
	}
	g.Resources.Versions[index]++
	return nil
}

// AllocateTransients (re)allocates every transient texture. Releases
// all owned handles first, so it's the right call after a resize. For
// initial allocation, [Compile] is enough.
func (g *Graph) AllocateTransients(device *wgpu.Device) error {
	g.Resources.ReleaseOwned()
	return g.allocateMissingTransients(device)
}

// ResizeTransients updates the recorded size on every transient
// texture and reallocates them. External resources are untouched.
func (g *Graph) ResizeTransients(device *wgpu.Device, width, height uint32) error {
	for index := range g.Resources.Descriptors {
		descriptor := &g.Resources.Descriptors[index]
		switch descriptor.Kind {
		case ResourceKindTransientColor, ResourceKindTransientDepth:
			descriptor.Texture.Width = width
			descriptor.Texture.Height = height
		}
	}
	return g.AllocateTransients(device)
}

// Execute runs every pass in topological order. Before each pass's
// Prepare, the graph compares the current versions of its read slots
// to the per-pass snapshot it captured last frame; on any mismatch it
// calls [Pass.InvalidateBindGroups] so the pass can drop cached bind
// groups whose backing views are stale. Write-only slots (attachments)
// are not part of bind groups and do not trigger invalidation.
//
// The caller must provide the device, queue, world, and a command
// encoder; the graph records into the encoder but does not finish or
// submit it (the renderer does that).
func (g *Graph) Execute(device *wgpu.Device, queue *wgpu.Queue, world *ecs.World, encoder *wgpu.CommandEncoder) error {
	if !g.compiled {
		return fmt.Errorf("render: graph not compiled")
	}
	for _, passIndex := range g.executionOrder {
		entry := &g.passes[passIndex]
		dirty := false
		for _, slot := range entry.pass.Reads {
			id := entry.bindings[slot]
			if g.Resources.Versions[id] != entry.lastVersions[id] {
				dirty = true
				break
			}
		}
		if dirty && entry.pass.InvalidateBindGroups != nil {
			entry.pass.InvalidateBindGroups(entry.pass.State)
		}
		if dirty {
			for _, slot := range entry.pass.Reads {
				id := entry.bindings[slot]
				entry.lastVersions[id] = g.Resources.Versions[id]
			}
		}

		context := &PassContext{
			Device:    device,
			Queue:     queue,
			Encoder:   encoder,
			World:     world,
			Resources: &g.Resources,
			Slots:     entry.bindings,
			PassIndex: passIndex,
			ClearOps:  g.clearOps,
		}
		if entry.pass.Prepare != nil {
			if err := entry.pass.Prepare(entry.pass.State, context); err != nil {
				return fmt.Errorf("render: pass %q prepare: %w", entry.pass.Name, err)
			}
		}
		if entry.pass.Execute != nil {
			if err := entry.pass.Execute(entry.pass.State, context); err != nil {
				return fmt.Errorf("render: pass %q execute: %w", entry.pass.Name, err)
			}
		}
	}
	return nil
}

// Release frees every owned GPU object: transient textures plus
// per-pass state via [Pass.Release].
func (g *Graph) Release() {
	for _, entry := range g.passes {
		if entry.pass.Release != nil {
			entry.pass.Release(entry.pass.State)
		}
	}
	g.Resources.ReleaseOwned()
}
