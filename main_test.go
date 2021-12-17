package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All JSON fields in Input are required, with zero values being meaningful.
func TestDefaultInput(t *testing.T) {
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
	for _, k := range expectedZeroValues {
		expectedZeroValuesMap[k] = true
	}

	os.Args[1] = "-bench"
	input := parseFlags()
	assert.True(t, input.IsBenchmark)

	encodedInput, err := json.Marshal(input)
	require.NoError(t, err)
	inputMap := make(map[string]interface{})
	require.NoError(t, json.Unmarshal(encodedInput, &inputMap))

	for k, v := range inputMap {
		if expectedZeroValuesMap[k] {
			continue
		}
		// any zero values not in `expectedZeroValues` should have been omitted
		r := reflect.ValueOf(v)
		assert.False(t, r.IsZero(), fmt.Sprintf("field %s has zero value %v", k, v))
	}
}
