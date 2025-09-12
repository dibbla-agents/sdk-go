package maps

import (
	"sync"
)

// SafeFunctionMap is a thread-safe map with generic key and value types.
type SafeFunctionMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewSafeFunctionMap creates a new SafeFunctionMap.
func NewSafeFunctionMap[K comparable, V any]() *SafeFunctionMap[K, V] {
	return &SafeFunctionMap[K, V]{
		m: make(map[K]V),
	}
}

// Store sets the value for a key.
func (sfm *SafeFunctionMap[K, V]) Store(key K, value V) {
	sfm.mu.Lock()
	defer sfm.mu.Unlock()
	sfm.m[key] = value
}

// Load returns the value stored in the map for a key, or nil if no value is present.
func (sfm *SafeFunctionMap[K, V]) Load(key K) (value V, ok bool) {
	sfm.mu.RLock()
	defer sfm.mu.RUnlock()
	value, ok = sfm.m[key]
	return
}

// Delete deletes the value for a key.
func (sfm *SafeFunctionMap[K, V]) Delete(key K) {
	sfm.mu.Lock()
	defer sfm.mu.Unlock()
	delete(sfm.m, key)
}

// Range calls f sequentially for each key and value present in the map.
func (sfm *SafeFunctionMap[K, V]) Range(f func(key K, value V) bool) {
	sfm.mu.RLock()
	defer sfm.mu.RUnlock()
	for k, v := range sfm.m {
		if !f(k, v) {
			break
		}
	}
}

// LoadOrStore returns the existing value for the key if present. Otherwise, it stores and returns the given value.
func (sfm *SafeFunctionMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	sfm.mu.Lock()
	defer sfm.mu.Unlock()
	actual, loaded = sfm.m[key]
	if !loaded {
		sfm.m[key] = value
		actual = value
	}
	return
}

// LoadAndDelete deletes the value for a key, returning the value if present.
func (sfm *SafeFunctionMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	sfm.mu.Lock()
	defer sfm.mu.Unlock()
	value, loaded = sfm.m[key]
	if loaded {
		delete(sfm.m, key)
	}
	return
}

// CompareAndSwap swaps old with new only if the value currently stored for key is equal to old.
func (sfm *SafeFunctionMap[K, V]) CompareAndSwap(key K, old, new V, equal func(a, b V) bool) (swapped bool) {
	sfm.mu.Lock()
	defer sfm.mu.Unlock()
	if current, ok := sfm.m[key]; ok && equal(current, old) {
		sfm.m[key] = new
		return true
	}
	return false
}

// Count returns the number of items in the map.
func (sfm *SafeFunctionMap[K, V]) Count() int {
	sfm.mu.RLock()
	defer sfm.mu.RUnlock()
	return len(sfm.m)
}
