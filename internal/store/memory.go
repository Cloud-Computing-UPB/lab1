package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/florinmihalache/ticketing/internal/ticket"
)

// Memory is a thread-safe in-memory ticket store. Data is lost on restart.
type Memory struct {
	mu      sync.RWMutex
	nextID  int64
	tickets map[int64]ticket.Ticket
}

func NewMemory() *Memory {
	return &Memory{nextID: 1, tickets: make(map[int64]ticket.Ticket)}
}

func (m *Memory) List(_ context.Context) ([]ticket.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]ticket.Ticket, 0, len(m.tickets))
	for _, t := range m.tickets {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) Get(_ context.Context, id int64) (ticket.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tickets[id]
	if !ok {
		return ticket.Ticket{}, ticket.ErrNotFound
	}
	return t, nil
}

func (m *Memory) Create(_ context.Context, in ticket.Input) (ticket.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	t := ticket.Ticket{
		ID:          m.nextID,
		Title:       in.Title,
		Description: in.Description,
		Status:      in.Status,
		Priority:    in.Priority,
		Reporter:    in.Reporter,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.tickets[t.ID] = t
	m.nextID++
	return t, nil
}

func (m *Memory) Update(_ context.Context, id int64, in ticket.Input) (ticket.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tickets[id]
	if !ok {
		return ticket.Ticket{}, ticket.ErrNotFound
	}
	t.Title = in.Title
	t.Description = in.Description
	t.Status = in.Status
	t.Priority = in.Priority
	t.Reporter = in.Reporter
	t.UpdatedAt = time.Now().UTC()
	m.tickets[id] = t
	return t, nil
}

func (m *Memory) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tickets[id]; !ok {
		return ticket.ErrNotFound
	}
	delete(m.tickets, id)
	return nil
}

func (m *Memory) Close() {}
