package events

import (
	"testing"
	"time"
)

func TestPublishAndDropSlowSubscriber(t *testing.T) {
	b := NewBus()
	ch, unsubscribe, err := b.Subscribe(1)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	if dropped := b.Publish(Event{Type: "starting"}); dropped != 0 {
		t.Fatalf("dropped=%d", dropped)
	}
	if dropped := b.Publish(Event{Type: "healthy"}); dropped != 1 {
		t.Fatalf("dropped=%d", dropped)
	}
	got := <-ch
	if got.Type != "starting" || got.At.IsZero() {
		t.Fatalf("event=%+v", got)
	}
}

func TestCloseAndSubscribeValidation(t *testing.T) {
	b := NewBus()
	if _, _, err := b.Subscribe(0); err == nil {
		t.Fatal("expected invalid buffer error")
	}
	b.Close()
	if _, _, err := b.Subscribe(1); err == nil {
		t.Fatal("expected closed bus error")
	}
	if b.Publish(Event{Type: "ignored"}) != 0 {
		t.Fatal("closed bus should not publish")
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	b := NewBus()
	ch, unsubscribe, err := b.Subscribe(1)
	if err != nil {
		t.Fatal(err)
	}
	unsubscribe()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel")
		}
	case <-time.After(time.Second):
		t.Fatal("channel did not close")
	}
	unsubscribe()
}
