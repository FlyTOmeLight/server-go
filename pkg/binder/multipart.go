package binder

import (
	"maps"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
)

// Gin exposes its form mapping without validation, but its multipart file mapper
// is private. Keep file assignment here so All can validate after URI/query merge.
func bindMultipart(request *http.Request, target any) error {
	if err := request.ParseMultipartForm(32 << 20); err != nil {
		return err
	}
	if _, err := bindMultipartFiles(
		reflect.ValueOf(target),
		reflect.StructField{},
		request.MultipartForm.File,
	); err != nil {
		return err
	}
	values := maps.Clone(request.MultipartForm.Value)
	for key, files := range request.MultipartForm.File {
		if len(files) > 0 {
			delete(values, key)
		}
	}
	return binding.MapFormWithTag(target, values, "form")
}

func bindMultipartFiles(
	value reflect.Value,
	field reflect.StructField,
	files map[string][]*multipart.FileHeader,
) (bool, error) {
	if !value.IsValid() || field.Tag.Get("form") == "-" {
		return false, nil
	}
	key, _, _ := strings.Cut(field.Tag.Get("form"), ",")
	if key == "" {
		key = field.Name
	}
	if headers := files[key]; len(headers) > 0 {
		return true, setMultipartFiles(value, headers)
	}
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			candidate := reflect.New(value.Type().Elem())
			set, err := bindMultipartFiles(candidate.Elem(), field, files)
			if err == nil && set {
				value.Set(candidate)
			}
			return set, err
		}
		return bindMultipartFiles(value.Elem(), field, files)
	}
	if value.Kind() != reflect.Struct {
		return false, nil
	}
	set := false
	for index := range value.NumField() {
		child := value.Type().Field(index)
		if child.PkgPath != "" && !child.Anonymous {
			continue
		}
		childSet, err := bindMultipartFiles(value.Field(index), child, files)
		if err != nil {
			return false, err
		}
		set = set || childSet
	}
	return set, nil
}

func setMultipartFiles(value reflect.Value, files []*multipart.FileHeader) error {
	switch value.Kind() {
	case reflect.Ptr:
		if _, ok := value.Interface().(*multipart.FileHeader); ok {
			value.Set(reflect.ValueOf(files[0]))
			return nil
		}
	case reflect.Struct:
		if _, ok := value.Interface().(multipart.FileHeader); ok {
			value.Set(reflect.ValueOf(*files[0]))
			return nil
		}
	case reflect.Slice, reflect.Array:
		if value.Kind() == reflect.Slice {
			value.Set(reflect.MakeSlice(value.Type(), len(files), len(files)))
		} else if value.Len() != len(files) {
			return binding.ErrMultiFileHeaderLenInvalid
		}
		for index, file := range files {
			if err := setMultipartFiles(value.Index(index), []*multipart.FileHeader{file}); err != nil {
				return err
			}
		}
		return nil
	}
	return binding.ErrMultiFileHeader
}
