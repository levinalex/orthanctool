package stringset

import (
	"sync"
)

type Set struct {
	m       *sync.Mutex
	strings map[string]struct{}
}

func New() Set {
	return Set{m: &sync.Mutex{}, strings: make(map[string]struct{})}
}

func (s *Set) Add(items []string) {
	s.m.Lock()
	defer s.m.Unlock()
	for _, item := range items {
		s.strings[item] = struct{}{}
	}
}

func (s *Set) HasKey(item string) bool {
	s.m.Lock()
	defer s.m.Unlock()
	_, ok := s.strings[item]
	return ok
}

func (s *Set) List() []string {
	s.m.Lock()
	defer s.m.Unlock()

	res := make([]string, len(s.strings))
	i := 0
	for k, _ := range s.strings {
		res[i] = k
		i++
	}
	return res
}
