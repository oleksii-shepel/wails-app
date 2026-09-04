package formula

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Value represents the result of a formula evaluation.
type Value struct {
	// Type indicates the kind of value.
	Type ValueType

	// Num holds numeric values.
	Num float64

	// Text holds string values.
	Text string

	// Bool holds boolean values.
	Bool bool

	// Err holds error values.
	Err ErrorType
}

// ValueType indicates the type of a formula result.
type ValueType int

const (
	TypeNumber ValueType = iota
	TypeText
	TypeBool
	TypeError
	TypeEmpty
)

// ErrorType represents Excel error values.
type ErrorType int

const (
	ErrNone ErrorType = iota
	ErrDiv0
	ErrValue
	ErrRef
	ErrName
	ErrNum
	ErrNA
	ErrSpill
	ErrNull
	ErrCalc
)

// String returns the Excel error string representation.
func (e ErrorType) String() string {
	switch e {
	case ErrDiv0:
		return "#DIV/0!"
	case ErrValue:
		return "#VALUE!"
	case ErrRef:
		return "#REF!"
	case ErrName:
		return "#NAME?"
	case ErrNum:
		return "#NUM!"
	case ErrNA:
		return "#N/A"
	case ErrSpill:
		return "#SPILL!"
	case ErrNull:
		return "#NULL!"
	case ErrCalc:
		return "#CALC!"
	default:
		return ""
	}
}

// IsError returns true if the value is an error.
func (v Value) IsError() bool {
	return v.Type == TypeError
}

// IsNumber returns true if the value is a number.
func (v Value) IsNumber() bool {
	return v.Type == TypeNumber
}

// IsText returns true if the value is text.
func (v Value) IsText() bool {
	return v.Type == TypeText
}

// IsBool returns true if the value is a boolean.
func (v Value) IsBool() bool {
	return v.Type == TypeBool
}

// IsEmpty returns true if the value is empty.
func (v Value) IsEmpty() bool {
	return v.Type == TypeEmpty
}

// ToNumber attempts to convert the value to a number.
func (v Value) ToNumber() (float64, error) {
	switch v.Type {
	case TypeNumber:
		return v.Num, nil
	case TypeBool:
		if v.Bool {
			return 1, nil
		}
		return 0, nil
	case TypeText:
		// Try to parse as number
		text := strings.TrimSpace(v.Text)
		if text == "" {
			return 0, fmt.Errorf("cannot convert text to number")
		}
		num, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert text to number")
		}
		return num, nil
	case TypeError:
		return 0, fmt.Errorf("error value: %s", v.Err.String())
	default:
		return 0, nil
	}
}

// ToText converts the value to text.
func (v Value) ToText() string {
	switch v.Type {
	case TypeText:
		return v.Text
	case TypeNumber:
		// Use general format
		if v.Num == math.Trunc(v.Num) && math.Abs(v.Num) < 1e15 {
			return fmt.Sprintf("%.0f", v.Num)
		}
		return fmt.Sprintf("%g", v.Num)
	case TypeBool:
		if v.Bool {
			return "TRUE"
		}
		return "FALSE"
	case TypeError:
		return v.Err.String()
	default:
		return ""
	}
}

// ToBool attempts to convert the value to a boolean.
func (v Value) ToBool() (bool, error) {
	switch v.Type {
	case TypeBool:
		return v.Bool, nil
	case TypeNumber:
		return v.Num != 0, nil
	case TypeText:
		// Empty text is false, non-empty is true
		return v.Text != "", nil
	case TypeError:
		return false, fmt.Errorf("error value: %s", v.Err.String())
	default:
		return false, nil
	}
}

// Number creates a numeric value.
func Number(n float64) Value {
	return Value{Type: TypeNumber, Num: n}
}

// Text creates a text value.
func Text(t string) Value {
	return Value{Type: TypeText, Text: t}
}

// Bool creates a boolean value.
func Bool(b bool) Value {
	return Value{Type: TypeBool, Bool: b}
}

// Error creates an error value.
func Error(e ErrorType) Value {
	return Value{Type: TypeError, Err: e}
}

// Empty creates an empty value.
func Empty() Value {
	return Value{Type: TypeEmpty}
}

// ParseError parses an Excel error string.
func ParseError(s string) ErrorType {
	switch strings.ToUpper(s) {
	case "#DIV/0!", "#DIV/0":
		return ErrDiv0
	case "#VALUE!", "#VALUE":
		return ErrValue
	case "#REF!", "#REF":
		return ErrRef
	case "#NAME?", "#NAME":
		return ErrName
	case "#NUM!", "#NUM":
		return ErrNum
	case "#N/A", "#N/A!":
		return ErrNA
	case "#SPILL!", "#SPILL":
		return ErrSpill
	case "#NULL!", "#NULL":
		return ErrNull
	case "#CALC!", "#CALC":
		return ErrCalc
	default:
		return ErrNone
	}
}
