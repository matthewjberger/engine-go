package ecs

type Archetype struct {
	Mask     Mask
	Entities []Entity
	columns  [maxComponents]*column
}

func newArchetype(mask Mask, reg *registry) *Archetype {
	table := &Archetype{Mask: mask}
	for bit := uint8(0); bit < maxComponents; bit++ {
		if mask&(Mask(1)<<bit) == 0 {
			continue
		}
		info := reg.infoForBit(bit)
		if info == nil {
			panic("freecs: archetype mask references unregistered bit")
		}
		table.columns[bit] = newColumn(info.elemType)
	}
	return table
}

type tableEdges struct {
	add    [maxComponents]int32
	remove [maxComponents]int32
}

func newTableEdges() *tableEdges {
	edges := &tableEdges{}
	for i := 0; i < maxComponents; i++ {
		edges.add[i] = -1
		edges.remove[i] = -1
	}
	return edges
}
