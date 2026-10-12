package safety

import (
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

func TestValidateCommandLine(t *testing.T) {
	t.Run("valid command", func(t *testing.T) {
		cmd := "ls ./"
		valid := ValidateCommand(cmd)
		assert.Equal(t, valid, true)
	})

	t.Run("invalid command with curl", func(t *testing.T) {
		cmd := "curl xxx.xxx.xxx"
		valid := ValidateCommand(cmd)
		assert.Equal(t, valid, false)
	})

	t.Run("invalid command with ip address", func(t *testing.T) {
		cmd := "xx 10.10.111.111"
		valid := ValidateCommand(cmd)
		assert.Equal(t, valid, false)
	})

	t.Run("invalid command with domain", func(t *testing.T) {
		cmd := "xx szth.ivip.dev.com"
		valid := ValidateCommand(cmd)
		assert.Equal(t, valid, false)
	})
}

func TestExistForbiddenCommand(t *testing.T) {
	t.Run("valid command", func(t *testing.T) {
		cmd := "ls ./"
		exist := MatchForbiddenCommand(cmd)
		assert.Equal(t, exist, false)
	})

	t.Run("invalid command with wget", func(t *testing.T) {
		cmd := "wget xxx"
		exist := MatchForbiddenCommand(cmd)
		assert.Equal(t, exist, true)
	})

	t.Run("invalid command with curl", func(t *testing.T) {
		cmd := "curl xxx"
		exist := MatchForbiddenCommand(cmd)
		assert.Equal(t, exist, true)
	})

	t.Run("invalid command with ping", func(t *testing.T) {
		cmd := "ping xxx"
		exist := MatchForbiddenCommand(cmd)
		assert.Equal(t, exist, true)
	})
}

func TestExistIPAddress(t *testing.T) {
	t.Run("valid command", func(t *testing.T) {
		cmd := "ls ./"
		exist := MatchForbiddenCommand(cmd)
		assert.Equal(t, exist, false)
	})

	t.Run("invalid command with address", func(t *testing.T) {
		cmd := "wget 10.10.111.111"
		exist := MatchForbiddenCommand(cmd)
		assert.Equal(t, exist, true)
	})
}

func TestExistDomain(t *testing.T) {
	t.Run("valid command", func(t *testing.T) {
		cmd := "ls ./"
		exist := MatchForbiddenCommand(cmd)
		assert.Equal(t, exist, false)
	})

	t.Run("valid command with python package", func(t *testing.T) {
		cmd := "python -m vistudio.pipeline.export"
		exist := MatchForbiddenCommand(cmd)
		assert.Equal(t, exist, false)
	})

	t.Run("invalid command with domain", func(t *testing.T) {
		cmd := "curl szth.ivip.dev.com"
		exist := MatchForbiddenCommand(cmd)
		assert.Equal(t, exist, true)
	})
}
