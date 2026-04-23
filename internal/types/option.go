package types

import (
	"bytes"
	"encoding/json"

	"github.com/samber/mo"
)

var (
	_ json.Marshaler   = Option[string]{}
	_ json.Unmarshaler = (*Option[string])(nil)
)

type Option[T any] struct {
	inner mo.Option[T]
}

func Some[T any](value T) Option[T] {
	return Option[T]{inner: mo.Some(value)}
}

func None[T any]() Option[T] {
	return Option[T]{inner: mo.None[T]()}
}

func (option Option[T]) Get() (T, bool) {
	return option.inner.Get()
}

func (option Option[T]) IsPresent() bool {
	_, ok := option.Get()
	return ok
}

func (option Option[T]) IsAbsent() bool {
	return !option.IsPresent()
}

func (option Option[T]) OrElse(fallback T) T {
	if value, ok := option.Get(); ok {
		return value
	}

	return fallback
}

func (option Option[T]) IsZero() bool {
	return option.IsAbsent()
}

func (option Option[T]) MarshalJSON() ([]byte, error) {
	value, ok := option.Get()
	if !ok {
		return []byte("null"), nil
	}

	return json.Marshal(value)
}

func (option *Option[T]) UnmarshalJSON(data []byte) error {
	if option == nil {
		return nil
	}

	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*option = None[T]()
		return nil
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	*option = Some(value)
	return nil
}
