package notev1

import "time"

type Note struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Priority    int8      `json:"priority"`
	Enabled     bool      `json:"enabled"`
	Reviewed    *bool     `json:"reviewed"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// NoteInput is the full note payload used by create and replacement updates.
type NoteInput struct {
	Name        string `json:"name" form:"name" binding:"required,max=64"`
	Description string `json:"description" form:"description" binding:"max=1024"`
	Priority    *int8  `json:"priority,omitempty" form:"priority" binding:"required,gte=0,lte=10" default:"2"`
	Enabled     *bool  `json:"enabled,omitempty" form:"enabled" binding:"required" default:"true"`
	Reviewed    *bool  `json:"reviewed" form:"reviewed"`
}

type CreateNoteRequest struct{ NoteInput }

type UpdateNoteRequest struct {
	ID string `json:"-" form:"-" uri:"id" binding:"required,uuid"`
	NoteInput
}

type GetNoteRequest struct {
	ID string `json:"-" form:"-" uri:"id" binding:"required,uuid"`
}

type DeleteNoteRequest struct {
	ID string `json:"-" form:"-" uri:"id" binding:"required,uuid"`
}

type ListNotesRequest struct {
	PageNo   int `json:"pageNo" form:"pageNo" binding:"min=1,max=1000000" default:"1"`
	PageSize int `json:"pageSize" form:"pageSize" binding:"min=1,max=100" default:"20"`
}

type ListNotesResponse struct {
	Results    []Note `json:"results"`
	TotalCount int64  `json:"totalCount"`
	PageNo     int    `json:"pageNo"`
	PageSize   int    `json:"pageSize"`
}
