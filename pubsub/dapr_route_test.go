package pubsub

import (
	"encoding/json"
	"testing"

	daprc "github.com/dapr/go-sdk/service/common"
)

func TestSubscription(t *testing.T) {
	subscription := &daprc.Subscription{
		PubsubName: "name",
		Topic:      "topic",
		Route:      "path/to/route",
	}

	bytes, err := json.Marshal(subscription)
	if err != nil {
		t.Errorf("Json serialize error: %v", err)
		return
	}

	json := string(bytes)
	if json != `{"pubsubname":"name","topic":"topic","route":"path/to/route","match":"","priority":0,"disableTopicValidation":false}` {
		t.Errorf("Json fields not match, actual: %v", json)
	}
}
