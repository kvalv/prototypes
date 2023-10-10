package pubsub

import (
	"sync"
)

type Publisher[T any] interface {
	Publish(v T)
	Subscribe(func(v T)) Subscriber[T]
	Close()
}

type Subscriber[T any] interface {
	Unsubscribe()
}

type subscription[T any] struct {
	done    chan struct{}
	ev      chan T
	handler func(v T)
}

func (s *subscription[T]) Unsubscribe() {
	if s.done == nil {
		return
	}
	s.done <- struct{}{}
}
func (s *subscription[T]) listen() {
	for e := range s.ev {
		s.handler(e)
	}
}

type publisher[T any] struct {
	subs   []*subscription[T]
	unsubs chan chan struct{}
	mu     sync.Mutex
}

func (p *publisher[T]) listen() {
	for {
		select {
		case c := <-p.unsubs:
			p.mu.Lock()
			for i, sub := range p.subs {
				if sub.done == c {
					p.subs = append(p.subs[:i], p.subs[i+1:]...)
					break
				}
			}
			p.mu.Unlock()
		}
	}
}

func NewPublisher[T any]() Publisher[T] {
	p := &publisher[T]{}
	go p.listen()
	return p
}

func (p *publisher[T]) Subscribe(handler func(v T)) Subscriber[T] {
	done := make(chan struct{})
	s := subscription[T]{
		done:    done,
		handler: handler,
		ev:      make(chan T),
	}
	go s.listen()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subs = append(p.subs, &s)
	return &s
}
func (p *publisher[T]) Close() {
	for _, sub := range p.subs {
		sub.Unsubscribe()
	}
}
func (p *publisher[T]) Publish(v T) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, sub := range p.subs {
		sub.ev <- v
	}
}
