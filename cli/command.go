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
	var showSecrets bool
	var revealSecrets bool
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get all config.",
		Args: func(cmd *cobra.Command, args []string) error {
			revealSecrets, showSecrets = showSecrets, false
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			config := commandConfig(cmd)
			var value any
			if len(args) == 0 {
				value = config.AllSettings()
			} else {
				key, _ := parseConfigParameter(args[0])
				if key == "" {
					return fmt.Errorf("invalid config key %q", args[0])
				}
				value = config.Get(args[0])
				if !revealSecrets && sensitiveConfigKey(args[0]) && value != nil {
					value = "[REDACTED]"
				}
			}
			if !revealSecrets {
				value = redactConfig(value)
			}
			return WriteFormatted(cmd.OutOrStdout(), value, RenderTypeJSON)
		},
		Example: DefaultName() + " config get",
	}
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "Show sensitive configuration values")
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		showSecrets = false
		if parent := cmd.Parent(); parent != nil {
			return parent.FlagErrorFunc()(cmd, err)
		}
		return err
	})
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
			DefaultName() + " config set core.api_key=test-key\n",
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

func sensitiveConfigKey(key string) bool {
	for segment := range strings.SplitSeq(strings.ToLower(key), parameterSplitSet) {
		switch segment {
		case "api_key", "secret_key", "password", "token":
			return true
		}
	}
	return false
}

func redactConfig(value any) any {
	switch items := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(items))
		for key, item := range items {
			if sensitiveConfigKey(key) {
				clean[key] = "[REDACTED]"
			} else {
				clean[key] = redactConfig(item)
			}
		}
		return clean
	case []any:
		clean := make([]any, len(items))
		for index, item := range items {
			clean[index] = redactConfig(item)
		}
		return clean
	}
	return value
}

func set(config *viper.Viper, arg string) error {
	key, value, found := strings.Cut(arg, "=")
	top, _ := parseConfigParameter(key)
	if !found || top == "" {
		return errors.New("expected a config assignment like core.endpoint=http://localhost:80")
	}
	return editConfig(config, func(mapping *yaml.Node) bool {
		setConfigKey(mapping, strings.Split(strings.ToLower(key), parameterSplitSet), value)
		return true
	})
}

func unset(config *viper.Viper, arg string) error {
	top, _ := parseConfigParameter(arg)
	if top == "" {
		return fmt.Errorf("invalid config key %q", arg)
	}
	return editConfig(config, func(mapping *yaml.Node) bool {
		return deleteConfigKey(mapping, strings.Split(arg, parameterSplitSet))
	})
}

func editConfig(config *viper.Viper, edit func(*yaml.Node) bool) error {
	filename, err := filepath.EvalSymlinks(config.ConfigFileUsed())
	if err != nil {
		return err
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return err
	}
	if len(document.Content) == 0 || document.Content[0].Tag == "!!null" {
		document = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	if document.Content[0].Kind != yaml.MappingNode {
		return errors.New("config file must contain a YAML mapping")
	}
	if !edit(document.Content[0]) {
		return nil
	}
	content, err = yaml.Marshal(&document)
	if err != nil {
		return err
	}
	if err := replaceConfigFile(filename, content); err != nil {
		return err
	}
	return config.ReadInConfig()
}

func replaceConfigFile(filename string, content []byte) error {
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(filename), ".config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(content); err != nil {
		return err
	}
	if err := file.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filename)
}

func configKeyIndex(mapping *yaml.Node, key string) int {
	if mapping.Kind != yaml.MappingNode {
		return -1
	}
	for index := 0; index < len(mapping.Content); index += 2 {
		if strings.EqualFold(mapping.Content[index].Value, key) {
			return index
		}
	}
	return -1
}

func setConfigKey(mapping *yaml.Node, path []string, value string) {
	index := configKeyIndex(mapping, strings.Join(path, parameterSplitSet))
	if index >= 0 {
		mapping.Content[index+1].SetString(value)
		return
	}
	index = configKeyIndex(mapping, path[0])
	if index < 0 {
		key := new(yaml.Node)
		key.SetString(path[0])
		index = len(mapping.Content)
		mapping.Content = append(mapping.Content, key, new(yaml.Node))
	}
	node := mapping.Content[index+1]
	if len(path) == 1 {
		node.SetString(value)
		return
	}
	if node.Kind != yaml.MappingNode {
		*node = yaml.Node{Kind: yaml.MappingNode}
	}
	setConfigKey(node, path[1:], value)
}

func deleteConfigKey(mapping *yaml.Node, path []string) bool {
	index := configKeyIndex(mapping, strings.Join(path, parameterSplitSet))
	if index >= 0 {
		mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)
		return true
	}
	if index := configKeyIndex(mapping, path[0]); index >= 0 && len(path) > 1 {
		return deleteConfigKey(mapping.Content[index+1], path[1:])
	}
	return false
}

func parseConfigParameter(arg string) (string, string) {
	for _, segment := range strings.Split(arg, parameterSplitSet) {
		if strings.TrimSpace(segment) == "" {
			return "", ""
		}
	}
	top, rest, _ := strings.Cut(arg, parameterSplitSet)
	return top, rest
}

func NewApplyCommand() *cobra.Command {
	var filenames []string
	var directory string
	cmd := &cobra.Command{
		Use: "apply",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if len(filenames) != 0 && directory != "" {
				return errors.New("can not use both --directory and --filename")
			}
			if len(filenames) == 0 && directory == "" {
				return errors.New("apply requires --filename (-f) or --directory (-d)")
			}
			return setCommandConfig(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if directory != "" {
				return walkDir(cmd.Context(), directory)
			}
			var errs []error
			for _, filename := range filenames {
				if err := cmd.Context().Err(); err != nil {
					return errors.Join(append(errs, err)...)
				}
				if err := apply(cmd.Context(), filename); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", filename, err))
				}
			}
			return errors.Join(errs...)
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
			return errors.Join(append(errs, err)...)
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
				fmt.Errorf("decode yaml: %w", err),
			)
			break
		}

		if spec == nil {
			continue
		}

		var applyHandler *ApplyHandler
		if line := commandLine(ctx); line != nil {
			applyHandler, err = getApplyHandler(line.handlers, spec.Kind)
		} else {
			applyHandler, err = GetApplyHandler(spec.Kind)
		}
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

		if serrors.IsStatusCode(err, http.StatusConflict) {
			succeededMap[spec.Kind]++
			continue
		}
		failedMap[spec.Kind]++
		logger.Errorf(ctx, "apply error: %v", err)
		errs = append(errs, err)
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

		return errors.Join(errs...)
	}

	fmt.Println("Apply Successful!")
	return nil
}

func walkDir(ctx context.Context, directory string) error {
	if directory == "" {
		return nil
	}
	var errs []error
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		if !info.IsDir() &&
			(strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml")) {
			if err := apply(ctx, path); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, err))
			}
		}
		return nil
	})
	return errors.Join(append(errs, err)...)
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
	return WriteFormatted(cmd.OutOrStdout(), info, outputFormat)
}
