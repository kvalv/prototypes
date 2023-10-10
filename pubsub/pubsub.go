package pubsub

import (
	"errors"
	"sync"
)

type subscriptionCloseFunc func()

var (
	ErrClosed = errors.New("pubsub: closed")
)

type subscriber[T any] struct {
	c       subscriptionCloseFunc
	ev      chan T
	handler func(v T)
	mu      sync.Mutex
}

func (s *subscriber[T]) Unsubscribe() {
	s.c()
}
func (s *subscriber[T]) listen() {
	for e := range s.ev {
		s.handler(e)
	}
}

type publisher[T any] struct {
	subs   []*subscriber[T]
	unsubs chan chan T
	mu     sync.Mutex
	closed bool
}

func (p *publisher[T]) listening() bool { return p.unsubs != nil }

func (p *publisher[T]) listen() {
	for c := range p.unsubs {
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

func New[T any]() *publisher[T] {
	p := &publisher[T]{}
	return p
}

type subscribeOpt[T any] func(*subscriber[T])

func ChannelCapacity[T any](n int) subscribeOpt[T] {
	return func(s *subscriber[T]) {
		s.ev = make(chan T, n)
	}
}

func (p *publisher[T]) Subscribe(handler func(v T), opts ...subscribeOpt[T]) *subscriber[T] {
	ev := make(chan T)
	s := subscriber[T]{
		handler: handler,
		ev:      ev,
	}
	for _, opt := range opts {
		opt(&s)
	}
	if !p.listening() {
		// allocate the channel now because we have at least one subscriber
		p.unsubs = make(chan chan T, 5)
		go p.listen()
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
	if p.unsubs != nil {
		close(p.unsubs)
	}
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
func (p *publisher[T]) SubscriptionCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.subs)
}
