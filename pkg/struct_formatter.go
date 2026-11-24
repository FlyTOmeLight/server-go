package core

import (
	"fmt"
	"reflect"

	"github.com/go-viper/mapstructure/v2"
)

const (
	RenderTypeJSON RenderType = iota
	RenderTypeTable
	RenderTypeCSV
	RenderTypeHTML
	RenderTypeMarkdown
)

type RenderType int

func JSONToMap(obj any) map[string]any {
	var result map[string]any
	_ = StructDecode(obj, &result, "json")
	return result
}

func FormToStringMap(obj any) map[string]string {
	var result map[string]any
	_ = StructDecode(obj, &result, "form")

	stringMap := make(map[string]string, len(result))
	for key, field := range result {
		value := reflect.Indirect(reflect.ValueOf(field))
		if value.IsValid() {
			stringMap[key] = fmt.Sprint(value)
		}
	}
	return stringMap
}

func StructDecode(in any, out any, tag string) error {
	cfg := mapstructure.DecoderConfig{
		TagName: tag,
		Result:  out,
	}

	decoder, err := mapstructure.NewDecoder(&cfg)
	if err != nil {
		return err
	}

	return decoder.Decode(in)
}
