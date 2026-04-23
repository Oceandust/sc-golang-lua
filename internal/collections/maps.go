package collections

import (
	"github.com/emirpasic/gods/maps/hashmap"
	"github.com/emirpasic/gods/maps/linkedhashmap"
)

type HashMap[K comparable, V any] struct {
	items *hashmap.Map
}

func NewHashMap[K comparable, V any]() *HashMap[K, V] {
	return &HashMap[K, V]{items: hashmap.New()}
}

func (mapValue *HashMap[K, V]) ensure() {
	if mapValue.items != nil {
		return
	}
	mapValue.items = hashmap.New()
}

func (mapValue *HashMap[K, V]) Put(key K, value V) {
	mapValue.ensure()
	mapValue.items.Put(key, value)
}

func (mapValue *HashMap[K, V]) Get(key K) (V, bool) {
	var zero V
	if mapValue == nil || mapValue.items == nil {
		return zero, false
	}
	raw, ok := mapValue.items.Get(key)
	if !ok {
		return zero, false
	}
	return raw.(V), true
}

func (mapValue *HashMap[K, V]) Delete(key K) {
	if mapValue == nil || mapValue.items == nil {
		return
	}
	mapValue.items.Remove(key)
}

func (mapValue *HashMap[K, V]) Contains(key K) bool {
	_, ok := mapValue.Get(key)
	return ok
}

func (mapValue *HashMap[K, V]) Len() int {
	if mapValue == nil || mapValue.items == nil {
		return 0
	}
	return mapValue.items.Size()
}

func (mapValue *HashMap[K, V]) Keys() []K {
	if mapValue == nil || mapValue.items == nil {
		return nil
	}
	keys := make([]K, 0, mapValue.items.Size())
	for _, rawKey := range mapValue.items.Keys() {
		keys = append(keys, rawKey.(K))
	}
	return keys
}

func (mapValue *HashMap[K, V]) Values() []V {
	if mapValue == nil || mapValue.items == nil {
		return nil
	}
	values := make([]V, 0, mapValue.items.Size())
	for _, rawValue := range mapValue.items.Values() {
		values = append(values, rawValue.(V))
	}
	return values
}

func (mapValue *HashMap[K, V]) Range(fn func(K, V) bool) {
	if mapValue == nil || mapValue.items == nil {
		return
	}
	for _, rawKey := range mapValue.items.Keys() {
		key := rawKey.(K)
		rawValue, _ := mapValue.items.Get(rawKey)
		if !fn(key, rawValue.(V)) {
			return
		}
	}
}

type OrderedMap[K comparable, V any] struct {
	items *linkedhashmap.Map
}

func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{items: linkedhashmap.New()}
}

func (mapValue *OrderedMap[K, V]) ensure() {
	if mapValue.items != nil {
		return
	}
	mapValue.items = linkedhashmap.New()
}

func (mapValue *OrderedMap[K, V]) Put(key K, value V) {
	mapValue.ensure()
	mapValue.items.Put(key, value)
}

func (mapValue *OrderedMap[K, V]) Get(key K) (V, bool) {
	var zero V
	if mapValue == nil || mapValue.items == nil {
		return zero, false
	}
	raw, ok := mapValue.items.Get(key)
	if !ok {
		return zero, false
	}
	return raw.(V), true
}

func (mapValue *OrderedMap[K, V]) Delete(key K) {
	if mapValue == nil || mapValue.items == nil {
		return
	}
	mapValue.items.Remove(key)
}

func (mapValue *OrderedMap[K, V]) Len() int {
	if mapValue == nil || mapValue.items == nil {
		return 0
	}
	return mapValue.items.Size()
}

func (mapValue *OrderedMap[K, V]) Keys() []K {
	if mapValue == nil || mapValue.items == nil {
		return nil
	}
	keys := make([]K, 0, mapValue.items.Size())
	for _, rawKey := range mapValue.items.Keys() {
		keys = append(keys, rawKey.(K))
	}
	return keys
}

func (mapValue *OrderedMap[K, V]) Range(fn func(K, V) bool) {
	if mapValue == nil || mapValue.items == nil {
		return
	}
	for _, rawKey := range mapValue.items.Keys() {
		key := rawKey.(K)
		rawValue, _ := mapValue.items.Get(rawKey)
		if !fn(key, rawValue.(V)) {
			return
		}
	}
}
