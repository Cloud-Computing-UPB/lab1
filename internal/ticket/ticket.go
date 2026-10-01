package ticket

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// Ticket is a reported issue.
type Ticket struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Priority    Priority  `json:"priority"`
	Reporter    string    `json:"reporter"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Input is the client-supplied payload for creating or updating a ticket.
type Input struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      Status   `json:"status"`
	Priority    Priority `json:"priority"`
	Reporter    string   `json:"reporter"`
}

var ErrNotFound = errors.New("ticket not found")

// ValidationError describes an invalid client payload.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// Normalize trims fields, applies defaults and validates the input.
func (in *Input) Normalize() error {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Reporter = strings.TrimSpace(in.Reporter)

	if in.Title == "" {
		return &ValidationError{"title is required"}
	}
	if len(in.Title) > 200 {
		return &ValidationError{"title must be at most 200 characters"}
	}
	if in.Status == "" {
		in.Status = StatusOpen
	}
	if in.Priority == "" {
		in.Priority = PriorityMedium
	}
	switch in.Status {
	case StatusOpen, StatusInProgress, StatusClosed:
	default:
		return &ValidationError{"status must be one of: open, in_progress, closed"}
	}
	switch in.Priority {
	case PriorityLow, PriorityMedium, PriorityHigh:
	default:
		return &ValidationError{"priority must be one of: low, medium, high"}
	}
	return nil
}

// Store is the persistence abstraction implemented by each database backend.
type Store interface {
	List(ctx context.Context) ([]Ticket, error)
	Get(ctx context.Context, id int64) (Ticket, error)
	Create(ctx context.Context, in Input) (Ticket, error)
	Update(ctx context.Context, id int64, in Input) (Ticket, error)
	Delete(ctx context.Context, id int64) error
	Close()
}
