package ticket_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cloud-Computing-UPB/lab1/internal/store"
	"github.com/Cloud-Computing-UPB/lab1/internal/ticket"
)

func newServer(t *testing.T, s ticket.Store) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	ticket.NewHandler(s).Register(mux)
	return mux
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, want, rec.Body.String())
	}
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMsg string) {
	t.Helper()
	assertStatus(t, rec, wantStatus)
	body := decode[map[string]string](t, rec)
	if !strings.Contains(body["error"], wantMsg) {
		t.Fatalf("error = %q, want it to contain %q", body["error"], wantMsg)
	}
}

func TestCreateTicket(t *testing.T) {
	h := newServer(t, store.NewMemory())

	rec := do(t, h, http.MethodPost, "/tickets", `{"title":"Login broken","priority":"high","reporter":"ana"}`)
	assertStatus(t, rec, http.StatusCreated)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if loc := rec.Header().Get("Location"); loc != "/tickets/1" {
		t.Errorf("Location = %q, want /tickets/1", loc)
	}
	got := decode[ticket.Ticket](t, rec)
	if got.ID != 1 || got.Title != "Login broken" || got.Priority != ticket.PriorityHigh ||
		got.Status != ticket.StatusOpen || got.Reporter != "ana" {
		t.Fatalf("unexpected ticket: %+v", got)
	}
}

func TestCreateTicketInvalid(t *testing.T) {
	h := newServer(t, store.NewMemory())

	tests := []struct {
		name, body, wantMsg string
	}{
		{"malformed json", `{"title":`, "invalid JSON body"},
		{"empty body", ``, "invalid JSON body"},
		{"unknown field", `{"title":"x","severity":"bad"}`, "unknown field"},
		{"wrong type", `{"title":123}`, "invalid JSON body"},
		{"missing title", `{"description":"no title"}`, "title is required"},
		{"bad status", `{"title":"x","status":"done"}`, "status must be"},
		{"bad priority", `{"title":"x","priority":"urgent"}`, "priority must be"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertError(t, do(t, h, http.MethodPost, "/tickets", tc.body), http.StatusBadRequest, tc.wantMsg)
		})
	}

	// Nothing should have been stored.
	list := decode[[]ticket.Ticket](t, do(t, h, http.MethodGet, "/tickets", ""))
	if len(list) != 0 {
		t.Fatalf("invalid requests created tickets: %+v", list)
	}
}

func TestListTickets(t *testing.T) {
	h := newServer(t, store.NewMemory())

	rec := do(t, h, http.MethodGet, "/tickets", "")
	assertStatus(t, rec, http.StatusOK)
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Fatalf("empty list should encode as [], got %s", body)
	}

	do(t, h, http.MethodPost, "/tickets", `{"title":"a"}`)
	do(t, h, http.MethodPost, "/tickets", `{"title":"b"}`)

	list := decode[[]ticket.Ticket](t, do(t, h, http.MethodGet, "/tickets", ""))
	if len(list) != 2 || list[0].Title != "a" || list[1].Title != "b" {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestGetTicket(t *testing.T) {
	h := newServer(t, store.NewMemory())
	do(t, h, http.MethodPost, "/tickets", `{"title":"a"}`)

	rec := do(t, h, http.MethodGet, "/tickets/1", "")
	assertStatus(t, rec, http.StatusOK)
	if got := decode[ticket.Ticket](t, rec); got.ID != 1 || got.Title != "a" {
		t.Fatalf("unexpected ticket: %+v", got)
	}

	assertError(t, do(t, h, http.MethodGet, "/tickets/42", ""), http.StatusNotFound, "not found")
}

func TestInvalidIDs(t *testing.T) {
	h := newServer(t, store.NewMemory())
	for _, id := range []string{"abc", "0", "-1", "1.5"} {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			t.Run(method+" "+id, func(t *testing.T) {
				assertError(t, do(t, h, method, "/tickets/"+id, `{"title":"x"}`), http.StatusBadRequest, "invalid ticket id")
			})
		}
	}
}

func TestUpdateTicket(t *testing.T) {
	h := newServer(t, store.NewMemory())
	do(t, h, http.MethodPost, "/tickets", `{"title":"a","description":"d","reporter":"ana"}`)

	rec := do(t, h, http.MethodPut, "/tickets/1", `{"title":"a2","status":"closed"}`)
	assertStatus(t, rec, http.StatusOK)
	got := decode[ticket.Ticket](t, rec)
	if got.Title != "a2" || got.Status != ticket.StatusClosed || got.Priority != ticket.PriorityMedium ||
		got.Description != "" || got.Reporter != "" {
		t.Fatalf("PUT should fully replace the ticket: %+v", got)
	}

	fetched := decode[ticket.Ticket](t, do(t, h, http.MethodGet, "/tickets/1", ""))
	if fetched.Title != "a2" {
		t.Fatalf("update not persisted: %+v", fetched)
	}

	assertError(t, do(t, h, http.MethodPut, "/tickets/1", `{"title":""}`), http.StatusBadRequest, "title is required")
	assertError(t, do(t, h, http.MethodPut, "/tickets/42", `{"title":"x"}`), http.StatusNotFound, "not found")
}

func TestDeleteTicket(t *testing.T) {
	h := newServer(t, store.NewMemory())
	do(t, h, http.MethodPost, "/tickets", `{"title":"a"}`)

	rec := do(t, h, http.MethodDelete, "/tickets/1", "")
	assertStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Errorf("204 response should have no body, got %q", rec.Body.String())
	}

	assertError(t, do(t, h, http.MethodGet, "/tickets/1", ""), http.StatusNotFound, "not found")
	assertError(t, do(t, h, http.MethodDelete, "/tickets/1", ""), http.StatusNotFound, "not found")
}

func TestMethodNotAllowed(t *testing.T) {
	h := newServer(t, store.NewMemory())
	assertStatus(t, do(t, h, http.MethodPatch, "/tickets/1", `{}`), http.StatusMethodNotAllowed)
	assertStatus(t, do(t, h, http.MethodDelete, "/tickets", ""), http.StatusMethodNotAllowed)
}

func TestRequestBodyTooLarge(t *testing.T) {
	h := newServer(t, store.NewMemory())
	body := `{"title":"x","description":"` + strings.Repeat("a", 2<<20) + `"}`
	assertError(t, do(t, h, http.MethodPost, "/tickets", body), http.StatusBadRequest, "invalid JSON body")
}

// failingStore returns an internal error from every method.
type failingStore struct{}

var errBoom = errors.New("db down")

func (failingStore) List(context.Context) ([]ticket.Ticket, error) { return nil, errBoom }
func (failingStore) Get(context.Context, int64) (ticket.Ticket, error) {
	return ticket.Ticket{}, errBoom
}
func (failingStore) Create(context.Context, ticket.Input) (ticket.Ticket, error) {
	return ticket.Ticket{}, errBoom
}
func (failingStore) Update(context.Context, int64, ticket.Input) (ticket.Ticket, error) {
	return ticket.Ticket{}, errBoom
}
func (failingStore) Delete(context.Context, int64) error { return errBoom }
func (failingStore) Close()                              {}

func TestInternalErrorsAreHidden(t *testing.T) {
	h := newServer(t, failingStore{})
	cases := []struct{ method, path, body string }{
		{http.MethodGet, "/tickets", ""},
		{http.MethodGet, "/tickets/1", ""},
		{http.MethodPost, "/tickets", `{"title":"x"}`},
		{http.MethodPut, "/tickets/1", `{"title":"x"}`},
		{http.MethodDelete, "/tickets/1", ""},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			rec := do(t, h, c.method, c.path, c.body)
			assertError(t, rec, http.StatusInternalServerError, "internal server error")
			if strings.Contains(rec.Body.String(), errBoom.Error()) {
				t.Fatalf("internal error leaked to client: %s", rec.Body.String())
			}
		})
	}
}
