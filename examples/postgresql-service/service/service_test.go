package service

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/examples/postgresql-service/internal/table"
	"github.com/FlyTOmeLight/server-go/examples/postgresql-service/sdk/go/notev1"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/logger"
)

func TestPostgreSQLCRUDAndLifecycle(t *testing.T) {
	dsn := os.Getenv("SERVER_GO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SERVER_GO_TEST_POSTGRES_DSN is unset; make check supplies an isolated PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	module := &db.Module{
		Config: &db.Config{Driver: db.DriverPostgreSQL, DSN: dsn, MaxOpenConns: 4, MaxIdleConns: 2},
	}
	if err := module.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	connection, err := module.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := module.Close(context.Background()); err != nil {
			t.Error(err)
		}
		if err := connection.PingContext(context.Background()); err == nil {
			t.Error("database module left its connection pool open")
		}
	})
	notes, err := New(ctx, module.DB, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := notes.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := notes.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	manager, err := conf.NewManagerFromConfig(
		&conf.Config{AppName: "postgresql-service", AppCode: 100, RunMode: "release"},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	log, err := logger.NewLogger(&logger.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	httpHost, err := host.NewHTTPHost(manager, log, nil, notes)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := httpHost.Close(); err != nil {
			t.Error(err)
		}
	})
	server := httptest.NewServer(httpHost.Handler())
	t.Cleanup(server.Close)
	client, err := notev1.NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	var ids []string
	t.Cleanup(func() {
		for _, id := range ids {
			if err := module.DB.WithContext(context.Background()).
				Delete(&table.Note{}, "id = ?", id).
				Error; err != nil {
				t.Error(err)
			}
		}
	})
	first, err := client.CreateNote(
		ctx,
		&notev1.CreateNoteRequest{NoteInput: notev1.NoteInput{Name: "default note"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	ids = append(ids, first.ID)
	if _, err := uuid.Parse(
		first.ID,
	); err != nil || first.Priority != 2 || !first.Enabled ||
		first.Reviewed != nil {
		t.Fatalf("defaults or UUID: %+v error=%v", first, err)
	}
	createdRead, err := client.GetNote(ctx, &notev1.GetNoteRequest{ID: first.ID})
	if err != nil || !reflect.DeepEqual(first, createdRead) {
		t.Fatalf("created record differs: created=%+v got=%+v error=%v", first, createdRead, err)
	}
	for _, reviewed := range []bool{true, false} {
		created, err := client.CreateNote(ctx, &notev1.CreateNoteRequest{NoteInput: notev1.NoteInput{
			Name: fmt.Sprintf("reviewed %v", reviewed), Reviewed: &reviewed,
		}})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, created.ID)
		got, err := client.GetNote(ctx, &notev1.GetNoteRequest{ID: created.ID})
		if err != nil || got.Reviewed == nil || *got.Reviewed != reviewed || got.Priority != 2 || !got.Enabled {
			t.Fatalf("nullable bool roundtrip: %+v error=%v", got, err)
		}
	}
	listed, err := client.ListNotes(ctx, &notev1.ListNotesRequest{})
	if err != nil || listed.PageNo != 1 || listed.PageSize != 20 || listed.TotalCount != 3 ||
		len(listed.Results) != 3 {
		t.Fatalf("default pagination: %+v error=%v", listed, err)
	}
	page, err := client.ListNotes(ctx, &notev1.ListNotesRequest{PageNo: 2, PageSize: 2})
	if err != nil || page.TotalCount != 3 || len(page.Results) != 1 || page.Results[0].ID != ids[2] {
		t.Fatalf("stable second page: %+v error=%v", page, err)
	}
	priority, enabled, reviewed := int8(0), false, false
	updated, err := client.UpdateNote(ctx, &notev1.UpdateNoteRequest{ID: first.ID, NoteInput: notev1.NoteInput{
		Name:        "replacement",
		Description: "hello`world",
		Priority:    &priority,
		Enabled:     &enabled,
		Reviewed:    &reviewed,
	}})
	if err != nil || updated.Priority != 0 || updated.Enabled || updated.Reviewed == nil || *updated.Reviewed ||
		updated.Description != "hello`world" || updated.CreatedAt.IsZero() {
		t.Fatalf("replacement did not preserve zero/false or timestamps: %+v error=%v", updated, err)
	}
	got, err := client.GetNote(ctx, &notev1.GetNoteRequest{ID: first.ID})
	if err != nil || !reflect.DeepEqual(updated, got) {
		t.Fatalf("updated record differs: updated=%+v got=%+v error=%v", updated, got, err)
	}
	updated, err = client.UpdateNote(
		ctx,
		&notev1.UpdateNoteRequest{ID: first.ID, NoteInput: notev1.NoteInput{Name: "cleared review"}},
	)
	if err != nil || updated.Reviewed != nil || updated.Priority != 2 || !updated.Enabled {
		t.Fatalf("nullable bool was not cleared: %+v error=%v", updated, err)
	}
	var literal string
	if err := module.DB.WithContext(ctx).
		Raw("SELECT $$hello`world$$::text").
		Scan(&literal).
		Error; err != nil ||
		literal != "hello`world" {
		t.Fatalf("native SQL text changed: %q error=%v", literal, err)
	}
	if err := client.DeleteNote(ctx, &notev1.DeleteNoteRequest{ID: first.ID}); err != nil {
		t.Fatal(err)
	}
	var detail *serrors.Error
	if _, err := client.GetNote(
		ctx,
		&notev1.GetNoteRequest{ID: first.ID},
	); !errors.As(err, &detail) ||
		detail.Status != 404 {
		t.Fatalf("deleted note did not return 404: %v", err)
	}
	canceled, cancelRequest := context.WithCancel(ctx)
	cancelRequest()
	if _, err := client.ListNotes(canceled, &notev1.ListNotesRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("SDK cancellation lost: %v", err)
	}
	if _, _, err := table.New(module.DB).List(canceled, 1, 20); !errors.Is(err, context.Canceled) {
		t.Fatalf("store cancellation lost: %v", err)
	}
	queryCtx, cancelQuery := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancelQuery()
	if err := module.DB.WithContext(queryCtx).
		Exec("SELECT pg_sleep(5)").
		Error; !errors.Is(
		err,
		context.DeadlineExceeded,
	) {
		t.Fatalf("database deadline lost: %v", err)
	}
}
