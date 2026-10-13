package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

var (
	ErrorNotImplemented = NewError(
		http.StatusInternalServerError,
		"NotImplemented",
		"API 未实现",
	) // API NotImplemented.
	ErrorAccessDenied = NewError(
		http.StatusForbidden,
		"AccessDenied",
		"拒绝访问",
	) // Access denied.
	ErrorInappropriateJSON = NewError(
		http.StatusBadRequest,
		"InappropriateJSON",
		"json 格式错误",
	) // The JSON you provided was well-formed and valid, but not appropriate for this operation.
	ErrorInternalError = NewError(
		http.StatusInternalServerError,
		"InternalError",
		"服务器内部错误，请稍后重试！",
	) // We encountered an internal error. Please try again.
	ErrorInvalidHTTPRequest = NewError(
		http.StatusBadRequest,
		"InvalidHTTPRequest",
		"HTTP 请求参数错误",
	) // There was an error in the body of your HTTP request.
	ErrorInvalidURI = NewError(
		http.StatusBadRequest,
		"InvalidURI",
		"无效 URI",
	) // Could not parse the specified URI.
	ErrorMalformedJSON = NewError(
		http.StatusBadRequest,
		"MalformedJSON",
		"无效 json",
	) // The JSON you provided was not well-formed.
	ErrorInvalidVersion = NewError(
		http.StatusNotFound,
		"InvalidVersion",
		"无效 API 版本",
	) // The API version specified was invalid.
	ErrorOptInRequired = NewError(
		http.StatusForbidden,
		"OptInRequired",
		"需要订阅服务",
	) // A subscription for the service is required.
	ErrorRequestExpired = NewError(
		http.StatusBadRequest,
		"RequestExpired",
		"请求已过期",
	) // Request has expired.
	ErrorIdempotentParameterMismatch = NewError(
		http.StatusForbidden,
		"IdempotentParameterMismatch",
		"幂等参数不匹配",
	) // The request uses the same client token as a previous, but non-identical request.
	ErrorSignatureDoesNotMatch = NewError(
		http.StatusBadRequest,
		"SignatureDoesNotMatch",
		"签名不匹配",
	) // The request signature we calculated does not match the signature you provided. Check your Secret Access Key and signing method. Consult the service documentation for details.
	ErrorResourceNotFound = NewError(
		http.StatusNotFound,
		"ResourceNotFound",
		"资源不存在",
	) // The resource was not found.
	ErrorResourceIsExists = NewError(http.StatusConflict, "ResourceIsExists", "资源已存在")
	ErrorIncorrectString  = NewError(http.StatusBadRequest, "IncorrectString", "字符串值不正确")
	ErrorAPINotFound      = NewError(
		http.StatusNotFound,
		"APINotFound",
		"API 不存在",
	) // The API was not found.
)

var _ error = (*Error)(nil)

// Error 公共错误结构
type Error struct {
	// Code 错误码
	Code string `json:"code"`

	// Message 详细错误信息
	Message string `json:"message,omitempty"`

	// RequestID 请求id，接收到请求的时候由中间件自动注入
	RequestID string `json:"requestId"`

	// StatusCode 状态码，推荐6位数字
	StatusCode int `json:"statusCode"`

	// Status HTTP状态码，JSON中忽略
	Status int `json:"-"`

	cause error
}

func (e *Error) Error() string {
	// 只返回错误信息，不返回错误码和请求ID
	return e.Message
}

// Unwrap preserves the original error when adding framework error context.
func (e *Error) Unwrap() error { return e.cause }

func (e *Error) SetStatusCode(appCode int) {
	if e.StatusCode != 0 {
		return
	}
	e.StatusCode = e.Status
	if appCode != 0 {
		code, err := strconv.Atoi(fmt.Sprintf("%d%d", appCode, e.Status))
		if err == nil {
			e.StatusCode = code
		}
	}
}

func NewError(status int, code string, message string) error {
	err := &Error{
		Status:  status,
		Code:    code,
		Message: message,
	}
	return err
}

func NewEmptyParameter(parameterName string) error {
	return &Error{
		Status:  http.StatusBadRequest,
		Code:    "InvalidParameter",
		Message: "Parameter " + parameterName + " is empty",
	}
}

func NewInvalidParameter(message string) error {
	return &Error{
		Status:  http.StatusBadRequest,
		Code:    "InvalidParameter",
		Message: message,
	}
}

func New(err error) *Error { return normalize(err, false) }

func normalize(err error, debug bool) *Error {
	if err == nil {
		return nil
	}

	var e *Error
	if stderrors.As(err, &e) && e != nil {
		message := e.Message
		if e.Status == http.StatusInternalServerError {
			message = ErrorInternalError.Error()
			if debug {
				message = err.Error()
			}
		}
		return &Error{
			Status:     e.Status,
			Code:       e.Code,
			Message:    message,
			RequestID:  e.RequestID,
			StatusCode: e.StatusCode,
			cause:      err,
		}
	}
	message := ErrorInternalError.Error()
	if debug {
		message = err.Error()
	}
	return &Error{Status: http.StatusInternalServerError, Code: "InternalError", Message: message, cause: err}
}

// IsStatusCode check error's http status code equal to targetCode
func IsStatusCode(err error, targetCode int) bool {
	var e *Error
	return stderrors.As(err, &e) && e != nil && e.Status == targetCode
}

func IsCode(err error, code string) bool {
	var e *Error
	return stderrors.As(err, &e) && e != nil && e.Code == code
}

// Wrap returns a new error with wrap message
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	var e *Error
	if stderrors.As(err, &e) && e != nil {
		newError := &Error{
			Code:       e.Code,
			Message:    message + " " + err.Error(),
			RequestID:  e.RequestID,
			StatusCode: e.StatusCode,
			Status:     e.Status,
			cause:      err,
		}
		return newError
	}
	return errors.Wrap(err, message)
}

// WithMessage returns a new error with new message
func WithMessage(err error, message string) error {
	if err == nil {
		return nil
	}
	var e *Error
	if stderrors.As(err, &e) && e != nil {
		newError := &Error{
			Code:       e.Code,
			Message:    message,
			RequestID:  e.RequestID,
			StatusCode: e.StatusCode,
			Status:     e.Status,
			cause:      err,
		}
		return newError
	}
	return errors.WithMessage(err, message)
}

// ErrorHandler completes queued errors using the same policy as immediate failures.
func ErrorHandler(config *conf.Config) core.HandlerFunc {
	return func(c *core.Context) {
		if config != nil {
			c.Set("server.AppCode", config.AppCode)
			c.Set("server.Debug", config.RunMode == conf.RunModeDebug)
		}
		if autoHandleErrorPath(config, c.Request.URL.Path) {
			c.Writer = &statusCodeWriter{ResponseWriter: c.Writer}
		}
		c.Next()
		if lastErr := c.Errors.Last(); lastErr != nil {
			completeError(c, lastErr.Err)
		}
	}
}

func autoHandleErrorPath(config *conf.Config, path string) bool {
	if config == nil || !config.HTTPServer.AltoHandlerAPIError || config.HTTPServer.ConsoleEnable {
		return false
	}
	for _, prefix := range config.HTTPServer.AllowAPIPathNoHandlerAPIError {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}
	prefix := "/" + config.AppName + "/api"
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// statusCodeWriter changes the wire status while preserving Gin's write state.
type statusCodeWriter struct {
	gin.ResponseWriter
}

func (w *statusCodeWriter) WriteHeader(int) { w.ResponseWriter.WriteHeader(http.StatusOK) }

func (w *statusCodeWriter) Status() int { return http.StatusOK }

func (w *statusCodeWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func AbortWithError(c *core.Context, err error) {
	if err == nil {
		return
	}
	// Preserve failure identity even when the wire policy maps its status to 200.
	// Idempotence and the outer renderer must observe the same failure.
	_ = c.Error(err)
	completeError(c, err)
}

func completeError(c *core.Context, err error) {
	c.Abort()
	if c.Writer.Written() {
		return
	}
	logger.Errorf(c, "API Error: %+v", err)
	wrong := normalize(err, c.GetBool("server.Debug"))
	wrong.RequestID = c.GetHeader(constants.HeaderRequestID)
	wrong.SetStatusCode(c.GetInt("server.AppCode"))
	c.JSON(wrong.Status, wrong)
}

func NewResourceIsExists(message string) error {
	return &Error{
		Status:  http.StatusConflict,
		Code:    "ResourceIsExists",
		Message: message,
	}
}

// Abort sends a named HTTP failure using the framework error contract.
func Abort(c *core.Context, status int, code, message string) {
	AbortWithError(c, NewError(status, code, message))
}
