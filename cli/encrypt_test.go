package cli

import (
	"testing"

	"github.com/FlyTOmeLight/server-go/conf"

	"github.com/stretchr/testify/assert"
)

func TestEncryptCommand(t *testing.T) {
	t.Run("encrypt success", func(t *testing.T) {
		t.Setenv(conf.EnvSecretKey, "0123456789abcdef")
		cmd := NewEncryptCommand()
		cmd.SetArgs([]string{"my-password"})
		assert.Nil(t, cmd.Execute())
	})

	t.Run("missing key", func(t *testing.T) {
		t.Setenv(conf.EnvSecretKey, "")
		cmd := NewEncryptCommand()
		cmd.SetArgs([]string{"my-password"})
		assert.ErrorContains(t, cmd.Execute(), conf.EnvSecretKey)
	})

	t.Run("missing argument", func(t *testing.T) {
		cmd := NewEncryptCommand()
		cmd.SetArgs([]string{})
		assert.NotNil(t, cmd.Execute())
	})
}
