package pubsub

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"os"
	"strings"
	"sync"

	dapr "github.com/dapr/go-sdk/client"
	daprc "github.com/dapr/go-sdk/service/common"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

const (
	MetadataRawPayload = "rawPayload"
	PubSubKey          = "pubsub.Subscription"
)

var topicAntiPattern = strings.NewReplacer(
	"/", "__",
	":", "_",
	"?", "_",
)

type Subscription = daprc.Subscription

type ConfigHandler func(*Subscription) *Subscription

// Registry owns the subscriptions for one service. Configure event routes before
// serving the event engine; Gin does not support route changes while serving.
type Registry struct {
	mu            sync.RWMutex
	subscriptions []Subscription
}

func NewRegistry() *Registry {
	return &Registry{subscriptions: make([]Subscription, 0)}
}

// ValidTopicName returns a valid name in case input is a path
func ValidTopicName(topic string) string {
	t := strings.TrimPrefix(topic, "/")
	return topicAntiPattern.Replace(t)
}

// Subscribe returns the Dapr discovery handler for this registry.
func (r *Registry) Subscribe() core.HandlerFunc {
	return func(c *core.Context) {
		r.mu.RLock()
		subscriptions := make([]Subscription, len(r.subscriptions))
		for i, subscription := range r.subscriptions {
			subscriptions[i] = copySubscription(subscription)
		}
		r.mu.RUnlock()
		c.JSON(http.StatusOK, subscriptions)
	}
}

func copySubscription(subscription Subscription) Subscription {
	subscription.Metadata = maps.Clone(subscription.Metadata)
	return subscription
}

// OnEvent subscribe topic on relativePath.
//
// Example:
//
//	registry := pubsub.NewRegistry()
//	registry.OnEvent(engine, "path_of_api", nil, ReceiveEvent)
func (r *Registry) OnEvent(
	engine *core.Engine,
	relativePath string,
	configHandler ConfigHandler,
	handlers ...core.HandlerFunc,
) {
	if engine == nil {
		return
	}
	r.OnEventWithGroup(&engine.RouterGroup, relativePath, configHandler, handlers...)
}

// OnEventWithGroup registers an event route and its Dapr subscription for a group.
//
// Example:
//
//	g := engine.Group("/v1/datasets")
//	g.POST("", Create)
//	g.GET("", List)
//	registry.OnEventWithGroup(g, "path_of_api", nil, ReceiveEvent)
func (r *Registry) OnEventWithGroup(
	group *core.RouterGroup,
	relativePath string,
	configHandler ConfigHandler,
	handlers ...core.HandlerFunc,
) {
	if r == nil || group == nil {
		return
	}
	topic := ValidTopicName(relativePath)
	route := strings.Trim(group.BasePath(), "/") + "/" + strings.TrimPrefix(relativePath, "/")
	subscription := &Subscription{
		PubsubName: topic,
		Topic:      topic,
		Route:      route,
	}
	if configHandler != nil {
		if configured := configHandler(subscription); configured != nil {
			subscription = configured
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	group.POST(relativePath, handlers...)
	r.subscriptions = append(r.subscriptions, copySubscription(*subscription))
}

type eventClient interface {
	PublishEvent(ctx context.Context, pubsubName, topic string, data any, opts ...dapr.PublishEventOption) error
	Close()
}

// Publisher owns a Dapr connection and supports concurrent publishing.
type Publisher struct {
	mu     sync.RWMutex
	client eventClient
}

// NewPublisher creates a publisher using DAPR_GRPC_PORT or port 50001.
func NewPublisher() (*Publisher, error) {
	port := os.Getenv("DAPR_GRPC_PORT")
	if port == "" {
		port = "50001"
	}
	return NewPublisherWithPort(port)
}

// NewPublisherWithPort creates a publisher with its own connection to the local sidecar.
func NewPublisherWithPort(port string) (*Publisher, error) {
	client, err := dapr.NewClientWithPort(port)
	if err != nil {
		return nil, err
	}
	return &Publisher{client: client}, nil
}

// PublishEvent sends an event through the publisher's connection.
func (p *Publisher) PublishEvent(ctx context.Context, pubsubName, topic string, data any) error {
	if p == nil {
		return errors.New("nil Dapr publisher")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.client == nil {
		return errors.New("Dapr publisher is closed")
	}
	return p.client.PublishEvent(ctx, pubsubName, topic, data)
}

// Close waits for active publishes to finish and closes the connection.
func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		p.client.Close()
		p.client = nil
	}
	return nil
}

// PublishEvent sends one event. Use NewPublisher to reuse a connection for frequent publishing.
func PublishEvent(ctx context.Context, pubsubName, topic string, data any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	publisher, err := NewPublisher()
	if err != nil {
		return err
	}
	defer publisher.Close()
	return publisher.PublishEvent(ctx, pubsubName, topic, data)
}
