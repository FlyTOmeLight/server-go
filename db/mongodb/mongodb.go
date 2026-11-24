package mongodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FlyTOmeLight/server-go/sdk"
)

type Client struct {
	*mongo.Client
	config            *Config
	collectionMu      sync.RWMutex
	defaultCollection *mongo.Collection
}

func (c *Client) Config() *Config {
	config := *c.config
	return &config
}

// replaceSpecialCharacters -- . ? + * | { } [ ] ( ) < > " \ @ #
var replaceSpecialCharacters = strings.NewReplacer(
	`.`, `\.`, `?`, `\?`, `+`, `\+`, `*`, `\*`, `|`, `\|`, `{`, `\{`,
	`}`, `\}`, `[`, `\[`, `]`, `\]`, `(`, `\(`, `)`, `\)`, `<`, `\<`,
	`>`, `\>`, `"`, `\"`, `\`, `\\`, `@`, `\@`, `#`, `\#`, `^`, `\^`, `$`, `\$`)

type DB = Client

func NewClientWithConfig(ctx context.Context, config *Config) (*DB, error) {
	if config == nil {
		return nil, errors.New("mongodb config is nil")
	}
	if err := config.check(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	conn, err := mongo.Connect(ctx, &options.ClientOptions{
		Hosts: []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		Auth: &options.Credential{
			Username: config.User,
			Password: config.Password,
		},
	})
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(ctx, nil); err != nil {
		return nil, errors.Join(err, disconnectAfterInitFailure(ctx, conn))
	}

	cfg := *config
	client := &Client{Client: conn, config: &cfg}
	client.SetDefaultCollection(cfg.Collection)
	return client, nil
}

func disconnectAfterInitFailure(ctx context.Context, client *mongo.Client) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return client.Disconnect(cleanupCtx)
}

func (c *DB) NewCollection(name string) *mongo.Collection {
	return c.Database(c.config.DB).Collection(name)
}

func (c *DB) GetDefaultCollection() *mongo.Collection {
	c.collectionMu.RLock()
	defer c.collectionMu.RUnlock()
	return c.defaultCollection
}

func (c *DB) SetDefaultCollection(name string) {
	collection := c.NewCollection(name)
	c.collectionMu.Lock()
	c.defaultCollection = collection
	c.collectionMu.Unlock()
}

func (c *DB) GetFindOptions(pageRequest sdk.PagingRequest) (*options.FindOptions, error) {
	var orders options.FindOptions

	orders.SetLimit(int64(pageRequest.GetPageSize()))
	orders.SetSkip(int64((pageRequest.GetPageNo() - 1) * pageRequest.GetPageSize()))

	var sort bson.D
	for _, orderJSON := range pageRequest.Orders {
		order, err := parseSortOrder(orderJSON)
		if err != nil {
			return nil, err
		}
		sort = append(sort, order...)
	}
	if len(sort) > 0 {
		orders.SetSort(sort)
	}

	return &orders, nil
}

func parseSortOrder(orderJSON string) (bson.D, error) {
	// Validate the whole input before reading keys in their original JSON order.
	if !json.Valid([]byte(orderJSON)) {
		return nil, errors.New("invalid JSON for MongoDB sort order")
	}
	decoder := json.NewDecoder(strings.NewReader(orderJSON))
	start, err := decoder.Token()
	if err != nil || start == nil {
		return nil, err
	}
	if start != json.Delim('{') {
		return nil, errors.New("MongoDB sort order must be a JSON object")
	}
	var sort bson.D
	positions := make(map[string]int)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var direction string
		if err := decoder.Decode(&direction); err != nil {
			return nil, err
		}
		value := 0
		switch direction {
		case sdk.OrderDESC:
			value = -1
		case sdk.OrderASC:
			value = 1
		}
		field := key.(string)
		if index, exists := positions[field]; exists {
			// As with encoding/json maps, later values replace earlier ones.
			sort[index].Value = value
			continue
		}
		positions[field] = len(sort)
		sort = append(sort, bson.E{Key: field, Value: value})
	}
	return slices.DeleteFunc(sort, func(field bson.E) bool { return field.Value == 0 }), nil
}

func (c *DB) Transaction(ctx context.Context, fn func(sessionCtx mongo.SessionContext) (any, error)) (
	any, error,
) {
	session, err := c.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	result, err := session.WithTransaction(ctx, fn)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func ReplaceSpecialCharacters(keyword string) string {
	return replaceSpecialCharacters.Replace(keyword)
}
