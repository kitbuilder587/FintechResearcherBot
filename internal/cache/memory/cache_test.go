package memory

import (
	"context"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	cache := New()
	defer cache.Stop()

	key := "test-key"
	value := "test-value"
	ttl := 5 * time.Second

	cache.Set(key, value, ttl)

	got, ok := cache.Get(key)
	if !ok {
		t.Error("Get() should return ok=true for existing key")
	}
	if got != value {
		t.Errorf("Get() = %v, want %v", got, value)
	}
}

func TestCache_GetNonExistent(t *testing.T) {
	cache := New()
	defer cache.Stop()

	got, ok := cache.Get("non-existent")
	if ok {
		t.Error("Get() should return ok=false for non-existent key")
	}
	if got != nil {
		t.Errorf("Get() = %v, want nil", got)
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	cache := New()
	defer cache.Stop()

	key := "expiring-key"
	value := "expiring-value"
	ttl := 50 * time.Millisecond

	cache.Set(key, value, ttl)

	if _, ok := cache.Get(key); !ok {
		t.Error("Key should exist before TTL expiration")
	}

	time.Sleep(100 * time.Millisecond)

	if _, ok := cache.Get(key); ok {
		t.Error("Key should be expired after TTL")
	}
}

func TestCache_Delete(t *testing.T) {
	cache := New()
	defer cache.Stop()

	key := "delete-key"
	value := "delete-value"

	cache.Set(key, value, time.Hour)

	if _, ok := cache.Get(key); !ok {
		t.Error("Key should exist before delete")
	}

	cache.Delete(key)

	if _, ok := cache.Get(key); ok {
		t.Error("Key should not exist after delete")
	}
}

func TestCache_Overwrite(t *testing.T) {
	cache := New()
	defer cache.Stop()

	key := "overwrite-key"
	value1 := "value1"
	value2 := "value2"

	cache.Set(key, value1, time.Hour)
	cache.Set(key, value2, time.Hour)

	got, _ := cache.Get(key)
	if got != value2 {
		t.Errorf("Get() = %v, want %v after overwrite", got, value2)
	}
}

func TestCache_Stop(t *testing.T) {
	cache := New()

	cache.Stop()

	cache.Stop()
}

func TestCache_NewWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cache := NewWithContext(ctx)

	key := "ctx-key"
	value := "ctx-value"

	cache.Set(key, value, time.Hour)

	if got, ok := cache.Get(key); !ok || got != value {
		t.Error("Cache should work before context cancel")
	}

	cancel()

	time.Sleep(10 * time.Millisecond)

	cache.Set("another", "value", time.Hour)
	if _, ok := cache.Get("another"); !ok {
		t.Error("Cache should still work after context cancel")
	}
}

func TestCache_DifferentValueTypes(t *testing.T) {
	cache := New()
	defer cache.Stop()

	cache.Set("string", "value", time.Hour)
	if got, _ := cache.Get("string"); got != "value" {
		t.Error("String value mismatch")
	}

	cache.Set("int", 42, time.Hour)
	if got, _ := cache.Get("int"); got != 42 {
		t.Error("Int value mismatch")
	}

	type TestStruct struct {
		Name string
		Age  int
	}
	cache.Set("struct", TestStruct{Name: "test", Age: 25}, time.Hour)
	if got, _ := cache.Get("struct"); got != (TestStruct{Name: "test", Age: 25}) {
		t.Error("Struct value mismatch")
	}

	cache.Set("slice", []int{1, 2, 3}, time.Hour)
	if got, _ := cache.Get("slice"); len(got.([]int)) != 3 {
		t.Error("Slice value mismatch")
	}
}

func TestCache_Concurrent(t *testing.T) {
	cache := New()
	defer cache.Stop()

	done := make(chan bool)

	go func() {
		for i := 0; i < 1000; i++ {
			cache.Set("concurrent-key", i, time.Hour)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			cache.Get("concurrent-key")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			cache.Delete("concurrent-key")
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	<-done
	<-done
	<-done
}
