package maps

import (
	"sync"
)

// Type-safe wrapper around sync.Map for a map with string keys and a generic value.
type SafeMap[K comparable, V any] struct {
	m sync.Map
}

// Store sets the value for a key.
func (sm *SafeMap[K, V]) Store(key K, value V) {
	sm.m.Store(key, value)
}

// Load returns the value stored in the map for a key, or nil if no value is present.
func (sm *SafeMap[K, V]) Load(key K) (value V, ok bool) {
	val, ok := sm.m.Load(key)
	if ok {
		return val.(V), ok
	}
	return
}

// Delete deletes the value for a key.
func (sm *SafeMap[K, V]) Delete(key K) {
	sm.m.Delete(key)
}

// Range calls f sequentially for each key and value present in the map.
func (sm *SafeMap[K, V]) Range(f func(key K, value V) bool) {
	sm.m.Range(func(k, v interface{}) bool {
		return f(k.(K), v.(V))
	})
}

// LoadOrStore returns the existing value for the key if present. Otherwise, it stores and returns the given value.
func (sm *SafeMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	actualInterface, loaded := sm.m.LoadOrStore(key, value)
	if loaded {
		return actualInterface.(V), loaded
	}
	return value, loaded
}

// LoadAndDelete deletes the value for a key, returning the value if present.
func (sm *SafeMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	val, loaded := sm.m.LoadAndDelete(key)
	if loaded {
		return val.(V), loaded
	}
	return
}

// CompareAndSwap swaps old with new only if the value currently stored for key is equal to old.
func (sm *SafeMap[K, V]) CompareAndSwap(key K, old, new V) (swapped bool) {
	return sm.m.CompareAndSwap(key, old, new)
}

func (sm *SafeMap[K, V]) Count() int {
	count := 0
	sm.Range(func(key K, value V) bool {
		count++
		return true
	})
	return count
}
