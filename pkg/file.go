package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

const (
	fileExtJSON = ".json"
	fileExtTOML = ".toml"
	fileExtYAML = ".yaml"
	fileExtYML  = ".yml"
)

// ParseFile 解析文件，将文件内容转为结构体，支持toml, yaml, json格式
func ParseFile(filePath string, obj any) error {
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case fileExtJSON:
		return json.Unmarshal(buf, obj)
	case fileExtTOML:
		return toml.Unmarshal(buf, obj)
	case fileExtYAML, fileExtYML:
		return yaml.Unmarshal(buf, obj)
	default:
		return fmt.Errorf("unsupported file extension %q", ext)
	}
}

// CreateFileIfNotExist creates an empty file and any missing parent directories.
func CreateFileIfNotExist(dir string, fileName string) error {
	target := filepath.Join(dir, fileName)
	_, err := os.Stat(target)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return file.Close()
}
