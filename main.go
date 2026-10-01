package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Cloud-Computing-UPB/lab1/internal/store"
	"github.com/Cloud-Computing-UPB/lab1/internal/ticket"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	usePostgres, err := strconv.ParseBool(getenv("USE_POSTGRES", "false"))
	if err != nil {
		log.Fatalf("invalid USE_POSTGRES value: %v", err)
	}

	var s ticket.Store
	if usePostgres {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Fatal("DATABASE_URL is required when USE_POSTGRES=true")
		}
		pg, err := store.NewPostgres(ctx, dsn)
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		s = pg
		log.Println("using PostgreSQL store")
	} else {
		s = store.NewMemory()
		log.Println("using in-memory store")
	}
	defer s.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	ticket.NewHandler(s).Register(mux)

	srv := &http.Server{
		Addr:              ":" + getenv("PORT", "8080"),
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
