package localcache

import (
	"testing"
	"time"
)

func TestCacheSetGetExpire(t *testing.T) {
	c := New()
	c.Set("k", []byte("v"), 20*time.Millisecond)
	got, ok := c.Get("k")
	if !ok || string(got) != "v" {
		t.Fatalf("Get() = %q, %v; want v, true", got, ok)
	}
	time.Sleep(25 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected expired entry")
	}
}

func TestCacheDeletePrefix(t *testing.T) {
	c := New()
	c.Set("review:v2:store_id:1:offset:0:limit:10", []byte("a"), time.Minute)
	c.Set("review:v2:store_id:1:offset:10:limit:10", []byte("b"), time.Minute)
	c.Set("review:v2:store_id:2:offset:0:limit:10", []byte("c"), time.Minute)

	c.DeletePrefix("review:v2:store_id:1:")
	if _, ok := c.Get("review:v2:store_id:1:offset:0:limit:10"); ok {
		t.Fatal("store 1 key should be deleted")
	}
	if _, ok := c.Get("review:v2:store_id:2:offset:0:limit:10"); !ok {
		t.Fatal("store 2 key should remain")
	}
}
