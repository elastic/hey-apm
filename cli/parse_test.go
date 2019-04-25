package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseCmdOptions(t *testing.T) {
	cmd := []string{"command", "-b", "--foo", "--str", "bar", "-i", "3", "--timeout", "1m"}

	cmd, str := ParseCmdStringOption(cmd, "--str", "DEFAULT")
	assert.Equal(t, "bar", str)
	cmd, str2 := ParseCmdStringOption(cmd, "--str2", "DEFAULT")
	assert.Equal(t, "DEFAULT", str2)

	cmd, b := ParseCmdBoolOption(cmd, "-b")
	assert.Equal(t, true, b)
	cmd, v := ParseCmdBoolOption(cmd, "-v")
	assert.Equal(t, false, v)

	cmd, i := ParseCmdIntOption(cmd, "-i", 10)
	assert.Equal(t, 3, i)

	cmd, i2 := ParseCmdIntOption(cmd, "-i2", 10)
	assert.Equal(t, 10, i2)

	cmd, timeout := ParseCmdDurationOption(cmd, "--timeout", time.Second)
	assert.Equal(t, time.Minute, timeout)

	cmd, timeout2 := ParseCmdDurationOption(cmd, "--timeout2", time.Second)
	assert.Equal(t, time.Second, timeout2)

	assert.Equal(t, []string{"command", "--foo"}, cmd)
}
