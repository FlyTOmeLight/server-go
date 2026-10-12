package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/FlyTOmeLight/server-go/conf"
)

func NewEncryptCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt a string, used to generate ${secret: xxx} ciphertext in config files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ciphertext, err := conf.EncryptSecret(args[0])
			if err != nil {
				return fmt.Errorf("EncryptError: %v", err)
			}
			fmt.Println(ciphertext)
			return nil
		},
		Example: DefaultName() + " encrypt my-password",
	}
	return cmd
}
