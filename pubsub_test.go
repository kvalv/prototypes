package pubsubdemo

import (
	"testing"
	"time"

	"github.com/kvalv/pubsub-demo/pubsub"
)

func TestPubsub(t *testing.T) {
	t.Run("two listeners", func(t *testing.T) {
		pub := pubsub.NewPublisher[int]()
		var got1, got2 int
		pub.Subscribe(func(v int) {
			t.Logf("got1: %d", v)
			got1 = v
		})
		pub.Subscribe(func(v int) {
			got2 = v
		})
		if got1 != 0 || got2 != 0 {
			t.Errorf("got1, got2 = %d, %d; want 0, 0", got1, got2)
		}
		pub.Publish(1)
		time.Sleep(10 * time.Millisecond)
		if got1 != 1 || got2 != 1 {
			t.Errorf("got1, got2 = %d, %d; want 1, 1", got1, got2)
		}
	})

	t.Run("struct pointer object", func(t *testing.T) {
		type mystruct struct {
			a int
		}
		pub := pubsub.NewPublisher[*mystruct]()
		pub.Subscribe(func(v *mystruct) {
			v.a = 1
		})
		m := &mystruct{}
		pub.Publish(m)
		time.Sleep(10 * time.Millisecond)
		if m.a != 1 {
			t.Errorf("m.a = %d; want 1", m.a)
		}
	})
}
