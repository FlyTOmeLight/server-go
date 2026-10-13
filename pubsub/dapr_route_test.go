package pubsub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestSubscriptionDiscoveryPreservesConfiguration(t *testing.T) {
	for _, deadLetterTopic := range []string{"", "failed-events"} {
		t.Run("dead-letter="+deadLetterTopic, func(t *testing.T) {
			registry := NewRegistry()
			engine := gin.New()
			registry.OnEvent(engine, "/events", func(subscription *Subscription) *Subscription {
				subscription.PubsubName = "component"
				subscription.Topic = "topic"
				subscription.DeadLetterTopic = deadLetterTopic
				return subscription
			}, func(c *core.Context) { c.Status(http.StatusNoContent) })
			subscriptions := discover(t, registry.Subscribe())
			if len(subscriptions) != 1 {
				t.Fatalf("discovery returned %d subscriptions", len(subscriptions))
			}
			actual := subscriptions[0]
			if actual.PubsubName != "component" || actual.Topic != "topic" || actual.Route != "/events" ||
				actual.DeadLetterTopic != deadLetterTopic {
				t.Fatalf("discovery changed subscription configuration: %#v", actual)
			}
		})
	}
}

func TestRegistriesOwnSeparateSubscriptions(t *testing.T) {
	first, second := NewRegistry(), NewRegistry()
	firstEngine, secondEngine := gin.New(), gin.New()
	metadata := map[string]string{MetadataRawPayload: "true"}
	first.OnEvent(firstEngine, "/first", func(subscription *Subscription) *Subscription {
		subscription.Metadata = metadata
		return subscription
	}, func(c *core.Context) { c.Status(http.StatusNoContent) })
	second.OnEventWithGroup(
		secondEngine.Group("/v1"),
		"/second",
		nil,
		func(c *core.Context) { c.Status(http.StatusNoContent) },
	)
	metadata[MetadataRawPayload] = "false"
	for _, test := range []struct {
		registry *Registry
		topic    string
		route    string
	}{
		{registry: first, topic: "first", route: "/first"},
		{registry: second, topic: "second", route: "v1/second"},
	} {
		subscriptions := discover(t, test.registry.Subscribe())
		if len(subscriptions) != 1 || subscriptions[0].Topic != test.topic ||
			subscriptions[0].Route != test.route {
			t.Fatalf("registry contains other service's subscriptions: %+v", subscriptions)
		}
		if test.registry == first && subscriptions[0].Metadata[MetadataRawPayload] != "true" {
			t.Fatal("registry retained caller-owned metadata map")
		}
	}
}

func TestRegistryConcurrentRegistrationAndDiscovery(t *testing.T) {
	registry := NewRegistry()
	// The discovery engine is already serving. Event routes are configured on a
	// different engine, which has not started serving yet.
	discovery := gin.New()
	discovery.GET("/dapr/subscribe", registry.Subscribe())
	events := gin.New()
	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			registry.OnEvent(
				events,
				fmt.Sprintf("/event/%d", i),
				nil,
				func(c *core.Context) { c.Status(http.StatusNoContent) },
			)
		}()
		go func() {
			defer wg.Done()
			writer := httptest.NewRecorder()
			discovery.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/dapr/subscribe", nil))
			if writer.Code != http.StatusOK || !json.Valid(writer.Body.Bytes()) {
				t.Errorf("invalid discovery response: %d %s", writer.Code, writer.Body.String())
			}
		}()
	}
	wg.Wait()
	if subscriptions := discover(t, registry.Subscribe()); len(subscriptions) != 30 {
		t.Fatalf("expected 30 subscriptions, got %d", len(subscriptions))
	}
}

func discover(t *testing.T, handler core.HandlerFunc) []Subscription {
	t.Helper()
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	handler(ctx)
	var subscriptions []Subscription
	if err := json.Unmarshal(writer.Body.Bytes(), &subscriptions); err != nil {
		t.Fatal(err)
	}
	return subscriptions
}
