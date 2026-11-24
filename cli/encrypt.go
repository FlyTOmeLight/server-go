package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/FlyTOmeLight/server-go/conf"
)

func NewEncryptCommand() *cobra.Command {
	var stdin bool
	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt a string, used to generate ${secret: xxx} ciphertext in config files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var plaintext string
			if len(args) == 1 {
				if stdin {
					return errors.New("use either --stdin or a positional argument")
				}
				plaintext = args[0]
			} else {
				input := cmd.InOrStdin()
				if file, ok := input.(*os.File); ok &&
					(isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())) {
					return errors.New("pipe the secret into standard input with --stdin")
				}
				secret, err := io.ReadAll(input)
				if err != nil {
					return fmt.Errorf("read secret: %w", err)
				}
				if line, found := bytes.CutSuffix(secret, []byte("\n")); found {
					secret = bytes.TrimSuffix(line, []byte("\r"))
				}
				if len(secret) == 0 {
					return errors.New("no secret supplied on stdin")
				}
				plaintext = string(secret)
			}
			ciphertext, err := conf.EncryptSecret(plaintext)
			if err != nil {
				return fmt.Errorf("EncryptError: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), ciphertext)
			return err
		},
		Example: DefaultName() + " encrypt my-password",
	}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read the secret from standard input")
	return cmd
}
