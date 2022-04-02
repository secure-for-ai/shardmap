package shardmap

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/zeebo/xxh3"
)

// Map is a hashmap. Like map[string]interface{}, but sharded and thread-safe.
type Map[K comparable, V any] struct {
	init       sync.Once
	cap        int
	shards     int
	shardIDMax uint64
	seed       uint64
	mus        []sync.RWMutex
	maps       []*mapShard[K, V]
	kstr       bool
	ksize      int
}

// New returns a new hashmap with the specified capacity. This function is only
// needed when you must define a minimum capacity, otherwise just use:
//    var m shardmap.Map
func New[K comparable, V any](cap int) *Map[K, V] {
	m := &Map[K, V]{cap: cap}
	m.Init()
	return m
}

func (m *Map[K, V]) detectHasher() {
	// Detect the key type. This is needed by the hasher.
	var k K
	switch ((interface{})(k)).(type) {
	case string:
		m.kstr = true
	default:
		m.ksize = int(unsafe.Sizeof(k))
	}
}

func (m *Map[K, V]) Init() {
	m.init.Do(func() {
		m.detectHasher()
		m.shards = 1
		for m.shards < runtime.NumCPU()*16 {
			m.shards *= 2
		}
		m.shardIDMax = uint64(m.shards - 1)
		scap := m.cap / m.shards
		m.mus = make([]sync.RWMutex, m.shards)
		m.maps = make([]*mapShard[K, V], m.shards)
		for i := 0; i < len(m.maps); i++ {
			m.maps[i] = newShard[K, V](scap)
		}
	})
}

// Clear out all values from map
func (m *Map[K, V]) Clear() {
	for i := 0; i < m.shards; i++ {
		m.mus[i].Lock()
		m.maps[i] = newShard[K, V](m.cap / m.shards)
		m.mus[i].Unlock()
	}
}

// Set assigns a value to a key.
// Returns the previous value, or false when no value was assigned.
func (m *Map[K, V]) Set(key K, value V) (prev V, replaced bool) {
	shard, shardKey := m.choose(key)
	m.mus[shard].Lock()
	prev, replaced = m.maps[shard].SetWithHash(shardKey, key, value)
	m.mus[shard].Unlock()
	return prev, replaced
}

// SetAccept assigns a value to a key. The "accept" function can be used to
// inspect the previous value, if any, and accept or reject the change.
// It's also provides a safe way to block other others from writing to the
// same shard while inspecting.
// Returns the previous value, or false when no value was assigned.
func (m *Map[K, V]) SetAccept(
	key K, value V,
	accept func(prev V, replaced bool) bool,
) (prev V, replaced bool) {
	var result V
	shard, shardKey := m.choose(key)
	m.mus[shard].Lock()
	defer m.mus[shard].Unlock()
	prev, replaced = m.maps[shard].SetWithHash(shardKey, key, value)
	if accept != nil {
		if !accept(prev, replaced) {
			// revert unaccepted change
			if !replaced {
				// delete the newly set data
				m.maps[shard].DeleteWithHash(shardKey, key)
			} else {
				// reset updated data
				m.maps[shard].SetWithHash(shardKey, key, prev)
			}
			return result, false
		}
	}
	return prev, replaced
}

// Get returns a value for a key.
// Returns false when no value has been assign for key.
func (m *Map[K, V]) Get(key K) (value V, ok bool) {
	shard, shardKey := m.choose(key)
	m.mus[shard].RLock()
	value, ok = m.maps[shard].GetWithHash(shardKey, key)
	m.mus[shard].RUnlock()
	return value, ok
}

// Delete deletes a value for a key.
// Returns the deleted value, or false when no value was assigned.
func (m *Map[K, V]) Delete(key K) (prev V, deleted bool) {
	shard, shardKey := m.choose(key)
	m.mus[shard].Lock()
	prev, deleted = m.maps[shard].DeleteWithHash(shardKey, key)
	m.mus[shard].Unlock()
	return prev, deleted
}

// DeleteAccept deletes a value for a key. The "accept" function can be used to
// inspect the previous value, if any, and accept or reject the change.
// It's also provides a safe way to block other others from writing to the
// same shard while inspecting.
// Returns the deleted value, or false when no value was assigned.
func (m *Map[K, V]) DeleteAccept(
	key K,
	accept func(prev interface{}, replaced bool) bool,
) (prev V, deleted bool) {
	var result V
	shard, shardKey := m.choose(key)
	m.mus[shard].Lock()
	defer m.mus[shard].Unlock()
	prev, deleted = m.maps[shard].DeleteWithHash(shardKey, key)
	if accept != nil {
		if !accept(prev, deleted) {
			// revert unaccepted change
			if deleted {
				// reset updated data
				m.maps[shard].SetWithHash(shardKey, key, prev)
			}
			return result, false
		}
	}

	return prev, deleted
}

// Len returns the number of values in map.
func (m *Map[K, V]) Len() int {
	var len int
	for i := 0; i < m.shards; i++ {
		m.mus[i].Lock()
		len += m.maps[i].Len()
		m.mus[i].Unlock()
	}
	return len
}

// Range iterates overall all key/values.
// It's not safe to call or Set or Delete while ranging.
func (m *Map[K, V]) Range(iter func(key K, value V) bool) {
	var done bool
	for i := 0; i < m.shards; i++ {
		func() {
			m.mus[i].RLock()
			defer m.mus[i].RUnlock()
			m.maps[i].Scan(func(key K, value V) bool {
				if !iter(key, value) {
					done = true
					return false
				}
				return true
			})
		}()
		if done {
			break
		}
	}
}

func (m *Map[K, V]) choose(key K) (shard, hashkey uint64) {
	var strKey string
	if m.kstr {
		strKey = *(*string)(unsafe.Pointer(&key))
	} else {
		strKey = *(*string)(unsafe.Pointer(&struct {
			data unsafe.Pointer
			len  int
		}{unsafe.Pointer(&key), m.ksize}))
	}
	gkey := xxh3.HashString(strKey)
	return gkey & m.shardIDMax, makeHash(gkey)
}
