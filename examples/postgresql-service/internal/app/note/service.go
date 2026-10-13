package note

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/examples/postgresql-service/internal/table"
	"github.com/FlyTOmeLight/server-go/examples/postgresql-service/sdk/go/notev1"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type Service struct{ store *table.Store }

func New(store *table.Store) *Service { return &Service{store: store} }

func (s *Service) Create(ctx *core.Context, req *notev1.CreateNoteRequest) (*notev1.Note, error) {
	now := time.Now().UTC()
	note := record(uuid.NewString(), req.NoteInput)
	note.CreatedAt, note.UpdatedAt = now, now
	if err := s.store.Create(ctx.Request.Context(), note); err != nil {
		return nil, err
	}
	return response(note), nil
}

func (s *Service) Get(ctx *core.Context, req *notev1.GetNoteRequest) (*notev1.Note, error) {
	note, err := s.store.Get(ctx.Request.Context(), req.ID)
	if err != nil {
		return nil, noteError(err)
	}
	return response(note), nil
}

func (s *Service) Update(ctx *core.Context, req *notev1.UpdateNoteRequest) (*notev1.Note, error) {
	note := record(req.ID, req.NoteInput)
	note.UpdatedAt = time.Now().UTC()
	if err := s.store.Update(ctx.Request.Context(), note); err != nil {
		return nil, noteError(err)
	}
	return response(note), nil
}

func (s *Service) Delete(ctx *core.Context, req *notev1.DeleteNoteRequest) error {
	return noteError(s.store.Delete(ctx.Request.Context(), req.ID))
}

func (s *Service) List(ctx *core.Context, req *notev1.ListNotesRequest) (*notev1.ListNotesResponse, error) {
	notes, count, err := s.store.List(ctx.Request.Context(), req.PageNo, req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &notev1.ListNotesResponse{
		Results: make(
			[]notev1.Note,
			0,
			len(notes),
		),
		TotalCount: count,
		PageNo:     req.PageNo,
		PageSize:   req.PageSize,
	}
	for index := range notes {
		result.Results = append(result.Results, *response(&notes[index]))
	}
	return result, nil
}

func record(id string, input notev1.NoteInput) *table.Note {
	note := &table.Note{
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
		Priority:    *input.Priority,
		Enabled:     *input.Enabled,
	}
	if input.Reviewed != nil {
		note.Reviewed = sql.NullBool{Bool: *input.Reviewed, Valid: true}
	}
	return note
}

func response(note *table.Note) *notev1.Note {
	result := &notev1.Note{
		ID:          note.ID,
		Name:        note.Name,
		Description: note.Description,
		Priority:    note.Priority,
		Enabled:     note.Enabled,
		CreatedAt:   note.CreatedAt.UTC(),
		UpdatedAt:   note.UpdatedAt.UTC(),
	}
	if note.Reviewed.Valid {
		value := note.Reviewed.Bool
		result.Reviewed = &value
	}
	return result
}

func noteError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return serrors.ErrorResourceNotFound
	}
	return err
}
