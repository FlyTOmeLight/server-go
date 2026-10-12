package cli

import (
	"testing"
	"time"
)

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
