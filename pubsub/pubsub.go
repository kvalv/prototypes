// Package pubsub provides a simple publish/subscribe implementation using channels.
// The publisher is safe for concurrent use.
package pubsub

import (
	"errors"
	"sync"
)

type subscriberCloseFunc func()

var (
	// ErrClosed is returned when a publish is attempted on a closed publisher.
	ErrClosed = errors.New("pubsub: closed")
)

type subscriber[T any] struct {
	c       subscriberCloseFunc
	ev      chan T
	handler func(v T)
	mu      sync.Mutex
	closed  bool
}

// Unsubscribe removes the subscription. No more events will be sent to the handler.
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

// listening returns whether the publisher is listening for events.
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

// New returns a new publisher.
func New[T any]() *publisher[T] {
	return &publisher[T]{}
}

type subscribeOpt[T any] func(*subscriber[T])

// ChannelCapacity sets the number of events that can be buffered before blocking.
func ChannelCapacity[T any](n int) subscribeOpt[T] {
	return func(s *subscriber[T]) {
		s.ev = make(chan T, n)
	}
}

// Subscribe returns a subscription that triggers the handler function when a value is published.
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
		if s.closed {
			return
		}
		s.closed = true
		p.unsubs <- s.ev
	}
	s.c = closer

	go s.listen()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subs = append(p.subs, &s)
	return &s
}

// Close closes the publisher and all subscriptions. If the publisher is already
// closed, this method does nothing.
func (p *publisher[T]) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
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

// Publish publishes a value to all subscribers.
// If the publisher is closed, ErrClosed is returned. This method is safe for
// concurrent use.
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

// SubscriberCount returns the number of active subscribers.
func (p *publisher[T]) SubscriberCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.subs)
}
