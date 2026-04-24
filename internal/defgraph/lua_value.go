package defgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"sc_cli/internal/collections"
)

var (
	_ json.Marshaler   = LuaValue{}
	_ json.Unmarshaler = (*LuaValue)(nil)
	_ json.Marshaler   = LuaObject{}
	_ json.Unmarshaler = (*LuaObject)(nil)
)

type LuaKind string

const (
	LuaKindNull   LuaKind = "null"
	LuaKindBool   LuaKind = "bool"
	LuaKindInt    LuaKind = "int"
	LuaKindFloat  LuaKind = "float"
	LuaKindString LuaKind = "string"
	LuaKindArray  LuaKind = "array"
	LuaKindObject LuaKind = "object"
)

type LuaValue struct {
	kind        LuaKind
	boolValue   bool
	intValue    int64
	floatValue  float64
	stringValue string
	arrayValue  []LuaValue
	objectValue *LuaObject
}

type LuaObject struct {
	items *collections.OrderedMap[string, LuaValue]
}

func LuaNull() LuaValue {
	return LuaValue{kind: LuaKindNull}
}

func LuaBool(value bool) LuaValue {
	return LuaValue{kind: LuaKindBool, boolValue: value}
}

func LuaInt(value int64) LuaValue {
	return LuaValue{kind: LuaKindInt, intValue: value}
}

func LuaFloat(value float64) LuaValue {
	return LuaValue{kind: LuaKindFloat, floatValue: value}
}

func LuaString(value string) LuaValue {
	return LuaValue{kind: LuaKindString, stringValue: value}
}

func LuaArray(values ...LuaValue) LuaValue {
	return LuaValue{kind: LuaKindArray, arrayValue: values}
}

func LuaObjectValue(object LuaObject) LuaValue {
	return LuaValue{kind: LuaKindObject, objectValue: &object}
}

func NewLuaObject() LuaObject {
	return LuaObject{items: collections.NewOrderedMap[string, LuaValue]()}
}

func LuaValueFromAny(raw any) (LuaValue, error) {
	switch typed := raw.(type) {
	case nil:
		return LuaNull(), nil
	case bool:
		return LuaBool(typed), nil
	case int:
		return LuaInt(int64(typed)), nil
	case int64:
		return LuaInt(typed), nil
	case float64:
		if math.Trunc(typed) == typed {
			return LuaInt(int64(typed)), nil
		}
		return LuaFloat(typed), nil
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return LuaInt(integer), nil
		}

		floatValue, err := typed.Float64()
		if err != nil {
			return LuaNull(), err
		}

		return LuaFloat(floatValue), nil
	case string:
		return LuaString(typed), nil
	case []any:
		values := make([]LuaValue, 0, len(typed))
		for _, item := range typed {
			value, err := LuaValueFromAny(item)
			if err != nil {
				return LuaNull(), err
			}
			values = append(values, value)
		}
		return LuaArray(values...), nil
	case map[string]any:
		object := NewLuaObject()
		for key, item := range typed {
			value, err := LuaValueFromAny(item)
			if err != nil {
				return LuaNull(), err
			}
			object.Set(key, value)
		}
		return LuaObjectValue(object), nil
	default:
		return LuaString(fmt.Sprintf("%v", raw)), nil
	}
}

func (value LuaValue) Kind() LuaKind {
	if value.kind == "" {
		return LuaKindNull
	}

	return value.kind
}

func (value LuaValue) IsNull() bool {
	return value.Kind() == LuaKindNull
}

func (value LuaValue) AsBool() (bool, bool) {
	if value.Kind() != LuaKindBool {
		return false, false
	}

	return value.boolValue, true
}

func (value LuaValue) AsString() (string, bool) {
	switch value.Kind() {
	case LuaKindNull, LuaKindBool, LuaKindArray, LuaKindObject:
		return "", false
	case LuaKindString:
		return value.stringValue, true
	case LuaKindInt:
		return strconv.FormatInt(value.intValue, 10), true
	case LuaKindFloat:
		return strconv.FormatFloat(value.floatValue, 'f', -1, 64), true
	}

	return "", false
}

func (value LuaValue) AsInt64() (int64, bool) {
	switch value.Kind() {
	case LuaKindNull, LuaKindBool, LuaKindString, LuaKindArray, LuaKindObject:
		return 0, false
	case LuaKindInt:
		return value.intValue, true
	case LuaKindFloat:
		if math.Trunc(value.floatValue) == value.floatValue {
			return int64(value.floatValue), true
		}
		return 0, false
	}

	return 0, false
}

func (value LuaValue) AsFloat64() (float64, bool) {
	switch value.Kind() {
	case LuaKindNull, LuaKindBool, LuaKindString, LuaKindArray, LuaKindObject:
		return 0, false
	case LuaKindInt:
		return float64(value.intValue), true
	case LuaKindFloat:
		return value.floatValue, true
	}

	return 0, false
}

func (value LuaValue) AsArray() ([]LuaValue, bool) {
	if value.Kind() != LuaKindArray {
		return nil, false
	}

	return value.arrayValue, true
}

func (value LuaValue) AsObject() (LuaObject, bool) {
	if value.Kind() != LuaKindObject || value.objectValue == nil {
		return LuaObject{}, false
	}

	return *value.objectValue, true
}

func (value LuaValue) Clone() LuaValue {
	switch value.Kind() {
	case LuaKindNull, LuaKindBool, LuaKindInt, LuaKindFloat, LuaKindString:
		return value
	case LuaKindArray:
		items := make([]LuaValue, len(value.arrayValue))
		for index := range value.arrayValue {
			items[index] = value.arrayValue[index].Clone()
		}
		return LuaValue{kind: LuaKindArray, arrayValue: items}
	case LuaKindObject:
		if value.objectValue == nil {
			return LuaValue{kind: LuaKindObject, objectValue: &LuaObject{items: collections.NewOrderedMap[string, LuaValue]()}}
		}
		cloned := value.objectValue.Clone()
		return LuaValue{kind: LuaKindObject, objectValue: &cloned}
	}

	return value
}

func LuaValuesEqual(left LuaValue, right LuaValue) bool {
	if left.Kind() != right.Kind() {
		return false
	}

	switch left.Kind() {
	case LuaKindNull:
		return true
	case LuaKindBool:
		return left.boolValue == right.boolValue
	case LuaKindInt:
		return left.intValue == right.intValue
	case LuaKindFloat:
		return left.floatValue == right.floatValue
	case LuaKindString:
		return left.stringValue == right.stringValue
	case LuaKindArray:
		if len(left.arrayValue) != len(right.arrayValue) {
			return false
		}
		for index := range left.arrayValue {
			if !LuaValuesEqual(left.arrayValue[index], right.arrayValue[index]) {
				return false
			}
		}
		return true
	case LuaKindObject:
		return LuaObjectsEqual(derefObject(left.objectValue), derefObject(right.objectValue))
	default:
		return false
	}
}

func LuaObjectsEqual(left LuaObject, right LuaObject) bool {
	if left.Len() != right.Len() {
		return false
	}

	equal := true
	left.Range(func(key string, leftValue LuaValue) bool {
		rightValue, ok := right.Get(key)
		if !ok || !LuaValuesEqual(leftValue, rightValue) {
			equal = false
			return false
		}
		return true
	})

	return equal
}

func DiffLuaObjects(baseline LuaObject, current LuaObject) LuaObject {
	out := NewLuaObject()

	current.Range(func(key string, currentValue LuaValue) bool {
		baselineValue, ok := baseline.Get(key)
		if !ok {
			out.Set(key, currentValue.Clone())
			return true
		}

		currentObject, currentIsObject := currentValue.AsObject()
		baselineObject, baselineIsObject := baselineValue.AsObject()
		if currentIsObject && baselineIsObject {
			nested := DiffLuaObjects(baselineObject, currentObject)
			if nested.Len() > 0 {
				out.Set(key, LuaObjectValue(nested))
			}
			return true
		}

		if !LuaValuesEqual(baselineValue, currentValue) {
			out.Set(key, currentValue.Clone())
		}
		return true
	})

	return out
}

func MergeLuaObjects(parent LuaObject, child LuaObject, shouldDelete func(LuaValue) bool) LuaObject {
	out := parent.Clone()

	child.Range(func(key string, childValue LuaValue) bool {
		if shouldDelete != nil && shouldDelete(childValue) {
			out.Delete(key)
			return true
		}

		childObject, childIsObject := childValue.AsObject()
		if !childIsObject {
			out.Set(key, childValue.Clone())
			return true
		}

		parentValue, hasParent := out.Get(key)
		parentObject, parentIsObject := parentValue.AsObject()
		if hasParent && parentIsObject {
			out.Set(key, LuaObjectValue(MergeLuaObjects(parentObject, childObject, shouldDelete)))
			return true
		}

		out.Set(key, LuaObjectValue(PruneLuaObject(childObject, shouldDelete)))
		return true
	})

	return out
}

func PruneLuaObject(object LuaObject, shouldDelete func(LuaValue) bool) LuaObject {
	out := NewLuaObject()

	object.Range(func(key string, value LuaValue) bool {
		if shouldDelete != nil && shouldDelete(value) {
			return true
		}

		nested, ok := value.AsObject()
		if ok {
			out.Set(key, LuaObjectValue(PruneLuaObject(nested, shouldDelete)))
			return true
		}

		out.Set(key, value.Clone())
		return true
	})

	return out
}

func (value LuaValue) MarshalJSON() ([]byte, error) {
	switch value.Kind() {
	case LuaKindNull:
		return []byte("null"), nil
	case LuaKindBool:
		return json.Marshal(value.boolValue)
	case LuaKindInt:
		return json.Marshal(value.intValue)
	case LuaKindFloat:
		return json.Marshal(value.floatValue)
	case LuaKindString:
		return json.Marshal(value.stringValue)
	case LuaKindArray:
		return json.Marshal(value.arrayValue)
	case LuaKindObject:
		if value.objectValue == nil {
			return []byte("{}"), nil
		}
		return value.objectValue.MarshalJSON()
	default:
		return nil, fmt.Errorf("unsupported lua value kind %q", value.kind)
	}
}

func (value *LuaValue) UnmarshalJSON(data []byte) error {
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

func (object LuaObject) Len() int {
	if object.items == nil {
		return 0
	}

	return object.items.Len()
}

func (object LuaObject) IsZero() bool {
	return object.Len() == 0
}

func (object LuaObject) Keys() []string {
	if object.items == nil {
		return nil
	}

	return object.items.Keys()
}

func (object *LuaObject) Set(key string, value LuaValue) {
	object.ensureMap()
	object.items.Put(key, value)
}

func (object LuaObject) Get(key string) (LuaValue, bool) {
	if object.items == nil {
		return LuaNull(), false
	}

	raw, ok := object.items.Get(key)
	if !ok {
		return LuaNull(), false
	}

	return raw, true
}

func (object LuaObject) MustGet(key string) LuaValue {
	if value, ok := object.Get(key); ok {
		return value
	}

	return LuaNull()
}

func (object *LuaObject) Delete(key string) {
	if object.items == nil {
		return
	}

	object.items.Delete(key)
}

func (object LuaObject) Range(fn func(key string, value LuaValue) bool) {
	if object.items == nil {
		return
	}

	object.items.Range(func(key string, value LuaValue) bool {
		return fn(key, value)
	})
}

func (object LuaObject) Clone() LuaObject {
	cloned := NewLuaObject()
	if object.items == nil {
		return cloned
	}

	object.items.Range(func(key string, value LuaValue) bool {
		cloned.items.Put(key, value.Clone())
		return true
	})
	return cloned
}

func (object LuaObject) MarshalJSON() ([]byte, error) {
	if object.items == nil || object.items.Len() == 0 {
		return []byte("{}"), nil
	}

	var buffer bytes.Buffer
	buffer.WriteByte('{')

	first := true
	var marshalErr error
	object.Range(func(key string, value LuaValue) bool {
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

func (object *LuaObject) UnmarshalJSON(data []byte) error {
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

func (object *LuaObject) ensureMap() {
	if object.items != nil {
		return
	}

	object.items = collections.NewOrderedMap[string, LuaValue]()
}

func derefObject(object *LuaObject) LuaObject {
	if object == nil {
		return LuaObject{}
	}
	return *object
}

func decodeJSONValue(decoder *json.Decoder) (LuaValue, error) {
	token, err := decoder.Token()
	if err != nil {
		return LuaNull(), err
	}

	switch typed := token.(type) {
	case nil:
		return LuaNull(), nil
	case bool:
		return LuaBool(typed), nil
	case string:
		return LuaString(typed), nil
	case json.Number:
		if integer, intErr := typed.Int64(); intErr == nil {
			return LuaInt(integer), nil
		}

		floatValue, floatErr := typed.Float64()
		if floatErr != nil {
			return LuaNull(), floatErr
		}

		return LuaFloat(floatValue), nil
	case json.Delim:
		switch typed {
		case '[':
			values := make([]LuaValue, 0)
			for decoder.More() {
				value, decodeErr := decodeJSONValue(decoder)
				if decodeErr != nil {
					return LuaNull(), decodeErr
				}
				values = append(values, value)
			}
			if _, closeErr := decoder.Token(); closeErr != nil {
				return LuaNull(), closeErr
			}
			return LuaArray(values...), nil
		case '{':
			object := NewLuaObject()
			for decoder.More() {
				keyToken, keyErr := decoder.Token()
				if keyErr != nil {
					return LuaNull(), keyErr
				}
				key, ok := keyToken.(string)
				if !ok {
					return LuaNull(), fmt.Errorf("expected JSON object key, got %T", keyToken)
				}
				value, valueErr := decodeJSONValue(decoder)
				if valueErr != nil {
					return LuaNull(), valueErr
				}
				object.Set(key, value)
			}
			if _, closeErr := decoder.Token(); closeErr != nil {
				return LuaNull(), closeErr
			}
			return LuaObjectValue(object), nil
		default:
			return LuaNull(), fmt.Errorf("unexpected JSON delimiter %q", typed)
		}
	default:
		return LuaNull(), fmt.Errorf("unsupported JSON token %T", token)
	}
}
