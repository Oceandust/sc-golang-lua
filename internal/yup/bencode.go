package yup

import (
	"bytes"
	"fmt"
	"strconv"

	"sc_cli/internal/collections"
)

type valueKind uint8

const (
	kindBytes valueKind = iota + 1
	kindInt
	kindList
	kindDict
)

type Value struct {
	kind      valueKind
	bytesData []byte
	intData   int64
	listData  []*Value
	dictData  *collections.OrderedMap[string, *Value]
}

func BytesValue(data []byte) *Value {
	return &Value{kind: kindBytes, bytesData: append([]byte(nil), data...)}
}

func StringValue(text string) *Value {
	return BytesValue([]byte(text))
}

func IntValue(value int64) *Value {
	return &Value{kind: kindInt, intData: value}
}

func ListValue(values []*Value) *Value {
	out := make([]*Value, len(values))
	copy(out, values)
	return &Value{kind: kindList, listData: out}
}

func DictValue() *Value {
	return &Value{kind: kindDict, dictData: collections.NewOrderedMap[string, *Value]()}
}

func (value *Value) AsBytes() ([]byte, bool) {
	if value == nil || value.kind != kindBytes {
		return nil, false
	}
	return append([]byte(nil), value.bytesData...), true
}

func (value *Value) AsString() (string, bool) {
	raw, ok := value.AsBytes()
	if !ok {
		return "", false
	}
	return string(raw), true
}

func (value *Value) AsInt() (int64, bool) {
	if value == nil || value.kind != kindInt {
		return 0, false
	}
	return value.intData, true
}

func (value *Value) AsList() ([]*Value, bool) {
	if value == nil || value.kind != kindList {
		return nil, false
	}
	out := make([]*Value, len(value.listData))
	copy(out, value.listData)
	return out, true
}

func (value *Value) AsDict() (*collections.OrderedMap[string, *Value], bool) {
	if value == nil || value.kind != kindDict {
		return nil, false
	}
	return value.dictData, true
}

func (value *Value) Get(key string) (*Value, bool) {
	dict, ok := value.AsDict()
	if !ok {
		return nil, false
	}
	return dict.Get(key)
}

func (value *Value) Set(key string, child *Value) bool {
	dict, ok := value.AsDict()
	if !ok {
		return false
	}
	dict.Put(key, child)
	return true
}

func Parse(data []byte) (*Value, error) {
	parser := bencodeParser{data: data}
	value, err := parser.parseValue()
	if err != nil {
		return nil, err
	}
	if parser.position != len(data) {
		return nil, fmt.Errorf("unexpected trailing data at offset %d", parser.position)
	}
	return value, nil
}

func Encode(value *Value) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	if err := encodeValue(buffer, value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

type bencodeParser struct {
	data     []byte
	position int
}

func (parser *bencodeParser) parseValue() (*Value, error) {
	if parser.position >= len(parser.data) {
		return nil, fmt.Errorf("unexpected end of input")
	}

	switch parser.data[parser.position] {
	case 'i':
		return parser.parseInt()
	case 'l':
		return parser.parseList()
	case 'd':
		return parser.parseDict()
	default:
		if parser.data[parser.position] < '0' || parser.data[parser.position] > '9' {
			return nil, fmt.Errorf("invalid token %q at offset %d", parser.data[parser.position], parser.position)
		}
		return parser.parseBytes()
	}
}

func (parser *bencodeParser) parseBytes() (*Value, error) {
	start := parser.position
	for parser.position < len(parser.data) && parser.data[parser.position] != ':' {
		digit := parser.data[parser.position]
		if digit < '0' || digit > '9' {
			return nil, fmt.Errorf("invalid byte string length at offset %d", parser.position)
		}
		parser.position++
	}
	if parser.position >= len(parser.data) {
		return nil, fmt.Errorf("unterminated byte string length")
	}

	lengthValue, err := strconv.ParseInt(string(parser.data[start:parser.position]), 10, 64)
	if err != nil {
		return nil, err
	}
	parser.position++

	if lengthValue < 0 || parser.position+int(lengthValue) > len(parser.data) {
		return nil, fmt.Errorf("byte string overruns input at offset %d", parser.position)
	}

	value := BytesValue(parser.data[parser.position : parser.position+int(lengthValue)])
	parser.position += int(lengthValue)
	return value, nil
}

func (parser *bencodeParser) parseInt() (*Value, error) {
	parser.position++
	start := parser.position
	for parser.position < len(parser.data) && parser.data[parser.position] != 'e' {
		parser.position++
	}
	if parser.position >= len(parser.data) {
		return nil, fmt.Errorf("unterminated integer")
	}

	value, err := strconv.ParseInt(string(parser.data[start:parser.position]), 10, 64)
	if err != nil {
		return nil, err
	}
	parser.position++
	return IntValue(value), nil
}

func (parser *bencodeParser) parseList() (*Value, error) {
	parser.position++
	items := make([]*Value, 0)
	for {
		if parser.position >= len(parser.data) {
			return nil, fmt.Errorf("unterminated list")
		}
		if parser.data[parser.position] == 'e' {
			parser.position++
			return ListValue(items), nil
		}

		item, err := parser.parseValue()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
}

func (parser *bencodeParser) parseDict() (*Value, error) {
	parser.position++
	value := DictValue()
	for {
		if parser.position >= len(parser.data) {
			return nil, fmt.Errorf("unterminated dictionary")
		}
		if parser.data[parser.position] == 'e' {
			parser.position++
			return value, nil
		}

		keyValue, err := parser.parseBytes()
		if err != nil {
			return nil, err
		}
		key, _ := keyValue.AsString()
		child, err := parser.parseValue()
		if err != nil {
			return nil, err
		}
		value.Set(key, child)
	}
}

func encodeValue(buffer *bytes.Buffer, value *Value) error {
	if value == nil {
		return fmt.Errorf("cannot encode nil bencode value")
	}

	switch value.kind {
	case kindBytes:
		if _, err := buffer.WriteString(strconv.Itoa(len(value.bytesData))); err != nil {
			return err
		}
		if err := buffer.WriteByte(':'); err != nil {
			return err
		}
		_, err := buffer.Write(value.bytesData)
		return err
	case kindInt:
		if err := buffer.WriteByte('i'); err != nil {
			return err
		}
		if _, err := buffer.WriteString(strconv.FormatInt(value.intData, 10)); err != nil {
			return err
		}
		return buffer.WriteByte('e')
	case kindList:
		if err := buffer.WriteByte('l'); err != nil {
			return err
		}
		for _, item := range value.listData {
			if err := encodeValue(buffer, item); err != nil {
				return err
			}
		}
		return buffer.WriteByte('e')
	case kindDict:
		if err := buffer.WriteByte('d'); err != nil {
			return err
		}
		if value.dictData != nil {
			var encodeErr error
			value.dictData.Range(func(key string, child *Value) bool {
				if encodeErr != nil {
					return false
				}
				if encodeErr = encodeValue(buffer, StringValue(key)); encodeErr != nil {
					return false
				}
				if encodeErr = encodeValue(buffer, child); encodeErr != nil {
					return false
				}
				return true
			})
			if encodeErr != nil {
				return encodeErr
			}
		}
		return buffer.WriteByte('e')
	default:
		return fmt.Errorf("unsupported bencode kind %d", value.kind)
	}
}
