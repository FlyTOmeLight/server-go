package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/FlyTOmeLight/server-go/sdk"
	"github.com/FlyTOmeLight/server-go/version"
)

const parameterSplitSet = "."

func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "config",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			config := commandConfig(cmd)
			if err := config.ReadInConfig(); err != nil {
				path := config.ConfigFileUsed()
				if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					return err
				}
				return config.ReadInConfig()
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {},
	}

	cmd.AddCommand(setCommand())
	cmd.AddCommand(unsetCommand())
	cmd.AddCommand(getCommand())
	return cmd
}

func getCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get all config.",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			get(commandConfig(cmd), args)
		},
		Example: DefaultName() + " config get",
	}

	return cmd
}

func setCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return set(commandConfig(cmd), args[0])
		},
		Example: DefaultName() + " config set core.endpoint=http://localhost:8080\n" +
			DefaultName() + " config set core.access_key=test-ak\n" +
			DefaultName() + " config set core.secret_key=test-sk\n",
	}

	return cmd
}

func unsetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Unset config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return unset(commandConfig(cmd), args[0])
		},
		Example: DefaultName() + " config unset core.endpoint",
	}

	return cmd
}

func get(config *viper.Viper, args []string) {
	if len(args) == 0 {
		// get all
		PrintJSON(config.AllSettings())
		return
	}

	arg := args[0]
	groupFlag, paramFlag := parseConfigParameter(arg)

	if paramFlag == "" {
		group := config.Get(groupFlag)
		PrintJSON(&group)
		return
	}

	param := config.Get(arg)
	PrintJSON(&param)
}

func set(config *viper.Viper, arg string) error {
	pair := strings.Split(arg, "=")
	if len(pair) != 2 {
		return fmt.Errorf("%s is not valid, please set parameter like core.endpoint=http://localhost:80", arg)
	}

	key := pair[0]
	value := pair[1]

	groupFlag, paramFlag := parseConfigParameter(key)
	if groupFlag == "" && paramFlag == "" {
		return fmt.Errorf("%s is not valid, please set parameter like core.endpoint=http://localhost:80", arg)
	}

	if paramFlag == "" {
		config.Set(groupFlag, value)
		return config.WriteConfig()
	}

	config.Set(key, value)
	return config.WriteConfig()
}

func unset(config *viper.Viper, arg string) error {
	groupFlag, paramFlag := parseConfigParameter(arg)

	if paramFlag == "" {
		config.Set(groupFlag, "")
		return config.WriteConfig()
	}

	config.Set(arg, "")
	return config.WriteConfig()
}

func parseConfigParameter(arg string) (string, string) {
	if strings.Contains(arg, parameterSplitSet) {
		pair := strings.Split(arg, parameterSplitSet)
		return pair[0], pair[1]
	}
	return arg, ""
}

func NewApplyCommand() *cobra.Command {
	var filenames []string
	var directory string
	cmd := &cobra.Command{
		Use: "apply",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := setCommandConfig(cmd)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if len(filenames) != 0 && directory != "" {
				return errors.New("can not use both --directory and --filename")
			}

			for _, f := range filenames {
				err = apply(cmd.Context(), f)
				if err != nil {
					return err
				}
			}

			err = walkDir(cmd.Context(), directory)
			if err != nil {
				return err
			}

			return nil
		},
	}
	cmd.Flags().StringArrayVarP(&filenames, "filename", "f", []string{}, "that contains the configuration to apply")
	cmd.Flags().
		StringVarP(&directory, "directory", "d", "", "Process a directory. This flag can't be used together with -f")
	return cmd
}

func apply(ctx context.Context, filename string) error {
	f, fileErr := os.Open(filename)
	if fileErr != nil {
		return fileErr
	}
	defer f.Close()

	succeededMap := make(map[string]int)
	failedMap := make(map[string]int)
	errs := make([]error, 0)

	d := yaml.NewDecoder(f)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var spec *CommandSpec
		err := d.Decode(&spec)
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Errorf(ctx, "decode yaml error: %v \n", err)
			errs = append(
				errs,
				serrors.NewError(
					http.StatusInternalServerError,
					"InvalidYaml",
					fmt.Sprintf("decode yaml error: %v", err),
				),
			)
			break
		}

		if spec == nil {
			continue
		}

		var registered map[string]*ApplyHandler
		if line := commandLine(ctx); line != nil {
			registered = line.handlers
		} else {
			registered = handlers
		}
		applyHandler, err := getApplyHandler(registered, spec.Kind)
		if err != nil {
			failedMap[spec.Kind]++
			logger.Errorf(ctx, "kind: %s is invalid \n", spec.Kind)
			errs = append(errs, err)
			continue
		}

		err = applyHandler.HandleContext(ctx, spec.Kind, spec.Metadata)
		if err == nil {
			succeededMap[spec.Kind]++
			continue
		}

		e := serrors.New(err)
		if e.Status == http.StatusConflict {
			succeededMap[spec.Kind]++
			continue
		}
		failedMap[spec.Kind]++
		logger.Errorf(ctx, "apply error: %v", err)
		errs = append(errs, e)
	}

	failedCount := 0
	for _, v := range failedMap {
		failedCount += v
	}

	succeededCount := 0
	for _, v := range succeededMap {
		succeededCount += v
	}

	fmt.Printf("Apply failed: %v\n", failedCount)
	PrintJSON(failedMap)
	fmt.Printf("Apply succeeded: %v\n", succeededCount)
	PrintJSON(succeededMap)

	if len(errs) > 0 {
		fmt.Println("Errors: ")
		for _, err := range errs {
			fmt.Println(err)
		}

		return errs[0]
	}

	fmt.Println("Apply Successful!")
	return nil
}

func walkDir(ctx context.Context, directory string) error {
	if directory == "" {
		return nil
	}
	return filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() &&
			(strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml")) {
			err = apply(ctx, path)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func NewVersionCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the client and server version information",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := setCommandConfig(cmd)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if output != "" && output != "yaml" && output != "json" {
				return fmt.Errorf("--output=%q must be 'yaml' or 'json' ", output)
			}

			err := getVersion(cmd, output)
			if err != nil {
				return fmt.Errorf("get ServerVersion error: %w", err)
			}
			return nil
		},
		Example: DefaultName() + " version",
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "One of 'yaml' or 'json'")
	return cmd
}

func getVersion(cmd *cobra.Command, outputFormat string) error {
	var client *sdk.Client
	if commandLine(cmd.Context()) != nil {
		var err error
		client, err = Client(cmd.Context())
		if err != nil {
			return err
		}
	} else {
		client = Config.Client
	}
	info := Version{ClientVersion: version.GetVersion(), ServerVersion: &version.VersionInfo{}}
	result, err := client.R().SetContext(cmd.Context()).SetResult(info.ServerVersion).Get("/version")
	if err := sdk.Complete(result, err); err != nil {
		return err
	}
	FormattedPrint(info, outputFormat)
	return nil
}
