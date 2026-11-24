package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	dapr "github.com/dapr/go-sdk/client"
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

func TestOnEventWithGroupNormalizesRouteAndKeepsDefaultSubscription(t *testing.T) {
	registry := NewRegistry()
	engine := gin.New()
	handler := func(c *core.Context) { c.Status(http.StatusNoContent) }
	registry.OnEventWithGroup(nil, "/unused", nil, handler)
	(*Registry)(nil).OnEventWithGroup(engine.Group("/unused/"), "/unused", nil, handler)
	registry.OnEventWithGroup(engine.Group("/v1/"), "/second", func(subscription *Subscription) *Subscription {
		subscription.DeadLetterTopic = "failed-events"
		return nil
	}, handler)
	subscriptions := discover(t, registry.Subscribe())
	if len(subscriptions) != 1 || subscriptions[0].Route != "v1/second" ||
		subscriptions[0].PubsubName != "second" || subscriptions[0].Topic != "second" ||
		subscriptions[0].DeadLetterTopic != "failed-events" {
		t.Fatalf("unexpected discovery response: %#v", subscriptions)
	}
	writer := httptest.NewRecorder()
	engine.ServeHTTP(writer, httptest.NewRequest(http.MethodPost, "/v1/second", nil))
	if writer.Code != http.StatusNoContent {
		t.Fatalf("event route returned HTTP %d", writer.Code)
	}
}

type testEventClient struct {
	publish func(context.Context, string, string, any) error
	close   func()
}

func (client *testEventClient) PublishEvent(
	ctx context.Context,
	pubsubName, topic string,
	data any,
	_ ...dapr.PublishEventOption,
) error {
	return client.publish(ctx, pubsubName, topic, data)
}

func (client *testEventClient) Close() { client.close() }

func TestPublisherReusesClientAndClosesOnce(t *testing.T) {
	var publishes, closes int
	publisher := &Publisher{client: &testEventClient{
		publish: func(_ context.Context, pubsubName, topic string, data any) error {
			publishes++
			if pubsubName != "component" || topic != "events" || data != publishes {
				t.Errorf("unexpected publish: %q %q %v", pubsubName, topic, data)
			}
			return nil
		},
		close: func() { closes++ },
	}}
	for sequence := 1; sequence <= 2; sequence++ {
		if err := publisher.PublishEvent(t.Context(), "component", "events", sequence); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := publisher.PublishEvent(ctx, "component", "events", 3); !errors.Is(err, context.Canceled) {
		t.Fatalf("publishing with a canceled context returned %v", err)
	}
	for range 2 {
		if err := publisher.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := publisher.PublishEvent(t.Context(), "component", "events", 3); err == nil {
		t.Fatal("publish succeeded after Close")
	}
	if publishes != 2 || closes != 1 {
		t.Fatalf("client used %d times and closed %d times", publishes, closes)
	}
	var missing *Publisher
	if err := missing.Close(); err != nil {
		t.Fatal(err)
	}
	if err := missing.PublishEvent(t.Context(), "component", "events", 1); err == nil {
		t.Fatal("publish succeeded with a nil publisher")
	}
}

func TestPublisherCloseWaitsForConcurrentPublishes(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	started := make(chan struct{}, 2)
	finished := make(chan struct{}, 2)
	publisher := &Publisher{client: &testEventClient{
		publish: func(ctx context.Context, _, _ string, _ any) error {
			started <- struct{}{}
			<-ctx.Done()
			finished <- struct{}{}
			return ctx.Err()
		},
		close: func() {
			if len(finished) != 2 {
				t.Error("client closed with publishes still active")
			}
		},
	}}

	published := make(chan error, 2)
	for range 2 {
		go func() { published <- publisher.PublishEvent(ctx, "component", "events", nil) }()
	}
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("publishes did not start concurrently")
		}
	}
	closed := make(chan error, 2)
	for range 2 {
		go func() { closed <- publisher.Close() }()
	}
	cancel()
	for range 2 {
		select {
		case err := <-published:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("active publish returned %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("active publish did not stop after cancellation")
		}
		select {
		case err := <-closed:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("Close did not finish after cancellation")
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
