package luavalue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"defgraph/internal/collections"
)

type Kind string

const (
	KindNull   Kind = "null"
	KindBool   Kind = "bool"
	KindInt    Kind = "int"
	KindFloat  Kind = "float"
	KindString Kind = "string"
	KindArray  Kind = "array"
	KindObject Kind = "object"
)

type Value struct {
	kind        Kind
	boolValue   bool
	intValue    int64
	floatValue  float64
	stringValue string
	arrayValue  []Value
	objectValue *Object
}

type Object struct {
	items *collections.OrderedMap[string, Value]
}

func Null() Value {
	return Value{kind: KindNull}
}

func Bool(value bool) Value {
	return Value{kind: KindBool, boolValue: value}
}

func Int(value int64) Value {
	return Value{kind: KindInt, intValue: value}
}

func Float(value float64) Value {
	return Value{kind: KindFloat, floatValue: value}
}

func String(value string) Value {
	return Value{kind: KindString, stringValue: value}
}

func Array(values ...Value) Value {
	cloned := make([]Value, len(values))
	for index := range values {
		cloned[index] = values[index].Clone()
	}

	return Value{kind: KindArray, arrayValue: cloned}
}

func ObjectValue(object Object) Value {
	cloned := object.Clone()
	return Value{kind: KindObject, objectValue: &cloned}
}

func NewObject() Object {
	return Object{items: collections.NewOrderedMap[string, Value]()}
}

func FromAny(raw any) (Value, error) {
	switch typed := raw.(type) {
	case nil:
		return Null(), nil
	case bool:
		return Bool(typed), nil
	case int:
		return Int(int64(typed)), nil
	case int64:
		return Int(typed), nil
	case float64:
		if math.Trunc(typed) == typed {
			return Int(int64(typed)), nil
		}
		return Float(typed), nil
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return Int(integer), nil
		}

		floatValue, err := typed.Float64()
		if err != nil {
			return Null(), err
		}

		return Float(floatValue), nil
	case string:
		return String(typed), nil
	case []any:
		values := make([]Value, 0, len(typed))
		for _, item := range typed {
			value, err := FromAny(item)
			if err != nil {
				return Null(), err
			}
			values = append(values, value)
		}
		return Array(values...), nil
	case map[string]any:
		object := NewObject()
		for key, item := range typed {
			value, err := FromAny(item)
			if err != nil {
				return Null(), err
			}
			object.Set(key, value)
		}
		return ObjectValue(object), nil
	default:
		return String(fmt.Sprintf("%v", raw)), nil
	}
}

func (value Value) Kind() Kind {
	if value.kind == "" {
		return KindNull
	}

	return value.kind
}

func (value Value) IsNull() bool {
	return value.Kind() == KindNull
}

func (value Value) AsBool() (bool, bool) {
	if value.Kind() != KindBool {
		return false, false
	}

	return value.boolValue, true
}

func (value Value) AsString() (string, bool) {
	switch value.Kind() {
	case KindNull, KindBool, KindArray, KindObject:
		return "", false
	case KindString:
		return value.stringValue, true
	case KindInt:
		return strconv.FormatInt(value.intValue, 10), true
	case KindFloat:
		return strconv.FormatFloat(value.floatValue, 'f', -1, 64), true
	}

	return "", false
}

func (value Value) AsInt64() (int64, bool) {
	switch value.Kind() {
	case KindNull, KindBool, KindString, KindArray, KindObject:
		return 0, false
	case KindInt:
		return value.intValue, true
	case KindFloat:
		if math.Trunc(value.floatValue) == value.floatValue {
			return int64(value.floatValue), true
		}
		return 0, false
	}

	return 0, false
}

func (value Value) AsFloat64() (float64, bool) {
	switch value.Kind() {
	case KindNull, KindBool, KindString, KindArray, KindObject:
		return 0, false
	case KindInt:
		return float64(value.intValue), true
	case KindFloat:
		return value.floatValue, true
	}

	return 0, false
}

func (value Value) AsArray() ([]Value, bool) {
	if value.Kind() != KindArray {
		return nil, false
	}

	items := make([]Value, len(value.arrayValue))
	for index := range value.arrayValue {
		items[index] = value.arrayValue[index].Clone()
	}

	return items, true
}

func (value Value) AsObject() (Object, bool) {
	if value.Kind() != KindObject || value.objectValue == nil {
		return Object{}, false
	}

	return value.objectValue.Clone(), true
}

func (value Value) Clone() Value {
	switch value.Kind() {
	case KindNull, KindBool, KindInt, KindFloat, KindString:
		return value
	case KindArray:
		items := make([]Value, len(value.arrayValue))
		for index := range value.arrayValue {
			items[index] = value.arrayValue[index].Clone()
		}
		return Value{kind: KindArray, arrayValue: items}
	case KindObject:
		if value.objectValue == nil {
			return Value{kind: KindObject, objectValue: &Object{items: collections.NewOrderedMap[string, Value]()}}
		}
		cloned := value.objectValue.Clone()
		return Value{kind: KindObject, objectValue: &cloned}
	}

	return value
}

func (value Value) MarshalJSON() ([]byte, error) {
	switch value.Kind() {
	case KindNull:
		return []byte("null"), nil
	case KindBool:
		return json.Marshal(value.boolValue)
	case KindInt:
		return json.Marshal(value.intValue)
	case KindFloat:
		return json.Marshal(value.floatValue)
	case KindString:
		return json.Marshal(value.stringValue)
	case KindArray:
		return json.Marshal(value.arrayValue)
	case KindObject:
		if value.objectValue == nil {
			return []byte("{}"), nil
		}
		return value.objectValue.MarshalJSON()
	default:
		return nil, fmt.Errorf("unsupported luavalue kind %q", value.kind)
	}
}

func (value *Value) UnmarshalJSON(data []byte) error {
	if value == nil {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	converted, err := decodeJSONValue(decoder)
	if err != nil {
		return err
	}

	*value = converted
	return nil
}

func (object Object) Len() int {
	if object.items == nil {
		return 0
	}

	return object.items.Len()
}

func (object Object) IsZero() bool {
	return object.Len() == 0
}

func (object Object) Keys() []string {
	if object.items == nil {
		return nil
	}

	return object.items.Keys()
}

func (object *Object) Set(key string, value Value) {
	object.ensureMap()
	object.items.Put(key, value.Clone())
}

func (object Object) Get(key string) (Value, bool) {
	if object.items == nil {
		return Null(), false
	}

	raw, ok := object.items.Get(key)
	if !ok {
		return Null(), false
	}

	return raw.Clone(), true
}

func (object Object) MustGet(key string) Value {
	if value, ok := object.Get(key); ok {
		return value
	}

	return Null()
}

func (object *Object) Delete(key string) {
	if object.items == nil {
		return
	}

	object.items.Delete(key)
}

func (object Object) Range(fn func(key string, value Value) bool) {
	if object.items == nil {
		return
	}

	object.items.Range(func(key string, value Value) bool {
		return fn(key, value.Clone())
	})
}

func (object Object) Clone() Object {
	cloned := NewObject()
	object.Range(func(key string, value Value) bool {
		cloned.Set(key, value.Clone())
		return true
	})
	return cloned
}

func (object Object) MarshalJSON() ([]byte, error) {
	if object.items == nil || object.items.Len() == 0 {
		return []byte("{}"), nil
	}

	var buffer bytes.Buffer
	buffer.WriteByte('{')

	first := true
	var marshalErr error
	object.Range(func(key string, value Value) bool {
		if !first {
			buffer.WriteByte(',')
		}
		first = false

		keyBytes, err := json.Marshal(key)
		if err != nil {
			marshalErr = err
			return false
		}

		valueBytes, err := json.Marshal(value)
		if err != nil {
			marshalErr = err
			return false
		}

		buffer.Write(keyBytes)
		buffer.WriteByte(':')
		buffer.Write(valueBytes)
		return true
	})

	if marshalErr != nil {
		return nil, marshalErr
	}

	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

func (object *Object) UnmarshalJSON(data []byte) error {
	if object == nil {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	value, err := decodeJSONValue(decoder)
	if err != nil {
		return err
	}

	result, ok := value.AsObject()
	if !ok {
		return fmt.Errorf("expected JSON object, got %s", value.Kind())
	}

	*object = result
	return nil
}

func (object *Object) ensureMap() {
	if object.items != nil {
		return
	}

	object.items = collections.NewOrderedMap[string, Value]()
}

func decodeJSONValue(decoder *json.Decoder) (Value, error) {
	token, err := decoder.Token()
	if err != nil {
		return Null(), err
	}

	switch typed := token.(type) {
	case nil:
		return Null(), nil
	case bool:
		return Bool(typed), nil
	case string:
		return String(typed), nil
	case json.Number:
		if integer, intErr := typed.Int64(); intErr == nil {
			return Int(integer), nil
		}

		floatValue, floatErr := typed.Float64()
		if floatErr != nil {
			return Null(), floatErr
		}

		return Float(floatValue), nil
	case json.Delim:
		switch typed {
		case '[':
			values := make([]Value, 0)
			for decoder.More() {
				value, decodeErr := decodeJSONValue(decoder)
				if decodeErr != nil {
					return Null(), decodeErr
				}
				values = append(values, value)
			}
			if _, closeErr := decoder.Token(); closeErr != nil {
				return Null(), closeErr
			}
			return Array(values...), nil
		case '{':
			object := NewObject()
			for decoder.More() {
				keyToken, keyErr := decoder.Token()
				if keyErr != nil {
					return Null(), keyErr
				}
				key, ok := keyToken.(string)
				if !ok {
					return Null(), fmt.Errorf("expected JSON object key, got %T", keyToken)
				}
				value, valueErr := decodeJSONValue(decoder)
				if valueErr != nil {
					return Null(), valueErr
				}
				object.Set(key, value)
			}
			if _, closeErr := decoder.Token(); closeErr != nil {
				return Null(), closeErr
			}
			return ObjectValue(object), nil
		default:
			return Null(), fmt.Errorf("unexpected JSON delimiter %q", typed)
		}
	default:
		return Null(), fmt.Errorf("unsupported JSON token %T", token)
	}
}
