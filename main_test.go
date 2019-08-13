package main

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/elastic/hey-apm/conv"
	"github.com/elastic/hey-apm/strcoll"
	"github.com/stretchr/testify/assert"
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

	input := parseFlags()
	for k, v := range conv.ToMap(input) {
		if strcoll.Contains(k, expectedZeroValues) {
			continue
		}
		r := reflect.ValueOf(v)
		// any zero values not in `expectedZeroValues` are actually missing values
		assert.NotEqual(t, reflect.Zero(r.Type()).Interface(), r.Interface(),
			fmt.Sprintf("field %s has zero value %v", k, v))
	}
}
