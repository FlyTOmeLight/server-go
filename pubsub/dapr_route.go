package pubsub

import (
	"context"
	"maps"
	"net/http"
	"os"
	"strings"
	"sync"

	dapr "github.com/dapr/go-sdk/client"
	daprc "github.com/dapr/go-sdk/service/common"
	"github.com/gin-gonic/gin"
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
func (r *Registry) Subscribe() gin.HandlerFunc {
	return func(c *gin.Context) {
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
	engine *gin.Engine,
	relativePath string,
	configHandler ConfigHandler,
	handlers ...gin.HandlerFunc,
) {
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
	group *gin.RouterGroup,
	relativePath string,
	configHandler ConfigHandler,
	handlers ...gin.HandlerFunc,
) {
	topic := ValidTopicName(relativePath)
	route := strings.TrimPrefix(group.BasePath(), "/") + "/" + strings.TrimPrefix(relativePath, "/")
	subscription := &Subscription{
		PubsubName: topic,
		Topic:      topic,
		Route:      route,
	}
	if configHandler != nil {
		subscription = configHandler(subscription)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	group.POST(relativePath, handlers...)
	r.subscriptions = append(r.subscriptions, copySubscription(*subscription))
}

func PublishEvent(ctx context.Context, pubsubName string, topic string, data any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	port := os.Getenv("DAPR_GRPC_PORT")
	if port == "" {
		port = "50001"
	}
	// The SDK's NewClient returns a process-wide singleton. Use an owned
	// connection so closing this operation cannot break other Dapr users.
	client, err := dapr.NewClientWithPort(port)
	if err != nil {
		return err
	}

	defer client.Close()
	return client.PublishEvent(ctx, pubsubName, topic, data)
}
