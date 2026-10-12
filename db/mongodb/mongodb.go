package mongodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FlyTOmeLight/server-go/sdk"
)

type Client struct {
	*mongo.Client
	config            *Config
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
		return nil, errors.Join(err, conn.Disconnect(ctx))
	}

	cfg := *config
	client := &Client{Client: conn, config: &cfg}
	client.SetDefaultCollection(cfg.Collection)
	return client, nil
}

func (c *DB) NewCollection(name string) *mongo.Collection {
	return c.Database(c.config.DB).Collection(name)
}

func (c *DB) GetDefaultCollection() *mongo.Collection {
	return c.defaultCollection
}

func (c *DB) SetDefaultCollection(name string) {
	c.defaultCollection = c.Database(c.config.DB).Collection(name)
}

func (c *DB) GetFindOptions(pageRequest sdk.PagingRequest) (*options.FindOptions, error) {
	var err error
	var orders options.FindOptions

	orders.SetLimit(int64(pageRequest.GetPageSize()))
	orders.SetSkip(int64((pageRequest.GetPageNo() - 1) * pageRequest.GetPageSize()))

	var sort bson.D
	for _, orderBytes := range pageRequest.Orders {
		var order = map[string]string{}
		if err = json.Unmarshal([]byte(orderBytes), &order); err != nil {
			return nil, err
		}

		for key, value := range order {
			switch value {
			case sdk.OrderDESC:
				sort = append(sort, bson.E{Key: key, Value: -1})
			case sdk.OrderASC:
				sort = append(sort, bson.E{Key: key, Value: 1})
			}
		}

	}
	if len(sort) > 0 {
		orders.SetSort(sort)
	}

	return &orders, nil
}

func (c *DB) Transaction(ctx context.Context, fn func(sessionCtx mongo.SessionContext) (interface{}, error)) (
	interface{}, error) {
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
