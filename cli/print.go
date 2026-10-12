package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/gookit/goutil/arrutil"
	"github.com/gookit/goutil/strutil"
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
	switch renderType {
	case RenderTypeJSON:
		PrintJSON(obj)
	case RenderTypeYAML:
		PrintYAML(obj)
	case RenderTypeTable:
		PrintTable(obj)
	case RenderTypeCSV:
		PrintCSV(obj)
	case RenderTypeHTML:
		PrintHTML(obj)
	case RenderTypeMarkdown:
		PrintMarkdown(obj)
	default:
		PrintJSON(obj)
	}
}

func PrintJSON(obj any) {
	out := formattedToJSON(obj)
	fmt.Println(out)
}

func PrintYAML(obj any) {
	out := formattedToYAML(obj)
	fmt.Println(out)
}

func PrintTable(obj any) {
	t := formattedToTable(obj)
	t.Render()
	fmt.Println()
}

func PrintCSV(obj any) {
	t := formattedToTable(obj)
	t.RenderCSV()
	fmt.Println()
}

func PrintHTML(obj any) {
	t := formattedToTable(obj)
	t.RenderHTML()
	fmt.Println()
}

func PrintMarkdown(obj any) {
	t := formattedToTable(obj)
	t.RenderMarkdown()
	fmt.Println()
}

func formattedToJSON(obj any) string {
	bs, _ := json.Marshal(obj)
	var out bytes.Buffer
	_ = json.Indent(&out, bs, "", "\t")
	return out.String()
}

func formattedToYAML(obj any) string {
	out, _ := yaml.Marshal(obj)
	return string(out)
}

func formattedToTable(obj any) table.Writer {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.SetOutputMirror(os.Stdout)

	buf, _ := json.Marshal(obj)

	var objList []map[string]any
	err := json.Unmarshal(buf, &objList)
	if err != nil {
		var objMap map[string]any
		_ = json.Unmarshal(buf, &objMap)
		objList = append(objList, objMap)
	}

	for i, item := range objList {
		keys, values := row(item)
		if i == 0 {
			t.AppendHeader(keys)
		}
		t.AppendRow(values)
	}
	t.AppendSeparator()

	return t
}

func row(m map[string]any) ([]any, []any) {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var values []string
	for _, k := range keys {
		v, _ := strutil.ToString(m[k])
		values = append(values, v)
	}

	k, _ := arrutil.AnyToSlice(keys)
	v, _ := arrutil.AnyToSlice(values)
	return k, v
}
