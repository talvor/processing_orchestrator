// Package utilities provides helper functions for various tasks
package utilities

import (
	"fmt"
	"reflect"
)

// FlattenStruct takes a nested struct as input and flattens it into a map[string]any. The keys for the map are generated such that nested fields use "_" as the separator. This allows for representing deeply nested fields in a flat key-value map.
func FlattenStruct(input any) (map[string]any, error) {
	if input == nil {
		return nil, fmt.Errorf("params must be a non-nil struct")
	}

	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("params must be a struct, got %s", val.Kind())
	}

	result := make(map[string]any)
	if err := flatten(reflect.ValueOf(input), "", result); err != nil {
		return nil, err
	}
	return result, nil
}

// Recursive helper function to perform the flattening.
func flatten(value reflect.Value, prefix string, result map[string]any) error {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		return flatten(value.Elem(), prefix, result)
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Type().Field(i)
			// Skip unexported fields
			if field.PkgPath != "" {
				continue
			}
			fieldValue := value.Field(i)
			fieldName := field.Name
			if prefix != "" {
				fieldName = prefix + "_" + field.Name
			}
			if err := flatten(fieldValue, fieldName, result); err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			stringKey := fmt.Sprintf("%v", key)
			fieldName := prefix + "_" + stringKey
			if err := flatten(value.MapIndex(key), fieldName, result); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			fieldName := fmt.Sprintf("%s_%d", prefix, i)
			if err := flatten(value.Index(i), fieldName, result); err != nil {
				return err
			}
		}
	default:
		result[prefix] = value.Interface()
	}
	return nil
}
