package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/FlyTOmeLight/server-go/conf"
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
		cmd.SetIn(strings.NewReader(""))
		assert.NotNil(t, cmd.Execute())
	})

	t.Run("pipe input", func(t *testing.T) {
		t.Setenv(conf.EnvSecretKey, "0123456789abcdef")
		for _, args := range [][]string{{}, {"--stdin"}} {
			var output bytes.Buffer
			cmd := NewEncryptCommand()
			cmd.SetArgs(args)
			cmd.SetIn(strings.NewReader("a padded secret==\r\n"))
			cmd.SetOut(&output)
			assert.NoError(t, cmd.Execute())
			assert.True(t, strings.HasPrefix(output.String(), "v1:"))
			assert.NotContains(t, output.String(), "a padded secret")
		}
	})
}
