package state

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStateLifecycle(t *testing.T) {
	os.Setenv("LDAPPY_STATE_DIR", t.TempDir())

	assert.NoError(t, Init(), "should initialize state")

	SetVerbose(true)
	SetJSONOutput(true)
	SetVersion("v9.9.9")
	SetConfigPath("/tmp/config.toml")

	st := Get()
	assert.True(t, st.Verbose)
	assert.True(t, st.JSONOutput)
	assert.Equal(t, "v9.9.9", st.AppVersion)
	assert.Equal(t, "/tmp/config.toml", st.ConfigPath)

	oldTime := st.LastUsed
	time.Sleep(10 * time.Millisecond)
	assert.NoError(t, Save())
	st2 := Get()
	assert.True(t, st2.LastUsed.After(oldTime))
}
