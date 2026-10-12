package console

import (
	"github.com/FlyTOmeLight/server-go/errors"
)

type Response struct {
	*errors.Error `json:",inline,omitempty"`
	Data          any `json:"data,omitempty"`
}
