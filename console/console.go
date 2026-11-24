package console

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func RegisterConsoleAPI(engine *core.Engine, config *conf.Config) {
	names := slices.Clone(config.HTTPServer.ConsoleProxyNames)
	if !slices.Contains(names, config.AppName) {
		names = append(names, config.AppName)
	}

	handler := Reverse(&config.HTTPServer, config.AppCode, engine)
	for _, name := range names {
		if name == "" {
			continue
		}
		path := "/" + name + "/api/*any"
		engine.GET(path, handler)
		engine.POST(path, handler)
		engine.PUT(path, handler)
		engine.PATCH(path, handler)
		engine.DELETE(path, handler)
	}
}

func Reverse(config *conf.HTTPServerConfig, appCode int, engine *core.Engine) core.HandlerFunc {
	proxy := NewConsoleProxy(config, appCode)
	return func(c *core.Context) {
		path, err := parseRequest(c.Request)
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}

		if isWebSocketUpgrade(c.Request) {
			if engine == nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			c.Request.URL.Path = path
			c.Request.URL.RawPath = ""
			engine.HandleContext(c)
			c.Abort()
			return
		}

		if strings.Contains(strings.ToLower(c.GetHeader("Accept")), "text/event-stream") {
			if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{}); err != nil &&
				!errors.Is(err, http.ErrNotSupported) {
				logger.FromContext(c.Request.Context()).
					Debugf(c.Request.Context(), "ProxyWriteDeadline: %v", err)
			}
		}

		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

// isWebSocketUpgrade 检查请求是否为 WebSocket 升级请求
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// NewConsoleProxy
// 由于OpenAPI和前端使用的API规范对同一个API的HTTP协议使用方式要求不同，故微服务会提供OpenAPI和FE API两套API
// ConsoleProxy能够将一个FE请求适配到OpenAPI，并将一个OpenAPI Response适配到FE响应上
// 目标地址和应用错误码由调用方传入；不读取进程全局配置。
func NewConsoleProxy(config *conf.HTTPServerConfig, appCode int) *httputil.ReverseProxy {
	listen := config.Listen
	writeTimeout := time.Duration(config.WriteTimeout) * time.Millisecond
	dialTimeout := 30 * time.Second
	if writeTimeout > 0 {
		dialTimeout = writeTimeout
	}
	idleTimeout := time.Duration(config.IdleTimeout) * time.Millisecond
	if idleTimeout <= 0 {
		idleTimeout = 90 * time.Second
	}
	return &httputil.ReverseProxy{
		Director:       func(r *http.Request) { modifyRequest(r, listen) },
		ModifyResponse: func(resp *http.Response) error { return modifyResponse(resp, appCode) },
		FlushInterval:  -1,
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: dialTimeout}).DialContext,
			ResponseHeaderTimeout: max(writeTimeout, 0),
			IdleConnTimeout:       idleTimeout,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			DisableCompression:    true,
		},
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			logger.FromContext(request.Context()).
				Errorf(request.Context(), "ProxyError method: %s, path: %s, err: %v", request.Method, request.URL.Path, err)
			writer.WriteHeader(http.StatusBadGateway)
		},
	}
}

func modifyRequest(r *http.Request, listen string) {
	r.URL.Scheme = "http"
	r.URL.Host = listen

	path, err := parseRequest(r)
	if err != nil {
		logger.FromContext(r.Context()).Errorf(r.Context(), "ProxyError url path is invalid: %s", r.URL.Path)
		return
	}
	r.URL.Path = path
	r.URL.RawPath = ""
	r.Header.Del("If-Modified-Since")
	r.Header.Del("Accept-Encoding")
	logger.FromContext(r.Context()).Debugf(r.Context(), "ProxyModifiedRequest method: %s, URL: %s", r.Method, r.URL)
}

func modifyResponse(resp *http.Response, appCode int) error {
	logger.FromContext(resp.Request.Context()).
		Debugf(resp.Request.Context(), "ProxyModifiedResponse url: %s", resp.Request.URL)
	ignoredContentType := []string{
		"application/zip",
		"application/pdf",
		"application/octet-stream",
		"text/event-stream",
	}
	contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	contentType = strings.ToLower(contentType)
	if slices.Contains(ignoredContentType, contentType) {
		logger.FromContext(resp.Request.Context()).
			Debugf(resp.Request.Context(), "ProxyModifiedResponse ignored content type %s", contentType)
		return nil
	}

	defer resp.Body.Close()
	var body io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		reader, err := gzip.NewReader(body)
		if err != nil {
			return err
		}
		defer reader.Close()
		body = reader
		resp.Header.Del("Content-Encoding")
	}
	buffer, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	// 如果buffer不是json string，则报错
	if len(buffer) > 0 && !json.Valid(buffer) {
		wrong := serrors.Error{
			Code:      "InappropriateJSON",
			Message:   string(buffer),
			RequestID: resp.Header.Get(constants.HeaderRequestID),
			Status:    http.StatusInternalServerError,
		}
		wrong.SetStatusCode(appCode)
		setResponse(resp, Response{Error: &wrong})
		return nil
	}

	// error
	if resp.StatusCode/100 != 2 && resp.StatusCode/100 != 3 {
		var wrong serrors.Error
		if len(buffer) == 0 {
			wrong.Code = http.StatusText(resp.StatusCode)
			wrong.Message = wrong.Code
		} else if err := json.Unmarshal(buffer, &wrong); err != nil {
			return err
		}
		if wrong.Status == 0 {
			wrong.Status = resp.StatusCode
		}
		if wrong.RequestID == "" {
			wrong.RequestID = resp.Header.Get(constants.HeaderRequestID)
		}
		wrong.SetStatusCode(appCode)
		setResponse(resp, Response{Error: &wrong})
		return nil
	}

	// success
	var content any
	_ = json.Unmarshal(buffer, &content)
	result := Response{
		Data: content,
		Error: &serrors.Error{
			Code:      "Success",
			RequestID: resp.Header.Get(constants.HeaderRequestID),
		},
	}
	setResponse(resp, result)
	return nil
}

func parseRequest(r *http.Request) (string, error) {
	path := r.URL.Path
	prefix, remaining, found := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	if prefix != "" && found {
		offset := len(path) - len(remaining)
		for segment := range strings.SplitSeq(remaining, "/") {
			if segment == "api" {
				return path[:offset] + "internal" + path[offset+len(segment):], nil
			}
			offset += len(segment) + 1
		}
	}
	return "", serrors.ErrorInvalidURI
}

func setResponse(resp *http.Response, content any) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(content); err != nil {
		return
	}
	buffer := bytes.TrimRight(buf.Bytes(), "\n")
	reader := bytes.NewReader(buffer)
	resp.StatusCode = http.StatusOK
	resp.Status = "200 OK"
	resp.Body = io.NopCloser(reader)
	contentLength := len(buffer)
	resp.ContentLength = int64(contentLength)
	resp.TransferEncoding = nil
	resp.Header.Set("Content-Length", strconv.Itoa(contentLength))
	resp.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Transfer-Encoding")
}
