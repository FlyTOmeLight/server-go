package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FlyTOmeLight/server-go/errors"
)

var handlers map[string]*ApplyHandler

type CommandSpec struct {
	Kind     string `yaml:"kind" json:"kind"`
	Metadata any    `yaml:"metadata" json:"metadata"`
}

type HandlerFunc func(args any) error

type ApplyHandler struct {
	input          any
	handler        HandlerFunc
	contextHandler func(context.Context, any) error
}

type applyRegistrationKey struct{}

// RegisterCommandApplyHandler attaches a domain's apply handler to its command.
// Append transfers registrations to the owning CommandLine before execution.
func RegisterCommandApplyHandler(cmd *cobra.Command, kind string, input any, handler func(context.Context, any) error) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	registrations, _ := ctx.Value(applyRegistrationKey{}).(map[string]*ApplyHandler)
	if registrations == nil {
		registrations = make(map[string]*ApplyHandler)
	}
	registrations[strings.ToLower(kind)] = &ApplyHandler{input: input, contextHandler: handler}
	cmd.SetContext(context.WithValue(ctx, applyRegistrationKey{}, registrations))
}

func (h *ApplyHandler) Handle(kind string, meta any) error {
	return h.HandleContext(context.Background(), kind, meta)
}

// HandleContext preserves cancellation while converting metadata for the handler.
func (h *ApplyHandler) HandleContext(ctx context.Context, kind string, meta any) error {
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

	if h.contextHandler != nil {
		return h.contextHandler(ctx, inputContent)
	}
	return h.handler(inputContent)
}

func GetApplyHandler(kind string) (*ApplyHandler, error) {
	return getApplyHandler(handlers, kind)
}

func getApplyHandler(registered map[string]*ApplyHandler, kind string) (*ApplyHandler, error) {
	k := strings.ToLower(kind)
	applyHandler, ok := registered[k]
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
