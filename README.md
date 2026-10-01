# Ticketing API

Simple CRUD REST API for reporting issues (tickets), written in Go.

## Configuration

| Variable       | Default | Description                                             |
|----------------|---------|---------------------------------------------------------|
| `USE_POSTGRES` | `false` (binary) / `true` (compose) | `true` = PostgreSQL, `false` = in-memory |
| `DATABASE_URL` | –       | Postgres DSN, required when `USE_POSTGRES=true`         |
| `PORT`         | `8080`  | HTTP port                                               |

## Run

```sh
cp .env.example .env            # optional, edit USE_POSTGRES etc.
docker compose up --build       # PostgreSQL store
USE_POSTGRES=false docker compose up --build   # in-memory store
```

## Endpoints

| Method | Path            | Description      |
|--------|-----------------|------------------|
| GET    | `/health`       | Health check     |
| GET    | `/tickets`      | List tickets     |
| POST   | `/tickets`      | Create ticket    |
| GET    | `/tickets/{id}` | Get ticket       |
| PUT    | `/tickets/{id}` | Replace ticket   |
| DELETE | `/tickets/{id}` | Delete ticket    |

Body for POST/PUT (`title` required; `status` ∈ open|in_progress|closed, default `open`;
`priority` ∈ low|medium|high, default `medium`):

```sh
curl -X POST localhost:8080/tickets -d '{"title":"Login broken","description":"500 on submit","priority":"high","reporter":"ana"}'
```

## Tests

```sh
go test -race ./...
```

The PostgreSQL store tests are skipped unless `TEST_DATABASE_URL` points at a database
(the `tickets` table is truncated, so use a throwaway one):

```sh
docker run -d --rm --name tickets-test-db -e POSTGRES_PASSWORD=t -p 55432:5432 postgres:16-alpine
TEST_DATABASE_URL='postgres://postgres:t@localhost:55432/postgres?sslmode=disable' go test ./...
```
