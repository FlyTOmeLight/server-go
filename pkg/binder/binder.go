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
	if b.context.Request.ContentLength != 0 {
		switch b.context.ContentType() {
		case binding.MIMEMultipartPOSTForm:
			b.MultiForm()
		default:
			b.JSON()
		}
	}

	return b.Query().Uri()
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
	b.context.Request.Body = http.MaxBytesReader(b.context.Writer, b.context.Request.Body, MaxJSONBytes)
	err := b.context.ShouldBindWith(b.req, strictJSONBinding{})
	var oversized *http.MaxBytesError
	if stderrors.As(err, &oversized) {
		err = errors.NewError(
			http.StatusRequestEntityTooLarge,
			"PayloadTooLarge",
			"JSON body exceeds size limit",
		)
	}
	if err != nil {
		b.errs = append(b.errs, err)
	}
	return b
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

type strictJSONBinding struct{}

func (strictJSONBinding) Name() string { return "json" }

func (strictJSONBinding) Bind(r *http.Request, dst any) error {
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
	return binding.Validator.ValidateStruct(dst)
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
