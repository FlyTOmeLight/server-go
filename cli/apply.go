package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/FlyTOmeLight/server-go/errors"
)

var handlers map[string]*ApplyHandler

type CommandSpec struct {
	Kind     string `yaml:"kind" json:"kind"`
	Metadata any    `yaml:"metadata" json:"metadata"`
}

type HandlerFunc func(args any) error

type ApplyHandler struct {
	input   any
	handler HandlerFunc
}

func (h *ApplyHandler) Handle(kind string, meta any) error {
	// 反射实现深拷贝
	inputType := reflect.TypeOf(h.input).Elem()
	inputContent := reflect.New(inputType).Interface()

	buf, err := json.Marshal(meta)
	if err != nil {
		return errors.NewError(
			http.StatusInternalServerError,
			kind+"InvalidMetadata",
			fmt.Sprintf("json marshal error: %v", err),
		)
	}
	err = json.Unmarshal(buf, &inputContent)
	if err != nil {
		return errors.NewError(
			http.StatusInternalServerError,
			kind+"InvalidMetadata",
			fmt.Sprintf("json unmarshal error: %v", err),
		)
	}

	return h.handler(inputContent)
}

func GetApplyHandler(kind string) (*ApplyHandler, error) {
	k := strings.ToLower(kind)
	applyHandler, ok := handlers[k]
	if !ok {
		return nil, errors.NewError(
			http.StatusInternalServerError,
			k+"InvalidKind",
			fmt.Sprintf("kind: %s is invalid", k),
		)
	}
	return applyHandler, nil
}

func RegisterApplyHandler(kind string, input any, handler HandlerFunc) {
	if handlers == nil {
		handlers = make(map[string]*ApplyHandler)
	}
	kind = strings.ToLower(kind)
	handlers[kind] = &ApplyHandler{
		input:   input,
		handler: handler,
	}
}
