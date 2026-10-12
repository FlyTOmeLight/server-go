package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptCommand(t *testing.T) {
	t.Run("encrypt success", func(t *testing.T) {
		cmd := NewEncryptCommand()
		cmd.SetArgs([]string{"my-password"})
		assert.Nil(t, cmd.Execute())
	})

	t.Run("missing argument", func(t *testing.T) {
		cmd := NewEncryptCommand()
		cmd.SetArgs([]string{})
		assert.NotNil(t, cmd.Execute())
	})
}
