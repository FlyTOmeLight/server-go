package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"gopkg.in/yaml.v3"
)

const (
	RenderTypeJSON     RenderType = "json"
	RenderTypeYAML     RenderType = "yaml"
	RenderTypeTable    RenderType = "table"
	RenderTypeCSV      RenderType = "csv"
	RenderTypeHTML     RenderType = "html"
	RenderTypeMarkdown RenderType = "markdown"
)

type RenderType = string

func FormattedPrint(obj any, renderType RenderType) {
	if err := WriteFormatted(os.Stdout, obj, renderType); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func PrintJSON(obj any) { FormattedPrint(obj, RenderTypeJSON) }

func PrintYAML(obj any) { FormattedPrint(obj, RenderTypeYAML) }

func PrintTable(obj any) { FormattedPrint(obj, RenderTypeTable) }

func PrintCSV(obj any) { FormattedPrint(obj, RenderTypeCSV) }

func PrintHTML(obj any) { FormattedPrint(obj, RenderTypeHTML) }

func PrintMarkdown(obj any) { FormattedPrint(obj, RenderTypeMarkdown) }

// WriteFormatted reports encoding and output errors to the caller.
func WriteFormatted(writer io.Writer, obj any, renderType RenderType) error {
	var output string
	switch renderType {
	case RenderTypeYAML:
		data, err := marshalYAML(obj)
		if err != nil {
			return err
		}
		output = string(data)
	case RenderTypeTable, RenderTypeCSV, RenderTypeHTML, RenderTypeMarkdown:
		formatted, err := formattedToTable(obj)
		if err != nil {
			return err
		}
		switch renderType {
		case RenderTypeTable:
			output = formatted.Render()
		case RenderTypeCSV:
			output = formatted.RenderCSV()
		case RenderTypeHTML:
			output = formatted.RenderHTML()
		case RenderTypeMarkdown:
			output = formatted.RenderMarkdown()
		}
		if output != "" {
			output += "\n"
		}
	default:
		data, err := json.MarshalIndent(obj, "", "\t")
		if err != nil {
			return err
		}
		output = string(data)
	}
	_, err := fmt.Fprintln(writer, output)
	return err
}

func marshalYAML(obj any) (_ []byte, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			message := fmt.Sprint(failure)
			switch {
			case strings.HasPrefix(message, "cannot marshal type:"),
				strings.HasPrefix(message, "cannot have key "),
				strings.HasPrefix(message, "duplicated key "):
				err = fmt.Errorf("yaml: %s", message)
			default:
				panic(failure)
			}
		}
	}()
	return yaml.Marshal(obj)
}

func formattedToTable(obj any) (table.Writer, error) {
	buf, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if len(buf) > 0 && buf[0] == '{' {
		var row map[string]any
		if err := json.Unmarshal(buf, &row); err != nil {
			return nil, err
		}
		rows = []map[string]any{row}
	} else if err := json.Unmarshal(buf, &rows); err != nil {
		return nil, err
	}

	keySet := make(map[string]struct{})
	for _, row := range rows {
		for key := range row {
			keySet[key] = struct{}{}
		}
	}
	keys := slices.Sorted(maps.Keys(keySet))
	formatted := table.NewWriter()
	formatted.SetStyle(table.StyleRounded)
	header := make(table.Row, len(keys))
	for index, key := range keys {
		header[index] = key
	}
	formatted.AppendHeader(header)
	for _, row := range rows {
		values := make(table.Row, len(keys))
		for index, key := range keys {
			if value := row[key]; value != nil {
				values[index] = value
			} else {
				values[index] = ""
			}
		}
		formatted.AppendRow(values)
	}
	return formatted, nil
}
