package core

import (
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
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
	for k, v := range result {
		if v != nil {
			refVal := reflect.ValueOf(v)
			if refVal.Kind() == reflect.Ptr {
				if refVal.IsNil() {
					continue
				}

				va := reflect.Indirect(reflect.ValueOf(v))
				reflect.ValueOf(v).Elem().Set(va)
				stringMap[k] = fmt.Sprint(va)
			} else {
				stringMap[k] = fmt.Sprint(v)
			}
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
