package cli

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWriteFormattedAlignsRowsAndReportsErrors(t *testing.T) {
	rows := []map[string]any{{"c": "third", "a": "first"}, {"b": "second"}}
	var output bytes.Buffer
	require.NoError(t, WriteFormatted(&output, rows, RenderTypeCSV))
	require.Equal(t, "a,b,c\nfirst,,third\n,second,\n\n", output.String())

	for _, format := range []RenderType{RenderTypeJSON, RenderTypeTable, RenderTypeYAML} {
		output.Reset()
		require.Error(t, WriteFormatted(&output, func() {}, format))
		require.Empty(t, output.String())
	}
	require.ErrorIs(t, WriteFormatted(failingOutput{}, rows, RenderTypeJSON), errTestOutput)
}

var errTestOutput = errors.New("output failed")

type failingOutput struct{}

func (failingOutput) Write([]byte) (int, error) { return 0, errTestOutput }

func TestFormattedPrint(t *testing.T) {
	t.Run("singlelayer", func(t *testing.T) {
		type singlelayer struct {
			ID          string    `json:"id"`
			Name        string    `json:"name"`
			DisplayName string    `json:"displayName"`
			Description string    `json:"description"`
			CreatedAt   time.Time `json:"createdAt"`
		}

		sl := singlelayer{
			ID:          "test-id",
			Name:        "test-name",
			DisplayName: "test-displayName",
			Description: "test-description",
			CreatedAt:   time.Now(),
		}

		FormattedPrint(&sl, RenderTypeJSON)
		FormattedPrint(&sl, RenderTypeYAML)
		FormattedPrint(&sl, RenderTypeTable)
		FormattedPrint(&sl, RenderTypeCSV)
		FormattedPrint(&sl, RenderTypeHTML)
		FormattedPrint(&sl, RenderTypeMarkdown)

		PrintJSON(&sl)
		PrintYAML(&sl)
		PrintTable(&sl)
		PrintCSV(&sl)
		PrintHTML(&sl)
		PrintMarkdown(&sl)
	})

	t.Run("multilayer", func(t *testing.T) {
		type secondLayer struct {
			Number    int       `json:"number"`
			Array     []string  `json:"array"`
			CreatedAt time.Time `json:"createdAt"`
		}
		type multilayer struct {
			String      string            `json:"str"`
			Map         map[string]string `json:"map"`
			SecondLayer secondLayer       `json:"secondLayer"`
		}

		ml := multilayer{
			String: "test-str",
			Map:    map[string]string{"test-map-key": "test-map-value"},
			SecondLayer: secondLayer{
				Number:    1,
				Array:     []string{"obj1", "obj2"},
				CreatedAt: time.Now(),
			},
		}

		FormattedPrint(&ml, RenderTypeJSON)
		FormattedPrint(&ml, RenderTypeYAML)
		FormattedPrint(&ml, RenderTypeTable)
		FormattedPrint(&ml, RenderTypeCSV)
		FormattedPrint(&ml, RenderTypeHTML)
		FormattedPrint(&ml, RenderTypeMarkdown)

		PrintJSON(&ml)
		PrintYAML(&ml)
		PrintTable(&ml)
		PrintCSV(&ml)
		PrintHTML(&ml)
		PrintMarkdown(&ml)
	})
}
