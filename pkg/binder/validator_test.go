package binder

import (
	"sync"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestRegisterDuringValidation(t *testing.T) {
	RegisterValidator()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				RegisterValidator()
				if err := validate(structLocalNameValidation{LocalName: "valid-name"}); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}

type structLowercaseAlphanumericValidation struct {
	LocalName string `json:"localName" binding:"lowercase_alphanumeric"`
}

func TestValidatorLowercaseAlphanumeric(t *testing.T) {
	RegisterValidator()

	testCases := []struct {
		desc        string
		validation  structLowercaseAlphanumericValidation
		shouldError bool
	}{
		{
			desc:       "can be empty",
			validation: structLowercaseAlphanumericValidation{LocalName: ""},
		},
		{
			desc:       "letters and numbers",
			validation: structLowercaseAlphanumericValidation{LocalName: "abcd123"},
		},
		{
			desc:       "begin with number",
			validation: structLowercaseAlphanumericValidation{LocalName: "1814ed13"},
		},
		{
			desc:        "with .",
			validation:  structLowercaseAlphanumericValidation{LocalName: "abcd.123"},
			shouldError: true,
		},
		{
			desc:        "with _",
			validation:  structLowercaseAlphanumericValidation{LocalName: "abcd_123"},
			shouldError: true,
		},
		{
			desc:        "with upper case",
			validation:  structLowercaseAlphanumericValidation{LocalName: "abcd123A"},
			shouldError: true,
		},
		{
			desc:        "too long",
			validation:  structLowercaseAlphanumericValidation{LocalName: "abcdefghij123456789abcdefghij123456789"},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validate(tc.validation)
			if err == nil == tc.shouldError {
				t.Errorf("got error: %v, shouldError: %v", err, tc.shouldError)
			}
		})
	}
}

type structCategoriesValidation struct {
	Categories []string `json:"categories" binding:"categories"`
}

func TestValidatorCategories(t *testing.T) {
	RegisterValidator()

	testCases := []struct {
		desc        string
		validation  structCategoriesValidation
		shouldError bool
	}{
		{
			desc: "with #@",
			validation: structCategoriesValidation{Categories: []string{
				"！@%23！¥@",
			}},
			shouldError: true,
		},
		{
			desc: "can be empty",
			validation: structCategoriesValidation{Categories: []string{
				"",
			}},
		},
		{
			desc:       "with chinese",
			validation: structCategoriesValidation{Categories: []string{"/一级分类/二级分类"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validate(tc.validation)
			if err == nil == tc.shouldError {
				t.Errorf("got error: %v, shouldError: %v", err, tc.shouldError)
			}
		})
	}
}

type structCategoryValidation struct {
	Category string `json:"category" binding:"category"`
}

func TestValidatorCategory(t *testing.T) {
	RegisterValidator()

	testCases := []struct {
		desc        string
		validation  structCategoryValidation
		shouldError bool
	}{
		{
			desc:       "can be empty",
			validation: structCategoryValidation{Category: ""},
		},
		{
			desc:       "with .",
			validation: structCategoryValidation{Category: "/first.cate/"},
		},
		{
			desc:       "with _",
			validation: structCategoryValidation{Category: "abcd_123"},
		},
		{
			desc:       "with -",
			validation: structCategoryValidation{Category: "abcd-123"},
		},
		{
			desc:       "with chinese",
			validation: structCategoryValidation{Category: "/一级分类/二级分类"},
		},
		{
			desc: "too long",
			validation: structCategoryValidation{Category: "abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij" +
				"123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789abcdefghij123456789"},
			shouldError: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validate(tc.validation)
			if err == nil == tc.shouldError {
				t.Errorf("got error: %v, shouldError: %v", err, tc.shouldError)
			}
		})
	}
}

type structAlphanumericValidation struct {
	LocalName string `json:"localName" binding:"alphanumeric"`
}

func TestValidatorAlphanumeric(t *testing.T) {
	RegisterValidator()

	testCases := []struct {
		desc        string
		validation  structAlphanumericValidation
		shouldError bool
	}{
		{
			desc:       "can be empty",
			validation: structAlphanumericValidation{LocalName: ""},
		},
		{
			desc:       "letters and numbers",
			validation: structAlphanumericValidation{LocalName: "abcd123"},
		},
		{
			desc:       "begin with number",
			validation: structAlphanumericValidation{LocalName: "1814ed13"},
		},
		{
			desc:        "with .",
			validation:  structAlphanumericValidation{LocalName: "abcd.123"},
			shouldError: true,
		},
		{
			desc:        "with _",
			validation:  structAlphanumericValidation{LocalName: "abcd_123"},
			shouldError: true,
		},
		{
			desc:        "with -",
			validation:  structAlphanumericValidation{LocalName: "abcd-123"},
			shouldError: true,
		},
		{
			desc:        "too long",
			validation:  structAlphanumericValidation{LocalName: "abcdefghij123456789abcdefghij123456789"},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validate(tc.validation)
			if err == nil == tc.shouldError {
				t.Errorf("got error: %v, shouldError: %v", err, tc.shouldError)
			}
		})
	}
}

type structLocalNameValidation struct {
	LocalName string `json:"localName" binding:"localname"`
}

func TestValidatorLocalName(t *testing.T) {
	RegisterValidator()

	testCases := []struct {
		desc        string
		validation  structLocalNameValidation
		shouldError bool
	}{
		{
			desc:       "can be empty",
			validation: structLocalNameValidation{LocalName: ""},
		},
		{
			desc:       "letters and numbers",
			validation: structLocalNameValidation{LocalName: "abcd123"},
		},
		{
			desc:       "begin with number",
			validation: structLocalNameValidation{LocalName: "1814ed13"},
		},
		{
			desc:        "with .",
			validation:  structLocalNameValidation{LocalName: "abcd.123"},
			shouldError: true,
		},
		{
			desc:        "with _",
			validation:  structLocalNameValidation{LocalName: "abcd_123"},
			shouldError: false,
		},
		{
			desc:        "with -",
			validation:  structLocalNameValidation{LocalName: "abcd-123"},
			shouldError: false,
		},
		{
			desc:        "too long",
			validation:  structLocalNameValidation{LocalName: "abcdefghij123456789abcdefghij123456789"},
			shouldError: true,
		},
		{
			desc:        "start with -",
			validation:  structLocalNameValidation{LocalName: "-abcd-123"},
			shouldError: true,
		},
		{
			desc:        "start with _",
			validation:  structLocalNameValidation{LocalName: "_abcd-123"},
			shouldError: true,
		},
		{
			desc:        "end with -",
			validation:  structLocalNameValidation{LocalName: "abcd-123-"},
			shouldError: true,
		},
		{
			desc:        "start with _",
			validation:  structLocalNameValidation{LocalName: "abcd-123_"},
			shouldError: true,
		},
		{
			desc:        "start end with -",
			validation:  structLocalNameValidation{LocalName: "-abcd-123-"},
			shouldError: true,
		},
		{
			desc:        "start end with _",
			validation:  structLocalNameValidation{LocalName: "_abcd-123_"},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validate(tc.validation)
			if err == nil == tc.shouldError {
				t.Errorf("got error: %v, shouldError: %v", err, tc.shouldError)
			}
		})
	}
}

type structOrderByValidation struct {
	OrderBy string `binding:"orderby"`
}

func TestValidatorOrderBy(t *testing.T) {
	RegisterValidator()

	testCases := []struct {
		desc        string
		validation  structOrderByValidation
		shouldError bool
	}{
		{
			desc:       "empty",
			validation: structOrderByValidation{OrderBy: ""},
		},
		{
			desc:       "valid",
			validation: structOrderByValidation{OrderBy: "abcd"},
		},
		{
			desc:        "with space",
			validation:  structOrderByValidation{OrderBy: "abc d"},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validate(tc.validation)
			if err == nil == tc.shouldError {
				t.Errorf("got error: %v, shouldError: %v", err, tc.shouldError)
			}
		})
	}
}

func validate(obj any) error {
	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(obj)
}
