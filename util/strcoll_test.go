package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNth(t *testing.T) {
	assert.Equal(t, "", Get(1, nil))
	assert.Equal(t, "", Get(1, []string{}))
	assert.Equal(t, "", Get(1, []string{"a"}))
	assert.Equal(t, "b", Get(1, []string{"a", "b"}))
}

func TestRest(t *testing.T) {
	assert.Equal(t, []string{}, From(1, nil))
	assert.Equal(t, []string{}, From(1, []string{}))
	assert.Equal(t, []string{}, From(1, []string{"a"}))
	assert.Equal(t, []string{"b", "c"}, From(1, []string{"a", "b", "c"}))
}

func TestCopy(t *testing.T) {
	assert.Equal(t, map[string][]string{}, Copy(nil))
	assert.Equal(t, map[string][]string{}, Copy(map[string][]string{}))
	assert.Equal(t, map[string][]string{"a": {"b", "c"}, "d": nil}, Copy(map[string][]string{"a": {"b", "c"}, "d": nil}))
}

func TestContains(t *testing.T) {
	xs := []string{"abc", "cde", "efg"}
	assert.True(t, Contains("cde", xs))
	assert.False(t, Contains("e", xs))
}

func TestConcat(t *testing.T) {
	m1 := map[string]string{"a": "1", "b": "2", "c": "3"}
	m2 := map[string]string{"d": "4", "e": "5"}
	m3 := map[string]string{}
	m4 := map[string]string{"f": "6", "a": "7"}

	expected := map[string]string{
		"b": "2", "c": "3", "d": "4", "e": "5", "f": "6", "a": "7",
	}
	assert.Equal(t, expected, Concat(m1, m2, m3, m4))
}
