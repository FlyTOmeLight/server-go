package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateFileIfNotExist(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dir      string
		fileName string
	}{
		{name: "existing parent", dir: ".", fileName: "config.json"},
		{name: "missing parents", dir: filepath.Join("new", "nested"), fileName: "config.json"},
		{name: "nested filename", dir: "new", fileName: filepath.Join("nested", "config.json")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), tc.dir)
			path := filepath.Join(dir, tc.fileName)
			require.NoError(t, CreateFileIfNotExist(dir, tc.fileName))
			contents, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Empty(t, contents)

			const saved = "existing content"
			require.NoError(t, os.WriteFile(path, []byte(saved), 0o644))
			require.NoError(t, CreateFileIfNotExist(dir, tc.fileName))
			contents, err = os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, saved, string(contents))
		})
	}

	t.Run("parent is a file", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(parent, nil, 0o644))
		require.Error(t, CreateFileIfNotExist(parent, "child"))
	})
}

func TestParseFile(t *testing.T) {
	for _, tc := range []struct {
		name     string
		contents string
		wantErr  bool
	}{
		{name: "config.JSON", contents: `{"name":"example"}`},
		{name: "config.Toml", contents: `name = "example"`},
		{name: "config.YAML", contents: "name: example"},
		{name: "config.YmL", contents: "name: example"},
		{name: "config.txt", contents: `{"name":"example"}`, wantErr: true},
		{name: "config", contents: `{"name":"example"}`, wantErr: true},
		{name: "invalid.json", contents: `{"name":`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name)
			require.NoError(t, os.WriteFile(path, []byte(tc.contents), 0o644))
			var config struct {
				Name string `json:"name" toml:"name" yaml:"name"`
			}
			err := ParseFile(path, &config)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "example", config.Name)
		})
	}
}
