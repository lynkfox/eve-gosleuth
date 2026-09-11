package comm

import "reflect"

// HasProperties checks if the given 'any' value is a struct that has fields.
func HasProperties(v any, fieldName string) (any, bool) {
	val := reflect.ValueOf(v)

	// If it's a pointer, dereference it to inspect the underlying value
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	// verify its actually a struct
	if val.Kind() != reflect.Struct {
		return nil, false
	}

	// then see if the field exists
	field := val.FieldByName(fieldName)

	if !field.IsValid() || !field.CanInterface() {
		return nil, false
	}

	return field.Interface(), true
}
