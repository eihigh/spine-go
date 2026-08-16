package spine

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// This file provides a small, ordered JSON tree mirroring libgdx's JsonValue,
// so SkeletonJson ports 1:1 from the reference Java implementation. Object key
// order and array order are preserved via the Child/Next linked list, exactly
// like libgdx.

// jsonError wraps an error raised by required accessors (Require*, As*). It is
// thrown with panic and recovered at loader boundaries by catchJSONError,
// mirroring Java's exception flow in SkeletonJson.
type jsonError struct{ error }

// jsonErrorf panics with a jsonError.
func jsonErrorf(format string, args ...any) {
	panic(jsonError{fmt.Errorf(format, args...)})
}

// catchJSONError recovers a jsonError panic into *err. Other panics are
// re-raised.
func catchJSONError(err *error) {
	if r := recover(); r != nil {
		if je, ok := r.(jsonError); ok {
			*err = je.error
		} else {
			panic(r)
		}
	}
}

type jsonType int

const (
	jsonTypeObject jsonType = iota
	jsonTypeArray
	jsonTypeString
	jsonTypeNumber
	jsonTypeBool
	jsonTypeNull
)

// jsonValue is a node of a parsed JSON tree, mirroring libgdx's JsonValue.
// Object members and array elements are stored as a Child/Next linked list,
// preserving document order.
type jsonValue struct {
	jsonType jsonType

	// Name is the object key this value is stored under, or "" for array
	// elements and the root.
	Name string

	// Child is the first member/element for objects and arrays, else nil.
	Child *jsonValue

	// Next is the next sibling, else nil.
	Next *jsonValue

	// Size is the number of children for objects and arrays, else 0.
	Size int

	str string  // String value, or the raw token for numbers.
	num float64 // Number value.
	b   bool    // Boolean value.
}

// IsString returns true if this is a JSON string value.
func (v *jsonValue) IsString() bool { return v.jsonType == jsonTypeString }

// IsNull returns true if this is a JSON null value.
func (v *jsonValue) IsNull() bool { return v.jsonType == jsonTypeNull }

// isValue returns true if this is a string, number, boolean or null value.
func (v *jsonValue) isValue() bool {
	switch v.jsonType {
	case jsonTypeString, jsonTypeNumber, jsonTypeBool, jsonTypeNull:
		return true
	}
	return false
}

// Get returns the child with the specified name, or nil. Like libgdx, the
// name comparison ignores case.
func (v *jsonValue) Get(name string) *jsonValue {
	current := v.Child
	for current != nil && (current.Name == "" || !strings.EqualFold(current.Name, name)) {
		current = current.Next
	}
	return current
}

// GetIndex returns the child at the specified index, or nil.
func (v *jsonValue) GetIndex(index int) *jsonValue {
	current := v.Child
	for current != nil && index > 0 {
		index--
		current = current.Next
	}
	return current
}

// GetChild returns the first child of the child with the specified name, or
// nil. Iterate with: for value := m.GetChild("name"); value != nil; value = value.Next.
func (v *jsonValue) GetChild(name string) *jsonValue {
	child := v.Get(name)
	if child == nil {
		return nil
	}
	return child.Child
}

// Has returns true if a child with the specified name exists.
func (v *jsonValue) Has(name string) bool { return v.Get(name) != nil }

// Require returns the child with the specified name, raising a jsonError if
// not found.
func (v *jsonValue) Require(name string) *jsonValue {
	child := v.Get(name)
	if child == nil {
		jsonErrorf("spine: named value not found: %s", name)
	}
	return child
}

// AsString returns this value as a string. Null returns "".
func (v *jsonValue) AsString() string {
	switch v.jsonType {
	case jsonTypeString, jsonTypeNumber:
		return v.str
	case jsonTypeBool:
		if v.b {
			return "true"
		}
		return "false"
	case jsonTypeNull:
		return ""
	}
	jsonErrorf("spine: value cannot be converted to string: %s", v.Name)
	return ""
}

// AsFloat returns this value as a float32.
func (v *jsonValue) AsFloat() float32 {
	switch v.jsonType {
	case jsonTypeNumber:
		return float32(v.num)
	case jsonTypeString:
		f, err := strconv.ParseFloat(v.str, 32)
		if err != nil {
			jsonErrorf("spine: value cannot be converted to float: %q", v.str)
		}
		return float32(f)
	case jsonTypeBool:
		if v.b {
			return 1
		}
		return 0
	}
	jsonErrorf("spine: value cannot be converted to float: %s", v.Name)
	return 0
}

// AsInt returns this value as an int.
func (v *jsonValue) AsInt() int {
	switch v.jsonType {
	case jsonTypeNumber:
		return int(v.num)
	case jsonTypeString:
		i, err := strconv.Atoi(v.str)
		if err != nil {
			jsonErrorf("spine: value cannot be converted to int: %q", v.str)
		}
		return i
	case jsonTypeBool:
		if v.b {
			return 1
		}
		return 0
	}
	jsonErrorf("spine: value cannot be converted to int: %s", v.Name)
	return 0
}

// AsBoolean returns this value as a bool.
func (v *jsonValue) AsBoolean() bool {
	switch v.jsonType {
	case jsonTypeBool:
		return v.b
	case jsonTypeNumber:
		return v.num != 0
	case jsonTypeString:
		return v.str == "true"
	}
	jsonErrorf("spine: value cannot be converted to boolean: %s", v.Name)
	return false
}

// AsFloatArray returns the children of this array as a []float32.
func (v *jsonValue) AsFloatArray() []float32 {
	if v.jsonType != jsonTypeArray {
		jsonErrorf("spine: value is not an array: %s", v.Name)
	}
	array := make([]float32, 0, v.Size)
	for child := v.Child; child != nil; child = child.Next {
		array = append(array, child.AsFloat())
	}
	return array
}

// AsIntArray returns the children of this array as a []int.
func (v *jsonValue) AsIntArray() []int {
	if v.jsonType != jsonTypeArray {
		jsonErrorf("spine: value is not an array: %s", v.Name)
	}
	array := make([]int, 0, v.Size)
	for child := v.Child; child != nil; child = child.Next {
		array = append(array, child.AsInt())
	}
	return array
}

// AsShortArray returns the children of this array as a []uint16 (Java short[]
// maps to []uint16 for triangles and edges).
func (v *jsonValue) AsShortArray() []uint16 {
	if v.jsonType != jsonTypeArray {
		jsonErrorf("spine: value is not an array: %s", v.Name)
	}
	array := make([]uint16, 0, v.Size)
	for child := v.Child; child != nil; child = child.Next {
		array = append(array, uint16(child.AsInt()))
	}
	return array
}

// GetString returns the string value of the child with the specified name, or
// defaultValue if not found or null.
func (v *jsonValue) GetString(name, defaultValue string) string {
	child := v.Get(name)
	if child == nil || !child.isValue() || child.IsNull() {
		return defaultValue
	}
	return child.AsString()
}

// GetFloat returns the float value of the child with the specified name, or
// defaultValue if not found or null.
func (v *jsonValue) GetFloat(name string, defaultValue float32) float32 {
	child := v.Get(name)
	if child == nil || !child.isValue() || child.IsNull() {
		return defaultValue
	}
	return child.AsFloat()
}

// GetInt returns the int value of the child with the specified name, or
// defaultValue if not found or null.
func (v *jsonValue) GetInt(name string, defaultValue int) int {
	child := v.Get(name)
	if child == nil || !child.isValue() || child.IsNull() {
		return defaultValue
	}
	return child.AsInt()
}

// GetBoolean returns the bool value of the child with the specified name, or
// defaultValue if not found or null.
func (v *jsonValue) GetBoolean(name string, defaultValue bool) bool {
	child := v.Get(name)
	if child == nil || !child.isValue() || child.IsNull() {
		return defaultValue
	}
	return child.AsBoolean()
}

// RequireString returns the string value of the child with the specified
// name, raising a jsonError if not found.
func (v *jsonValue) RequireString(name string) string { return v.Require(name).AsString() }

// RequireFloat returns the float value of the child with the specified name,
// raising a jsonError if not found.
func (v *jsonValue) RequireFloat(name string) float32 { return v.Require(name).AsFloat() }

// RequireInt returns the int value of the child with the specified name,
// raising a jsonError if not found.
func (v *jsonValue) RequireInt(name string) int { return v.Require(name).AsInt() }

// parseJSON parses data as a JSON document and returns the root value.
func parseJSON(data []byte) (*jsonValue, error) {
	p := &jsonParser{data: data}
	// Skip a UTF-8 byte order mark, if present.
	if len(p.data) >= 3 && p.data[0] == 0xef && p.data[1] == 0xbb && p.data[2] == 0xbf {
		p.pos = 3
	}
	p.skipWhitespace()
	root, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	p.skipWhitespace()
	if p.pos != len(p.data) {
		return nil, p.errorf("unexpected trailing data")
	}
	return root, nil
}

type jsonParser struct {
	data []byte
	pos  int
}

func (p *jsonParser) errorf(format string, args ...any) error {
	return fmt.Errorf("spine: invalid JSON at offset %d: %s", p.pos, fmt.Sprintf(format, args...))
}

func (p *jsonParser) skipWhitespace() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *jsonParser) parseValue() (*jsonValue, error) {
	if p.pos >= len(p.data) {
		return nil, p.errorf("unexpected end of data")
	}
	switch c := p.data[p.pos]; c {
	case '{':
		return p.parseObject()
	case '[':
		return p.parseArray()
	case '"':
		s, err := p.parseString()
		if err != nil {
			return nil, err
		}
		return &jsonValue{jsonType: jsonTypeString, str: s}, nil
	case 't':
		if err := p.expect("true"); err != nil {
			return nil, err
		}
		return &jsonValue{jsonType: jsonTypeBool, b: true}, nil
	case 'f':
		if err := p.expect("false"); err != nil {
			return nil, err
		}
		return &jsonValue{jsonType: jsonTypeBool}, nil
	case 'n':
		if err := p.expect("null"); err != nil {
			return nil, err
		}
		return &jsonValue{jsonType: jsonTypeNull}, nil
	default:
		if c == '-' || (c >= '0' && c <= '9') {
			return p.parseNumber()
		}
		return nil, p.errorf("unexpected character %q", c)
	}
}

func (p *jsonParser) expect(literal string) error {
	if p.pos+len(literal) > len(p.data) || string(p.data[p.pos:p.pos+len(literal)]) != literal {
		return p.errorf("expected %q", literal)
	}
	p.pos += len(literal)
	return nil
}

func (p *jsonParser) parseObject() (*jsonValue, error) {
	object := &jsonValue{jsonType: jsonTypeObject}
	p.pos++ // '{'
	p.skipWhitespace()
	if p.pos < len(p.data) && p.data[p.pos] == '}' {
		p.pos++
		return object, nil
	}
	var last *jsonValue
	for {
		p.skipWhitespace()
		if p.pos >= len(p.data) || p.data[p.pos] != '"' {
			return nil, p.errorf("expected object key")
		}
		name, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if p.pos >= len(p.data) || p.data[p.pos] != ':' {
			return nil, p.errorf("expected ':' after object key")
		}
		p.pos++
		p.skipWhitespace()
		child, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		child.Name = name
		if last == nil {
			object.Child = child
		} else {
			last.Next = child
		}
		last = child
		object.Size++
		p.skipWhitespace()
		if p.pos >= len(p.data) {
			return nil, p.errorf("unterminated object")
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return object, nil
		default:
			return nil, p.errorf("expected ',' or '}' in object")
		}
	}
}

func (p *jsonParser) parseArray() (*jsonValue, error) {
	array := &jsonValue{jsonType: jsonTypeArray}
	p.pos++ // '['
	p.skipWhitespace()
	if p.pos < len(p.data) && p.data[p.pos] == ']' {
		p.pos++
		return array, nil
	}
	var last *jsonValue
	for {
		p.skipWhitespace()
		child, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if last == nil {
			array.Child = child
		} else {
			last.Next = child
		}
		last = child
		array.Size++
		p.skipWhitespace()
		if p.pos >= len(p.data) {
			return nil, p.errorf("unterminated array")
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			return array, nil
		default:
			return nil, p.errorf("expected ',' or ']' in array")
		}
	}
}

func (p *jsonParser) parseString() (string, error) {
	p.pos++ // '"'
	start := p.pos
	// Fast path: no escapes.
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == '"' {
			s := string(p.data[start:p.pos])
			p.pos++
			return s, nil
		}
		if c == '\\' {
			break
		}
		p.pos++
	}
	if p.pos >= len(p.data) {
		return "", p.errorf("unterminated string")
	}
	// Slow path: escapes present.
	buf := append([]byte(nil), p.data[start:p.pos]...)
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		switch c {
		case '"':
			p.pos++
			return string(buf), nil
		case '\\':
			p.pos++
			if p.pos >= len(p.data) {
				return "", p.errorf("unterminated string escape")
			}
			switch e := p.data[p.pos]; e {
			case '"', '\\', '/':
				buf = append(buf, e)
				p.pos++
			case 'b':
				buf = append(buf, '\b')
				p.pos++
			case 'f':
				buf = append(buf, '\f')
				p.pos++
			case 'n':
				buf = append(buf, '\n')
				p.pos++
			case 'r':
				buf = append(buf, '\r')
				p.pos++
			case 't':
				buf = append(buf, '\t')
				p.pos++
			case 'u':
				p.pos++
				r1, err := p.parseHex4()
				if err != nil {
					return "", err
				}
				r := rune(r1)
				if utf16.IsSurrogate(r) && p.pos+1 < len(p.data) && p.data[p.pos] == '\\' && p.data[p.pos+1] == 'u' {
					p.pos += 2
					r2, err := p.parseHex4()
					if err != nil {
						return "", err
					}
					r = utf16.DecodeRune(r, rune(r2))
				}
				buf = utf8.AppendRune(buf, r)
			default:
				return "", p.errorf("invalid string escape %q", e)
			}
		default:
			buf = append(buf, c)
			p.pos++
		}
	}
	return "", p.errorf("unterminated string")
}

func (p *jsonParser) parseHex4() (uint16, error) {
	if p.pos+4 > len(p.data) {
		return 0, p.errorf("invalid unicode escape")
	}
	v, err := strconv.ParseUint(string(p.data[p.pos:p.pos+4]), 16, 32)
	if err != nil {
		return 0, p.errorf("invalid unicode escape")
	}
	p.pos += 4
	return uint16(v), nil
}

func (p *jsonParser) parseNumber() (*jsonValue, error) {
	start := p.pos
	for p.pos < len(p.data) {
		switch c := p.data[p.pos]; {
		case c >= '0' && c <= '9', c == '-', c == '+', c == '.', c == 'e', c == 'E':
			p.pos++
		default:
			goto done
		}
	}
done:
	token := string(p.data[start:p.pos])
	num, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return nil, p.errorf("invalid number %q", token)
	}
	return &jsonValue{jsonType: jsonTypeNumber, str: token, num: num}, nil
}
