package collections

import (
	"github.com/emirpasic/gods/sets/hashset"
	"github.com/emirpasic/gods/sets/linkedhashset"
)

type HashSet[T comparable] struct {
	items *hashset.Set
}

func NewHashSet[T comparable](values ...T) *HashSet[T] {
	set := &HashSet[T]{items: hashset.New()}
	for _, value := range values {
		set.Add(value)
	}
	return set
}

func (set *HashSet[T]) ensure() {
	if set.items != nil {
		return
	}
	set.items = hashset.New()
}

func (set *HashSet[T]) Add(value T) {
	set.ensure()
	set.items.Add(value)
}

func (set *HashSet[T]) Remove(value T) {
	if set == nil || set.items == nil {
		return
	}
	set.items.Remove(value)
}

func (set *HashSet[T]) Clear() {
	if set == nil {
		return
	}
	set.ensure()
	set.items.Clear()
}

func (set *HashSet[T]) Contains(value T) bool {
	if set == nil || set.items == nil {
		return false
	}
	return set.items.Contains(value)
}

func (set *HashSet[T]) Len() int {
	if set == nil || set.items == nil {
		return 0
	}
	return set.items.Size()
}

type OrderedSet[T comparable] struct {
	items *linkedhashset.Set
}

func NewOrderedSet[T comparable](values ...T) *OrderedSet[T] {
	set := &OrderedSet[T]{items: linkedhashset.New()}
	for _, value := range values {
		set.Add(value)
	}
	return set
}

func (set *OrderedSet[T]) ensure() {
	if set.items != nil {
		return
	}
	set.items = linkedhashset.New()
}

func (set *OrderedSet[T]) Add(value T) {
	set.ensure()
	set.items.Add(value)
}

func (set *OrderedSet[T]) Clear() {
	if set == nil {
		return
	}
	set.ensure()
	set.items.Clear()
}

func (set *OrderedSet[T]) Contains(value T) bool {
	if set == nil || set.items == nil {
		return false
	}
	return set.items.Contains(value)
}

func (set *OrderedSet[T]) Len() int {
	if set == nil || set.items == nil {
		return 0
	}
	return set.items.Size()
}

func (set *OrderedSet[T]) Values() []T {
	if set == nil || set.items == nil {
		return nil
	}
	values := make([]T, 0, set.items.Size())
	for _, rawValue := range set.items.Values() {
		values = append(values, rawValue.(T))
	}
	return values
}

func (set *OrderedSet[T]) Range(fn func(T) bool) {
	if set == nil || set.items == nil {
		return
	}
	iterator := set.items.Iterator()
	for iterator.Next() {
		if !fn(iterator.Value().(T)) {
			return
		}
	}
}
