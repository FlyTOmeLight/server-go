package binder

import (
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type HTTPBinder struct {
	req     any
	context *core.Context
	errs    []error
}

func (b *HTTPBinder) Error() error {
	if len(b.errs) == 0 {
		return nil
	}
	return combineErrors(b.errs)
}

func (b *HTTPBinder) All() *HTTPBinder {
	// Each source maps into the same request before validating the complete object.
	// URI values take precedence over query values, which take precedence over the body.
	if b.context.Request.ContentLength != 0 {
		var err error
		switch b.context.ContentType() {
		case binding.MIMEMultipartPOSTForm:
			err = bindMultipart(b.context.Request, b.req)
		case binding.MIMEPOSTForm:
			err = b.context.Request.ParseForm()
			if err == nil {
				err = binding.MapFormWithTag(b.req, b.context.Request.PostForm, "form")
			}
		default:
			err = b.decodeJSON()
		}
		if err != nil {
			b.errs = append(b.errs, err)
		}
	}
	if err := binding.MapFormWithTag(b.req, b.context.Request.URL.Query(), "form"); err != nil {
		b.errs = append(b.errs, err)
	}
	params := make(map[string][]string, len(b.context.Params))
	for _, param := range b.context.Params {
		params[param.Key] = []string{param.Value}
	}
	if err := binding.MapFormWithTag(b.req, params, "uri"); err != nil {
		b.errs = append(b.errs, err)
	}
	if len(b.errs) == 0 {
		if err := binding.Validator.ValidateStruct(b.req); err != nil {
			b.errs = append(b.errs, err)
		}
	}
	return b
}

func (b *HTTPBinder) Header() *HTTPBinder {
	err := b.context.ShouldBindHeader(b.req)
	if err != nil {
		b.errs = append(b.errs, err)
	}
	return b
}

// nolint
func (b *HTTPBinder) Uri() *HTTPBinder {
	err := b.context.ShouldBindUri(b.req)
	if err != nil {
		b.errs = append(b.errs, err)
	}
	return b
}

func (b *HTTPBinder) Form() *HTTPBinder {
	err := b.context.ShouldBindWith(b.req, binding.Form)
	if err != nil {
		b.errs = append(b.errs, err)
	}
	return b
}

func (b *HTTPBinder) MultiForm() *HTTPBinder {
	err := b.context.ShouldBindWith(b.req, binding.FormMultipart)
	if err != nil {
		b.errs = append(b.errs, err)
	}
	return b
}

func (b *HTTPBinder) Query() *HTTPBinder {
	err := b.context.ShouldBindQuery(b.req)
	if err != nil {
		b.errs = append(b.errs, err)
	}
	return b
}

func (b *HTTPBinder) JSON() *HTTPBinder {
	err := b.decodeJSON()
	if err == nil {
		err = binding.Validator.ValidateStruct(b.req)
	}
	if err != nil {
		b.errs = append(b.errs, err)
	}
	return b
}

func (b *HTTPBinder) decodeJSON() error {
	b.context.Request.Body = http.MaxBytesReader(b.context.Writer, b.context.Request.Body, MaxJSONBytes)
	err := decodeJSON(b.context.Request, b.req)
	var oversized *http.MaxBytesError
	if stderrors.As(err, &oversized) {
		err = errors.NewError(
			http.StatusRequestEntityTooLarge,
			"PayloadTooLarge",
			"JSON body exceeds size limit",
		)
	}
	return err
}

func Bind(c *core.Context, req any) *HTTPBinder {
	return &HTTPBinder{
		context: c,
		req:     req,
		errs:    make([]error, 0),
	}
}

func combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	for _, err := range errs {
		var detail *errors.Error
		if stderrors.As(err, &detail) {
			return detail
		}
	}
	messageSet := make(map[string]struct{})
	for _, err := range errs {
		if err == nil {
			continue
		}

		switch e := err.(type) {
		case validator.ValidationErrors:
			for _, fe := range e {
				messageSet[fieldError(fe)] = struct{}{}
			}
		default:
			messageSet[err.Error()] = struct{}{}
		}
	}

	var messages string
	for m := range messageSet {
		if messages != "" {
			messages += "\n"
		}
		messages += m
	}

	return errors.NewInvalidParameter(messages)
}

func fieldError(fe validator.FieldError) string {
	if message := translate(fe.Tag()); message != "" {
		return message
	}
	return fe.Error()
}

func translate(tag string) string {
	return map[string]string{
		tagLocalname: tagLocalname + " should match regular expression " + LocalnamePattern,
		tagOrderby:   tagOrderby + " contains invalid character",
	}[tag]
}

// MaxJSONBytes bounds JSON input consistently across HTTP services.
const MaxJSONBytes = 1 << 20

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err != nil {
			return err
		}
		return stderrors.New("expected one JSON document")
	}
	return nil
}

// BindJSON binds one bounded JSON object and writes a framework error on failure.
func BindJSON(c *core.Context, dst any) bool {
	if err := Bind(c, dst).JSON().Error(); err != nil {
		errors.AbortWithError(c, err)
		return false
	}
	return true
}

// QueryLimit reads a positive query limit using the caller's pagination policy.
func QueryLimit(c *core.Context, fallback, maximum int) (int, bool) {
	value, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(fallback)))
	if err != nil || value < 1 || value > maximum {
		errors.AbortWithError(c, errors.NewInvalidParameter("invalid page size"))
		return 0, false
	}
	return value, true
}
