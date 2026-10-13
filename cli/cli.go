package cli

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/FlyTOmeLight/server-go/sdk"
)

const defaultRootName = "ctl"

type CommandLine struct {
	Root     *cobra.Command
	config   *viper.Viper
	client   *sdk.Client
	log      *logger.Logger
	handlers map[string]*ApplyHandler
}

type commandLineKey struct{}

func commandLine(ctx context.Context) *CommandLine {
	line, _ := ctx.Value(commandLineKey{}).(*CommandLine)
	return line
}

// Client returns the client owned by the current command execution.
func Client(ctx context.Context) (*sdk.Client, error) {
	line := commandLine(ctx)
	if line == nil || line.client == nil {
		return nil, errors.New("CLI client has not been initialized")
	}
	return line.client, nil
}

// SetConfigFile selects the configuration file for this command line only.
func (cli *CommandLine) SetConfigFile(path string) { cli.config.SetConfigFile(path) }

func (cli *CommandLine) setConfig(cmd *cobra.Command) error {
	if cli.client != nil {
		return nil
	}
	if err := cli.config.ReadInConfig(); err != nil {
		return err
	}
	if cli.config.GetString("core.endpoint") == "" {
		return ErrEndpointNotFound
	}
	client, err := newClient(cli.config)
	if err != nil {
		return err
	}
	log, err := logger.NewLogger(logConfig(cli.config))
	if err != nil {
		return errors.Join(err, client.Close())
	}
	cli.client, cli.log = client, log
	cmd.SetContext(logger.WithContext(cmd.Context(), log))
	return nil
}

func setCommandConfig(cmd *cobra.Command) error {
	if cli := commandLine(cmd.Context()); cli != nil {
		return cli.setConfig(cmd)
	}
	return SetConfig()
}

func commandConfig(cmd *cobra.Command) *viper.Viper {
	if cli := commandLine(cmd.Context()); cli != nil {
		return cli.config
	}
	return viper.GetViper()
}

type CommandBuilder interface {
	NewCommand() *cobra.Command
}

func DefaultName() string {
	exe, err := os.Executable()
	if err != nil {
		exe = defaultRootName
	}
	base := filepath.Base(exe)
	return base
}

func New() *CommandLine {
	name := DefaultName()
	return NewCommandLineWithName(name)
}

func NewCommandLine(cmd *cobra.Command) *CommandLine {
	config := viper.New()
	name := ConfigName
	if name == "" {
		name = cmd.Name()
	}
	path := ConfigFilePath
	if path == "" {
		path = filepath.Join(os.Getenv("HOME"), "."+strings.ToLower(name), "config.yaml")
	}
	prefix := ConfigEnvPrefix
	if prefix == "" {
		prefix = strings.ToUpper(name)
	}
	config.SetEnvPrefix(prefix)
	config.SetConfigFile(path)
	config.SetConfigType("yaml")
	registered := make(map[string]*ApplyHandler, len(handlers))
	for kind, handler := range handlers {
		registered[kind] = handler
	}
	return &CommandLine{Root: cmd, config: config, handlers: registered}
}

func NewCommandLineWithName(name string) *CommandLine {
	cli := NewCommandLine(&cobra.Command{Use: name})
	cli.Root.AddCommand(NewConfigCommand())
	cli.Root.AddCommand(NewApplyCommand())
	cli.Root.AddCommand(NewVersionCommand())
	cli.Root.AddCommand(NewEncryptCommand())
	return cli
}

func (cli *CommandLine) Append(builders ...CommandBuilder) {
	for _, builder := range builders {
		before := make(map[string]*ApplyHandler, len(handlers))
		for kind, handler := range handlers {
			before[kind] = handler
		}
		cmd := builder.NewCommand()
		// Legacy builders register during NewCommand. Capture their registrations
		// now so later registrations cannot change this command line's behavior.
		for kind, handler := range handlers {
			if before[kind] != handler {
				cli.handlers[kind] = handler
			}
		}
		if ctx := cmd.Context(); ctx != nil {
			if registrations, ok := ctx.Value(applyRegistrationKey{}).(map[string]*ApplyHandler); ok {
				for kind, handler := range registrations {
					cli.handlers[kind] = handler
				}
				cmd.SetContext(nil)
			}
		}

		persistentPreRunE := cmd.PersistentPreRunE
		cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			err := setCommandConfig(cmd)
			if err != nil {
				return err
			}
			if persistentPreRunE == nil {
				return nil
			}
			return persistentPreRunE(cmd, args)
		}

		cli.Root.AddCommand(cmd)
	}
}

func (cli *CommandLine) Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := cli.ExecuteContext(ctx)
	stop()
	if err != nil {
		os.Exit(1)
	}
}

// ExecuteContext runs one command and releases its client and logger on every exit.
// Separate CommandLine instances have independent configuration and apply handlers.
// The supplied context replaces command contexts from previous executions.
func (cli *CommandLine) ExecuteContext(ctx context.Context) (err error) {
	defer func() {
		if cli.client != nil {
			err = errors.Join(err, cli.client.Close())
			cli.client = nil
		}
		if cli.log != nil {
			cli.log.Close()
			cli.log = nil
		}
	}()
	execution := context.WithValue(ctx, commandLineKey{}, cli)
	var attach func(*cobra.Command)
	attach = func(cmd *cobra.Command) {
		cmd.SetContext(execution)
		for _, child := range cmd.Commands() {
			attach(child)
		}
	}
	attach(cli.Root)
	return cli.Root.ExecuteContext(execution)
}
