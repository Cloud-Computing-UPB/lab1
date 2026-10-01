package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Cloud-Computing-UPB/lab1/internal/ticket"
)

const schema = `
CREATE TABLE IF NOT EXISTS tickets (
	id          BIGSERIAL PRIMARY KEY,
	title       VARCHAR(200) NOT NULL,
	description TEXT         NOT NULL DEFAULT '',
	status      VARCHAR(20)  NOT NULL DEFAULT 'open',
	priority    VARCHAR(20)  NOT NULL DEFAULT 'medium',
	reporter    VARCHAR(100) NOT NULL DEFAULT '',
	created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
	updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);`

const columns = `id, title, description, status, priority, reporter, created_at, updated_at`

// Postgres is a ticket store backed by PostgreSQL.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres connects to the database (retrying while it starts up) and
// ensures the schema exists.
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	var pingErr error
	for attempt := 1; attempt <= 10; attempt++ {
		if pingErr = pool.Ping(ctx); pingErr == nil {
			break
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if pingErr != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", pingErr)
	}

	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func scanTicket(row pgx.Row) (ticket.Ticket, error) {
	var t ticket.Ticket
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.Reporter, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ticket.Ticket{}, ticket.ErrNotFound
	}
	return t, err
}

func (p *Postgres) List(ctx context.Context) ([]ticket.Ticket, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM tickets ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ticket.Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (p *Postgres) Get(ctx context.Context, id int64) (ticket.Ticket, error) {
	return scanTicket(p.pool.QueryRow(ctx, `SELECT `+columns+` FROM tickets WHERE id = $1`, id))
}

func (p *Postgres) Create(ctx context.Context, in ticket.Input) (ticket.Ticket, error) {
	return scanTicket(p.pool.QueryRow(ctx,
		`INSERT INTO tickets (title, description, status, priority, reporter)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+columns,
		in.Title, in.Description, in.Status, in.Priority, in.Reporter))
}

func (p *Postgres) Update(ctx context.Context, id int64, in ticket.Input) (ticket.Ticket, error) {
	return scanTicket(p.pool.QueryRow(ctx,
		`UPDATE tickets
		 SET title = $2, description = $3, status = $4, priority = $5, reporter = $6, updated_at = now()
		 WHERE id = $1
		 RETURNING `+columns,
		id, in.Title, in.Description, in.Status, in.Priority, in.Reporter))
}

func (p *Postgres) Delete(ctx context.Context, id int64) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM tickets WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ticket.ErrNotFound
	}
	return nil
}

func (p *Postgres) Close() { p.pool.Close() }
