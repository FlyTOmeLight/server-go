package binder

import (
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

const (
	TagKeyPattern   = "^.{0,64}$"
	TagValuePattern = "^.{0,255}$"

	CategoryPattern              = "^[a-zA-Z0-9-_/\\.\\*\\p{Han}]{0,105}$"
	LocalnamePattern             = "^(?:[a-zA-Z0-9](?:[a-zA-Z0-9_-]{0,34}[a-zA-Z0-9])?)?$"
	AlphanumericPattern          = "^[0-9a-zA-Z]{0,36}$"
	LowercaseAlphanumericPattern = "^[0-9a-z]{0,36}$"

	tagLowercaseAlphanumeric = "lowercase_alphanumeric"
	tagCategory              = "category"
	tagCategories            = "categories"
	tagAlphanumeric          = "alphanumeric"
	tagLocalname             = "localname"
	tagOrderby               = "orderby"
	tagTags                  = "tags"
)

var (
	nameRE                  = regexp.MustCompile(`^[0-9a-zA-Z_]{1,20}$`)
	lowercaseAlphanumericRE = regexp.MustCompile(LowercaseAlphanumericPattern)
	alphanumericRE          = regexp.MustCompile(AlphanumericPattern)
	localnameRE             = regexp.MustCompile(LocalnamePattern)
	orderbyRE               = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	idRE                    = regexp.MustCompile(`^[a-z0-9-]{9,12}$`)
	categoryRE              = regexp.MustCompile(CategoryPattern)
	tagKeyRE                = regexp.MustCompile(TagKeyPattern)
	tagValueRE              = regexp.MustCompile(TagValuePattern)
)

func name(fl validator.FieldLevel) bool {
	v, ok := fl.Field().Interface().(string)
	return ok && (v == "" || nameRE.MatchString(v))
}

func lowercaseAlphanumeric(fl validator.FieldLevel) bool {
	v, ok := fl.Field().Interface().(string)
	return ok && lowercaseAlphanumericRE.MatchString(v)
}

func alphanumeric(fl validator.FieldLevel) bool {
	v, ok := fl.Field().Interface().(string)
	return ok && alphanumericRE.MatchString(v)
}

func localname(fl validator.FieldLevel) bool {
	v, ok := fl.Field().Interface().(string)
	return ok && localnameRE.MatchString(v)
}

func orderby(fl validator.FieldLevel) bool {
	v, ok := fl.Field().Interface().(string)
	return ok && (v == "" || orderbyRE.MatchString(v))
}

func id(fl validator.FieldLevel) bool {
	v, ok := fl.Field().Interface().(string)
	if !ok {
		// id字段必须为string类型
		return false
	}

	if v == "" {
		// id字段可为空
		return true
	}

	// 增加前缀判断
	prefix := fl.Param()
	if !strings.HasPrefix(v, prefix) {
		return false
	}

	return idRE.MatchString(v)
}

func category(fl validator.FieldLevel) bool {
	v, ok := fl.Field().Interface().(string)
	return ok && categoryRE.MatchString(v)
}

func categories(fl validator.FieldLevel) bool {
	v, ok := fl.Field().Interface().([]string)
	if !ok {
		return false
	}

	for _, s := range v {
		if !categoryRE.MatchString(s) {
			return false
		}
	}

	return true
}

func tag(fl validator.FieldLevel) bool {
	var tags map[string][]string

	switch v := fl.Field().Interface().(type) {
	case map[string][]string:
		tags = v
	case map[string]string:
		tags = make(map[string][]string, len(v))
		for k, val := range v {
			tags[k] = []string{val}
		}
	default:
		return false
	}

	for k, vs := range tags {
		if !tagKeyRE.MatchString(k) {
			return false
		}
		for _, v := range vs {
			if !tagValueRE.MatchString(v) {
				return false
			}
		}
	}
	return true
}

var registerOnce sync.Once

// RegisterValidator installs process-wide Gin validators before serving requests.
// A router reload must not rewrite the validator's maps while binding requests.
func RegisterValidator() {
	registerOnce.Do(registerValidator)
}

func registerValidator() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	_ = v.RegisterValidation("name", name)
	_ = v.RegisterValidation(tagLocalname, localname)
	_ = v.RegisterValidation(tagOrderby, orderby)
	_ = v.RegisterValidation("id", id)
	_ = v.RegisterValidation(tagAlphanumeric, alphanumeric)
	_ = v.RegisterValidation(tagCategory, category)
	_ = v.RegisterValidation(tagCategories, categories)
	_ = v.RegisterValidation(tagLowercaseAlphanumeric, lowercaseAlphanumeric)
	_ = v.RegisterValidation(tagTags, tag)
}
