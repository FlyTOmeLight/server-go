package mongodb

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver"

	"github.com/FlyTOmeLight/server-go/sdk"
)

func TestModuleRejectsMissingConfigAndCancelledInitialization(t *testing.T) {
	if err := (&Module{}).Initialize(context.Background()); err == nil {
		t.Fatal("missing configuration accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := &Module{
		Config: &Config{
			Host:       "127.0.0.1",
			Port:       1,
			User:       "user",
			Password:   "password",
			DB:         "db",
			Collection: "items",
		},
	}
	if err := module.Initialize(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled initialization: %v", err)
	}
	if module.Client != nil {
		t.Fatal("failed initialization published a client")
	}
	if err := module.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

type cancellableDeployment struct{ driver.SingleServerDeployment }

func (cancellableDeployment) Disconnect(ctx context.Context) error { return ctx.Err() }

func TestModuleRetainsClientForCloseRetry(t *testing.T) {
	conn, err := mongo.NewClient(&options.ClientOptions{Deployment: cancellableDeployment{}})
	if err != nil {
		t.Fatal(err)
	}
	client := &DB{Client: conn}
	module := &Module{Client: client}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := module.Close(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled close: %v", err)
	}
	if module.Client != client {
		t.Fatal("failed close discarded the client")
	}
	if err := module.Close(t.Context()); err != nil || module.Client != nil {
		t.Fatalf("close retry: client=%v error=%v", module.Client, err)
	}
}

func TestGetFindOptionsPreservesSortPriority(t *testing.T) {
	client := &DB{}
	request := sdk.PagingRequest{
		PageNo: 2, PageSize: 10,
		Orders: []string{`{"z":"asc","a":"asc","z":"desc","ignored":null}`, `{"id":"asc"}`},
	}
	want := bson.D{{Key: "z", Value: -1}, {Key: "a", Value: 1}, {Key: "id", Value: 1}}
	for range 32 {
		options, err := client.GetFindOptions(request)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(options.Sort, want) || *options.Limit != 10 || *options.Skip != 10 {
			t.Fatalf("unexpected find options: %+v, want sort %v, limit 10, skip 10", options, want)
		}
	}
	for _, invalid := range []string{`[]`, `{"id":1}`, `{"id":"asc"} {}`} {
		if _, err := client.GetFindOptions(sdk.PagingRequest{Orders: []string{invalid}}); err == nil {
			t.Errorf("invalid sort accepted: %s", invalid)
		}
	}
}

func TestDefaultCollectionConcurrentAccess(t *testing.T) {
	conn, err := mongo.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client := &DB{Client: conn, config: &Config{DB: "test"}}
	client.SetDefaultCollection("first")
	var workers sync.WaitGroup
	for _, name := range []string{"first", "second"} {
		workers.Go(func() {
			for range 100 {
				client.SetDefaultCollection(name)
				collection := client.GetDefaultCollection()
				if collection == nil ||
					(collection.Name() != "first" && collection.Name() != "second") {
					t.Errorf("unexpected default collection: %v", collection)
					return
				}
			}
		})
	}
	workers.Wait()
}
