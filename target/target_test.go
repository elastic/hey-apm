package target

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTargets(t *testing.T) {
	var target *Target
	var err error

	urls := []string{""}
	_, err = NewTargetFromOptions(urls, RunTimeout("2"))
	assert.Error(t, err)
	assert.Equal(t, "time: missing unit in duration 2", err.Error())

	target, err = NewTargetFromOptions(urls, RunTimeout("2s"))
	assert.NoError(t, err)
	assert.Equal(t, time.Second*2, target.Config.RunTimeout)

	target, err = NewTargetFromOptions(urls, NumTransactions("a"), RunTimeout("3s"))
	assert.Error(t, err)
	assert.Equal(t, "strconv.Atoi: parsing \"a\": invalid syntax", err.Error())
	assert.Equal(t, time.Duration(0), target.Config.RunTimeout)

	target, err = NewTargetFromOptions(urls, RequestTimeout(time.Second))
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(1)*time.Second, target.Config.RequestTimeout)
}
