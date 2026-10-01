package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/florinmihalache/ticketing/internal/ticket"
)

// testStore runs the same behavioural contract against any ticket.Store.
func testStore(t *testing.T, newStore func(t *testing.T) ticket.Store) {
	ctx := context.Background()
	valid := ticket.Input{Title: "Bug", Description: "desc", Status: ticket.StatusOpen, Priority: ticket.PriorityHigh, Reporter: "ana"}

	t.Run("list empty", func(t *testing.T) {
		s := newStore(t)
		got, err := s.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || len(got) != 0 {
			t.Fatalf("expected empty non-nil slice, got %#v", got)
		}
	})

	t.Run("create and get", func(t *testing.T) {
		s := newStore(t)
		created, err := s.Create(ctx, valid)
		if err != nil {
			t.Fatal(err)
		}
		if created.ID < 1 {
			t.Fatalf("expected positive id, got %d", created.ID)
		}
		if created.CreatedAt.IsZero() || !created.CreatedAt.Equal(created.UpdatedAt) {
			t.Fatalf("bad timestamps: %v / %v", created.CreatedAt, created.UpdatedAt)
		}
		if created.Title != valid.Title || created.Description != valid.Description ||
			created.Status != valid.Status || created.Priority != valid.Priority || created.Reporter != valid.Reporter {
			t.Fatalf("fields not stored: %+v", created)
		}

		got, err := s.Get(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != created.ID || got.Title != created.Title || !got.CreatedAt.Equal(created.CreatedAt) {
			t.Fatalf("got %+v, want %+v", got, created)
		}
	})

	t.Run("list ordered by id", func(t *testing.T) {
		s := newStore(t)
		var ids []int64
		for _, title := range []string{"a", "b", "c"} {
			in := valid
			in.Title = title
			tk, err := s.Create(ctx, in)
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, tk.ID)
		}
		got, err := s.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 tickets, got %d", len(got))
		}
		for i, tk := range got {
			if tk.ID != ids[i] {
				t.Fatalf("position %d: got id %d, want %d", i, tk.ID, ids[i])
			}
		}
	})

	t.Run("update", func(t *testing.T) {
		s := newStore(t)
		created, err := s.Create(ctx, valid)
		if err != nil {
			t.Fatal(err)
		}
		upd := ticket.Input{Title: "Bug v2", Status: ticket.StatusClosed, Priority: ticket.PriorityLow}
		got, err := s.Update(ctx, created.ID, upd)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "Bug v2" || got.Status != ticket.StatusClosed || got.Priority != ticket.PriorityLow ||
			got.Description != "" || got.Reporter != "" {
			t.Fatalf("update not applied as full replace: %+v", got)
		}
		if !got.CreatedAt.Equal(created.CreatedAt) {
			t.Fatalf("created_at changed: %v -> %v", created.CreatedAt, got.CreatedAt)
		}
		if got.UpdatedAt.Before(created.UpdatedAt) {
			t.Fatalf("updated_at went backwards: %v -> %v", created.UpdatedAt, got.UpdatedAt)
		}
		fetched, err := s.Get(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if fetched.Title != "Bug v2" {
			t.Fatalf("update not persisted: %+v", fetched)
		}
	})

	t.Run("delete", func(t *testing.T) {
		s := newStore(t)
		created, err := s.Create(ctx, valid)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(ctx, created.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, created.ID); !errors.Is(err, ticket.ErrNotFound) {
			t.Fatalf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("ids are not reused after delete", func(t *testing.T) {
		s := newStore(t)
		first, _ := s.Create(ctx, valid)
		if err := s.Delete(ctx, first.ID); err != nil {
			t.Fatal(err)
		}
		second, err := s.Create(ctx, valid)
		if err != nil {
			t.Fatal(err)
		}
		if second.ID == first.ID {
			t.Fatalf("id %d reused", second.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		s := newStore(t)
		const missing = 999999
		if _, err := s.Get(ctx, missing); !errors.Is(err, ticket.ErrNotFound) {
			t.Errorf("Get: expected ErrNotFound, got %v", err)
		}
		if _, err := s.Update(ctx, missing, valid); !errors.Is(err, ticket.ErrNotFound) {
			t.Errorf("Update: expected ErrNotFound, got %v", err)
		}
		if err := s.Delete(ctx, missing); !errors.Is(err, ticket.ErrNotFound) {
			t.Errorf("Delete: expected ErrNotFound, got %v", err)
		}
	})
}

func TestMemoryStore(t *testing.T) {
	testStore(t, func(t *testing.T) ticket.Store { return NewMemory() })
}

func TestMemoryStoreConcurrentCreate(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()
	const n = 100

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Create(ctx, ticket.Input{Title: "t"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	got, _ := s.List(ctx)
	if len(got) != n {
		t.Fatalf("expected %d tickets, got %d", n, len(got))
	}
	seen := make(map[int64]bool, n)
	for _, tk := range got {
		if seen[tk.ID] {
			t.Fatalf("duplicate id %d", tk.ID)
		}
		seen[tk.ID] = true
	}
}

// TestPostgresStore runs against a real database and is skipped unless
// TEST_DATABASE_URL is set, e.g.:
//
//	TEST_DATABASE_URL=postgres://tickets:tickets@localhost:5432/tickets?sslmode=disable go test ./...
func TestPostgresStore(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	testStore(t, func(t *testing.T) ticket.Store {
		pg, err := NewPostgres(context.Background(), dsn)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pg.pool.Exec(context.Background(), "TRUNCATE tickets RESTART IDENTITY"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pg.Close)
		return pg
	})
}
