package pubsub

import (
	"errors"
	"sync"
)

type subscriptionCloseFunc func()

var (
	ErrClosed = errors.New("pubsub: closed")
)

type subscription[T any] struct {
	c       subscriptionCloseFunc
	ev      chan T
	handler func(v T)
	mu      sync.Mutex
}

func (s *subscription[T]) Unsubscribe() {
	s.c()
}
func (s *subscription[T]) listen() {
	for e := range s.ev {
		s.handler(e)
	}
}

type publisher[T any] struct {
	subs   []*subscription[T]
	unsubs chan chan T
	mu     sync.Mutex
	closed bool
}

func (p *publisher[T]) listen() {
	for {
		select {
		case c := <-p.unsubs:
			p.mu.Lock()
			for i, sub := range p.subs {
				if sub.ev == c {
					p.subs = append(p.subs[:i], p.subs[i+1:]...)
					break
				}
			}
			p.mu.Unlock()
		}
	}
}

func NewPublisher[T any]() *publisher[T] {
	p := &publisher[T]{
		unsubs: make(chan chan T, 5),
	}
	go p.listen()
	return p
}

type subscribeOpt[T any] func(*subscription[T])

func ChannelCapacity[T any](n int) subscribeOpt[T] {
	return func(s *subscription[T]) {
		s.ev = make(chan T, n)
	}
}

func (p *publisher[T]) Subscribe(handler func(v T), opts ...subscribeOpt[T]) *subscription[T] {
	ev := make(chan T)
	s := subscription[T]{
		handler: handler,
		ev:      ev,
	}
	for _, opt := range opts {
		opt(&s)
	}
	closer := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.ev == nil {
			return
		}
		p.unsubs <- s.ev
	}
	s.c = closer

	go s.listen()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subs = append(p.subs, &s)
	return &s
}
func (p *publisher[T]) Close() {
	if p.subs == nil {
		return
	}
	for _, sub := range p.subs {
		sub.Unsubscribe()
	}
	p.subs = nil
	close(p.unsubs)
	p.closed = true
}
func (p *publisher[T]) Publish(v T) error {
	if p.closed {
		return ErrClosed
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, sub := range p.subs {
		sub.ev <- v
	}
	return nil
}
