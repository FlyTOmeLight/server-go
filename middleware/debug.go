package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/internal/httpbody"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

const (
	maxLength = 8192
)

func shouldLogBody(c *core.Context) bool {
	// Usually chunked or compressed, not readable
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Transfer-Encoding
	transferEncoding := c.Request.Header.Get("Transfer-Encoding")
	if transferEncoding != "" {
		return false
	}

	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Range_requests
	hRange := c.Request.Header.Get("Range")
	if hRange != "" {
		return false
	}

	contentType := c.Request.Header.Get(constants.HeaderContentType)
	if binaryMIME(contentType) {
		return false
	}
	// SDK request header does not contain application/json
	if contentType == "" {
		return true
	}

	// API request body is json
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types
	if strings.Contains(contentType, "application/json") {
		return true
	}

	// https://stackoverflow.com/questions/4007969/application-x-www-form-urlencoded-or-multipart-form-data
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		return true
	}

	if strings.Contains(contentType, "text/plain") {
		return true
	}

	return false
}

func noLog(c *core.Context, prefixFilters []string, suffixFilters []string) bool {
	for _, prefix := range prefixFilters {
		if strings.HasPrefix(c.Request.RequestURI, prefix) {
			return true
		}
	}
	for _, suffix := range suffixFilters {
		if strings.HasSuffix(c.Request.URL.Path, suffix) {
			return true
		}
	}
	return false
}

type bodyPreview struct {
	io.ReadCloser
	buffer bytes.Buffer
}

func (preview *bodyPreview) Read(data []byte) (int, error) {
	read, err := preview.ReadCloser.Read(data)
	remaining := maxLength*utf8.UTFMax - preview.buffer.Len()
	preview.buffer.Write(data[:min(read, remaining)])
	return read, err
}

func Debug(prefixFilters []string, suffixFilters []string) core.HandlerFunc {
	return func(c *core.Context) {
		if noLog(c, prefixFilters, suffixFilters) {
			c.Next()
			return
		}

		var rawRequest []byte
		var preview *bodyPreview
		if shouldLogBody(c) {
			if httpbody.IsJSON(c) {
				var err error
				rawRequest, err = httpbody.Read(c)
				if err != nil {
					errors.AbortWithError(c, err)
					return
				}
			} else if c.Request.Body != nil {
				preview = &bodyPreview{ReadCloser: c.Request.Body}
				c.Request.Body = preview
			}
		}

		var writer core.BufferedWriter
		// 为了输出response，需要将response缓存一份，这里为了和幂等 middleware 合并使用，需要封装为core.BufferedWriter，并且需要避免多次封装
		if w, ok := c.Writer.(core.BufferedWriter); !ok {
			bufferedWriter := core.BufferedWriter{
				Buf:            bytes.NewBufferString(""),
				ResponseWriter: c.Writer,
				Limit:          maxLength,
			}
			c.Writer = bufferedWriter
			writer = bufferedWriter
		} else {
			writer = w
		}

		startTime := time.Now() // 请求开始时间

		c.Next()

		endTime := time.Now()

		reqHeader := header("> ", &c.Request.Header)
		var reqBody string
		if rawRequest != nil {
			var buf bytes.Buffer
			if err := json.Compact(&buf, rawRequest); err == nil {
				reqBody = buf.String()
			}
		} else if preview != nil {
			reqBody = preview.buffer.String()
		}

		statusCode := c.Writer.Status()
		h := c.Writer.Header()
		respHeader := header("< ", &h)
		respBody := ""
		contentType := c.Writer.Header().Get("Content-Type")
		if !binaryMIME(contentType) {
			respBody = strings.Trim(writer.Buf.String(), constants.LineBreak)
		}

		message := fmt.Sprintf(
			"\nlatency: %v, [C]%s --> [S]%s  %s\n%s %s\n%s \n> %s\n=================\n%d %s\n%s\n< %s",
			endTime.Sub(startTime),
			c.ClientIP(),
			c.Request.Host,
			c.Request.Proto,
			c.Request.Method,
			c.Request.RequestURI,
			reqHeader,
			body(reqBody),
			statusCode,
			http.StatusText(statusCode),
			respHeader,
			body(respBody),
		)

		log(c, message, statusCode)
	}
}

func log(ctx context.Context, message string, statusCode int) {
	switch statusCode / 100 { // reduce 2xx status to 2
	case 1:
		logger.FromContext(ctx).Infof(ctx, "%s", message)
	case 2:
		logger.FromContext(ctx).Infof(ctx, "%s", message)
	case 3:
		logger.FromContext(ctx).Infof(ctx, "%s", message)
	case 4:
		logger.FromContext(ctx).Warnf(ctx, "%s", message)
	case 5:
		logger.FromContext(ctx).Errorf(ctx, "%s", message)
	default:
		logger.FromContext(ctx).Errorf(ctx, "%s", message)
	}
}

func body(body string) string {
	body = strings.ToValidUTF8(body, "")
	count := 0
	for index := range body {
		if count == maxLength {
			return body[:index]
		}
		count++
	}
	return body
}

func binaryMIME(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch mediaType {
	case "application/octet-stream", "application/pdf", "application/zip":
		return true
	}
	return strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(mediaType, "video/") ||
		strings.HasPrefix(mediaType, "audio/")
}

func header(direction string, h *http.Header) string {
	headers := make([]string, 0, len(*h))
	for k, vs := range *h {
		switch strings.ToLower(k) {
		case "authorization", "cookie", "set-cookie", "x-api-key", "proxy-authorization":
			headers = append(headers, direction+k+"=[REDACTED]")
			continue
		}
		for _, v := range vs {
			headers = append(headers, direction+k+"="+v)
		}
	}
	return strings.Join(headers, "\n")
}
