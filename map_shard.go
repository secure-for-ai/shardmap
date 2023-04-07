// Copyright 2019 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an ISC-style
// license that can be found in the LICENSE file.

package shardmap

const (
	loadFactor  = 0.85                      // must be above 50%
	dibBitSize  = 16                        // 0xFFFF
	hashBitSize = 64 - dibBitSize           // 0xFFFFFFFFFFFF
	maxHash     = ^uint64(0) >> dibBitSize  // max 28,147,497,671,0655
	maxDIB      = ^uint64(0) >> hashBitSize // max 65,535
	hashMask    = ^maxDIB                   // 0xFFFF FFFF FFFF 0000
)

type entry[K comparable, V any] struct {
	hdib  uint64 // bitfield { hash:48 dib:16 }
	key   K      // user key
	value V      // user value
}

func (e *entry[K, V]) dib() uint64 {
	return e.hdib & maxDIB
}
func (e *entry[K, V]) hash() uint64 {
	return e.hdib >> dibBitSize
}
func (e *entry[K, V]) setDIB(dib uint64) {
	e.hdib = e.hdib&hashMask | dib&maxDIB
}
func (e *entry[K, V]) setHash(hash uint64) {
	e.hdib = hash<<dibBitSize | e.hdib&maxDIB
}
func makeHDIB(hash, dib uint64) uint64 {
	return hash<<dibBitSize | dib&maxDIB
}
func makeHash(hash uint64) uint64 {
	return hash >> dibBitSize
}

// hash returns a 48-bit hash for 64-bit environments, or 32-bit hash for
// 32-bit environments.
// func (m *mapShard[K, V]) hash(key K) uint64 {
// 	// The unsafe package is used here to cast the key into a string container
// 	// so that the hasher can work. The hasher normally only accept a string or
// 	// []byte, but this effectively allows it to accept value type.
// 	// The m.kstr bool, which is set from the New function, indicates that the
// 	// key is known to already be a true string. Otherwise, a fake string is
// 	// derived by setting the string data to value of the key, and the string
// 	// length to the size of the value.
// 	var strKey string
// 	if m.kstr {
// 		strKey = *(*string)(unsafe.Pointer(&key))
// 		// strKey = *(*[]byte)(unsafe.Pointer(&sliceHeader{unsafe.Pointer(&str), len(str)}))
// 	} else {
// 		strKey = *(*string)(unsafe.Pointer(&struct {
// 			data unsafe.Pointer
// 			len  int
// 		}{unsafe.Pointer(&key), m.ksize}))
// 	}
// 	// Now for the actually hashing.
// 	// return xxhash.Sum64String(strKey) >> dibBitSize
// 	return xxh3.HashString(strKey) >> dibBitSize
// }

// Map is a hashmap. Like map[string]interface{}
type mapShard[K comparable, V any] struct {
	cap      int
	length   int
	growAt   int
	shrinkAt int
	mask     uint64
	buckets  []entry[K, V]
}

// New returns a new Map. Like map[string]interface{}
func newShard[K comparable, V any](cap int) *mapShard[K, V] {
	m := new(mapShard[K, V])
	m.cap = cap
	sz := 8
	for sz < m.cap {
		sz *= 2
	}
	m.buckets = make([]entry[K, V], sz)
	m.mask = uint64(len(m.buckets) - 1)
	m.growAt = int(float64(len(m.buckets)) * loadFactor)
	m.shrinkAt = int(float64(len(m.buckets)) * (1 - loadFactor))
	return m
}

func (m *mapShard[K, V]) resize(newCap int) {
	nmap := newShard[K, V](newCap)
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			nmap.set(m.buckets[i].hash(), m.buckets[i].key, m.buckets[i].value)
		}
	}
	cap := m.cap
	*m = *nmap
	m.cap = cap
}

// Set assigns a value to a key.
// Returns the previous value, or false when no value was assigned.
func (m *mapShard[K, V]) SetWithHash(hash uint64, key K, value V) (V, bool) {
	if len(m.buckets) == 0 {
		*m = *newShard[K, V](0)
	}
	if m.length >= m.growAt {
		m.resize(len(m.buckets) * 2)
	}
	return m.set(hash, key, value)
}

func (m *mapShard[K, V]) set(hash uint64, key K, value V) (prev V, ok bool) {
	e := entry[K, V]{hdib: makeHDIB(hash, 1), value: value, key: key}
	i := e.hash() & m.mask
	for {
		if m.buckets[i].dib() == 0 {
			m.buckets[i] = e
			m.length++
			return prev, false
		}
		if e.hash() == m.buckets[i].hash() && e.key == m.buckets[i].key {
			prev = m.buckets[i].value
			m.buckets[i].value = e.value
			return prev, true
		}
		if m.buckets[i].dib() < e.dib() {
			e, m.buckets[i] = m.buckets[i], e
		}
		i = (i + 1) & m.mask
		e.setDIB(e.dib() + 1)
	}
}

func (m *mapShard[K, V]) GetWithHash(hash uint64, key K) (value V, ok bool) {
	if len(m.buckets) == 0 {
		return value, false
	}

	i := hash & m.mask
	for {
		if m.buckets[i].dib() == 0 {
			return value, false
		}
		if m.buckets[i].hash() == hash && m.buckets[i].key == key {
			return m.buckets[i].value, true
		}
		i = (i + 1) & m.mask
	}
}

// Len returns the number of values in map.
func (m *mapShard[K, V]) Len() int {
	return m.length
}

// Delete deletes a value for a key.
// Returns the deleted value, or false when no value was assigned.
func (m *mapShard[K, V]) DeleteWithHash(hash uint64, key K) (prev V, deleted bool) {
	if len(m.buckets) == 0 {
		return prev, false
	}

	i := hash & m.mask
	for {
		if m.buckets[i].dib() == 0 {
			return prev, false
		}
		if m.buckets[i].hash() == hash && m.buckets[i].key == key {
			prev = m.buckets[i].value
			m.remove(i)
			return prev, true
		}
		i = (i + 1) & m.mask
	}
}

func (m *mapShard[K, V]) remove(i uint64) {
	m.buckets[i].setDIB(0)
	for {
		pi := i
		i = (i + 1) & m.mask
		if m.buckets[i].dib() <= 1 {
			m.buckets[pi] = entry[K, V]{}
			break
		}
		m.buckets[pi] = m.buckets[i]
		m.buckets[pi].setDIB(m.buckets[pi].dib() - 1)
	}
	m.length--
	if len(m.buckets) > m.cap && m.length <= m.shrinkAt {
		m.resize(m.length)
	}
}

// Scan iterates over all key/values.
// It's not safe to call or Set or Delete while scanning.
func (m *mapShard[K, V]) Scan(iter func(key K, value V) bool) {
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			if !iter(m.buckets[i].key, m.buckets[i].value) {
				return
			}
		}
	}
}

// Keys returns all keys as a slice
func (m *mapShard[K, V]) Keys() []K {
	keys := make([]K, 0, m.length)
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			keys = append(keys, m.buckets[i].key)
		}
	}
	return keys
}

// Values returns all values as a slice
func (m *mapShard[K, V]) Values() []V {
	values := make([]V, 0, m.length)
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			values = append(values, m.buckets[i].value)
		}
	}
	return values
}
