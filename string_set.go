package main

import (
	"sync"
)

func NewStringSet() StringSet {
	return StringSet{m: &sync.Mutex{}, strings: make(map[string]struct{})}
}

func (s *StringSet) Add(items []string) {
	s.m.Lock()
	defer s.m.Unlock()
	for _, item := range items {
		s.strings[item] = struct{}{}
	}
}

func (s *StringSet) HasKey(item string) bool {
	s.m.Lock()
	defer s.m.Unlock()
	_, ok := s.strings[item]
	return ok
}
