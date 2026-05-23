package ecs

type SystemFn func(world *World)

type Schedule struct {
	names   []string
	systems []SystemFn
}

func NewSchedule() *Schedule { return &Schedule{} }

func (s *Schedule) Push(name string, system SystemFn) {
	if s.indexOf(name) >= 0 {
		panic("freecs: schedule already contains system " + name)
	}
	s.names = append(s.names, name)
	s.systems = append(s.systems, system)
}

func (s *Schedule) InsertBefore(target, name string, system SystemFn) {
	pos := s.requireIndex(target)
	if s.indexOf(name) >= 0 {
		panic("freecs: schedule already contains system " + name)
	}
	s.names = append(s.names[:pos], append([]string{name}, s.names[pos:]...)...)
	s.systems = append(s.systems[:pos], append([]SystemFn{system}, s.systems[pos:]...)...)
}

func (s *Schedule) InsertAfter(target, name string, system SystemFn) {
	pos := s.requireIndex(target) + 1
	if s.indexOf(name) >= 0 {
		panic("freecs: schedule already contains system " + name)
	}
	s.names = append(s.names[:pos], append([]string{name}, s.names[pos:]...)...)
	s.systems = append(s.systems[:pos], append([]SystemFn{system}, s.systems[pos:]...)...)
}

func (s *Schedule) Replace(name string, system SystemFn) {
	pos := s.requireIndex(name)
	s.systems[pos] = system
}

func (s *Schedule) Remove(name string) bool {
	pos := s.indexOf(name)
	if pos < 0 {
		return false
	}
	s.names = append(s.names[:pos], s.names[pos+1:]...)
	s.systems = append(s.systems[:pos], s.systems[pos+1:]...)
	return true
}

func (s *Schedule) Contains(name string) bool { return s.indexOf(name) >= 0 }

func (s *Schedule) Names() []string {
	out := make([]string, len(s.names))
	copy(out, s.names)
	return out
}

func (s *Schedule) Len() int { return len(s.systems) }

func (s *Schedule) IsEmpty() bool { return len(s.systems) == 0 }

func (s *Schedule) Run(world *World) {
	for _, system := range s.systems {
		system(world)
	}
}

func SubSchedule(child *Schedule) SystemFn {
	return func(world *World) { child.Run(world) }
}

func (s *Schedule) indexOf(name string) int {
	for i, candidate := range s.names {
		if candidate == name {
			return i
		}
	}
	return -1
}

func (s *Schedule) requireIndex(name string) int {
	pos := s.indexOf(name)
	if pos < 0 {
		panic("freecs: no system named " + name + " in schedule")
	}
	return pos
}
