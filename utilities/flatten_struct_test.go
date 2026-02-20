package utilities

import (
	"reflect"
	"testing"
)

func TestFlattenStruct(t *testing.T) {
	type Nested struct {
		Level2 string
	}

	type Data struct {
		Field1 string
		Field2 int
		Field3 Nested
	}

	tests := []struct {
		in       Data
		expected map[string]any
	}{
		{
			Data{
				Field1: "value1",
				Field2: 42,
				Field3: Nested{Level2: "value2"},
			},
			map[string]any{
				"Field1":        "value1",
				"Field2":        42,
				"Field3_Level2": "value2",
			},
		},
	}

	for _, test := range tests {
		result, err := FlattenStruct(test.in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("For struct %+v, expected %+v but got %+v", test.in, test.expected, result)
		}
	}
}
