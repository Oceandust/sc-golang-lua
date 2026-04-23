package collections

import "github.com/emirpasic/gods/sets/linkedhashset"

type OrderedSet[T comparable] struct {
	items *linkedhashset.Set
}

func NewOrderedSet[T comparable]() *OrderedSet[T] {
	return &OrderedSet[T]{items: linkedhashset.New()}
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
	for _, rawValue := range set.items.Values() {
		if !fn(rawValue.(T)) {
			return
		}
	}
}
