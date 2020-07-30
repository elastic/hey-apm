package main

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/elastic/hey-apm/conv"
	"github.com/stretchr/testify/assert"
)

// All JSON fields in Input are required, with zero values being meaningful.
func TestDefaultInput(t *testing.T) {
	os.Args[1] = "-bench"
	expectedZeroValues := []string{
		"transaction_generation_limit",
		"transaction_generation_frequency",
		"spans_generated_max_limit",
		"spans_generated_min_limit",
		"error_generation_limit",
		"error_generation_frequency",
		"error_generation_frames_max_limit",
		"error_generation_frames_min_limit",
	}
	expectedZeroValuesMap := make(map[string]bool)
	for _, field := range expectedZeroValues {
		expectedZeroValuesMap[field] = true
	}

	input := parseFlags()
	assert.True(t, input.IsBenchmark)
	for k, v := range conv.ToMap(input) {
		if expectedZeroValuesMap[k] {
			continue
		}
		r := reflect.ValueOf(v)
		// any zero values not in `expectedZeroValues` are actually missing values
		assert.NotEqual(t, reflect.Zero(r.Type()).Interface(), r.Interface(),
			fmt.Sprintf("field %s has zero value %v", k, v))
	}
}
