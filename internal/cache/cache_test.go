package cache

import (
	"testing"
	"time"
)

func TestVelocityCache_GetSet(t *testing.T) {
	c := NewVelocityCache(100*time.Millisecond, 50*time.Millisecond)
	defer c.Stop()

	// Set and get
	c.Set("card_abc", 5)
	val, ok := c.Get("card_abc")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != 5 {
		t.Fatalf("expected 5, got %d", val)
	}

	// Missing key
	_, ok = c.Get("missing")
	if ok {
		t.Fatal("expected missing key to not exist")
	}
}

func TestVelocityCache_TTLExpiration(t *testing.T) {
	c := NewVelocityCache(50*time.Millisecond, 20*time.Millisecond)
	defer c.Stop()

	c.Set("key1", 10)
	// Should exist immediately
	if _, ok := c.Get("key1"); !ok {
		t.Fatal("expected key1 to exist right after set")
	}

	// Wait for TTL + eviction tick
	time.Sleep(150 * time.Millisecond)

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected key1 to expire")
	}
}

func TestVelocityCache_ConcurrentAccess(t *testing.T) {
	c := NewVelocityCache(time.Minute, time.Minute)
	defer c.Stop()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			c.Set("concurrent_key", int64(idx))
			c.Get("concurrent_key")
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
