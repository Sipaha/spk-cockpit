package eventbus_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/eventbus"
)

func TestBus_PublishesToAllSubscribers(t *testing.T) {
	b := eventbus.New(8)
	defer b.Close()

	ch1 := b.Subscribe(8)
	ch2 := b.Subscribe(8)

	b.Publish(api.Event{Type: "x", Data: 1})

	for _, ch := range []chan api.Event{ch1, ch2} {
		select {
		case e := <-ch:
			require.Equal(t, "x", e.Type)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
}

func TestBus_DropsForSlowSubscriber(t *testing.T) {
	b := eventbus.New(8)
	defer b.Close()

	ch := b.Subscribe(1)
	for i := 0; i < 100; i++ {
		b.Publish(api.Event{Type: "x"})
	}
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("bus blocked")
	}
}

func TestBus_UnsubscribeStopsDelivery(t *testing.T) {
	b := eventbus.New(8)
	defer b.Close()

	ch := b.Subscribe(8)
	b.Unsubscribe(ch)
	b.Publish(api.Event{Type: "x"})

	var got int32
	go func() {
		for range ch {
			atomic.AddInt32(&got, 1)
		}
	}()
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&got))
}
