package stringset

import (
	"context"
	"fmt"
	"sync"
)

type Set struct {
	m       *sync.Mutex
	strings map[string]struct{}
	c       chan string
	ctx     context.Context
	done    bool
}

func New() Set {
	return Set{m: &sync.Mutex{}, strings: make(map[string]struct{}), ctx: context.TODO()}
}

func (s *Set) Add(items []string) error {
	s.m.Lock()
	defer s.m.Unlock()
	for _, item := range items {
		if s.c != nil {
			select {
			case s.c <- item:
			case <-s.ctx.Done():
				s.reset()
				return s.ctx.Err()
			}
		} else {
			s.strings[item] = struct{}{}
		}
	}

	return nil
}

func (s *Set) HasKey(item string) bool {
	s.m.Lock()
	defer s.m.Unlock()
	_, ok := s.strings[item]
	return ok
}

func (s *Set) Reset() {
	s.m.Lock()
	defer s.m.Unlock()

	s.done = true
	s.reset()
}

func (s *Set) reset() {
	if s.c != nil {
		close(s.c)
		s.c = nil
		s.ctx = context.TODO()
		s.done = false
	}
}

func (s *Set) Drain(ctx context.Context) <-chan string {
	c := make(chan string, 0)
	s.c = c
	s.ctx = ctx

	go func() {
		s.m.Lock()
		defer s.m.Unlock()

		for k := range s.strings {
			select {
			case c <- k:
				delete(s.strings, k)
			case <-s.ctx.Done():
				fmt.Println("cancel drain")
				s.reset()
				break
			}
		}

		if s.done {
			s.reset()
		}
	}()

	return c
}
