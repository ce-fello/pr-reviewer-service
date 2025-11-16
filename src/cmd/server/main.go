package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	api2 "github.com/ce-fello/pr-reviewer-service/src/internal/api"
	"github.com/ce-fello/pr-reviewer-service/src/internal/service"
	"github.com/ce-fello/pr-reviewer-service/src/internal/store"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	port := getenv("PORT", "8080")
	dsn := getenv("DATABASE_URL", "postgres://pguser:pgpass@db:5432/prdb?sslmode=disable")

	migDir := flag.String("migrations", "./migrations", "migrations directory")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			logger.Fatal("failed to sync logger", zap.Error(err))
		}
	}(logger)
	sugar := logger.Sugar()

	db, err := connectDBWithRetry(dsn, 15, 2*time.Second, sugar)
	if err != nil {
		sugar.Fatalf("failed to connect to db: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			sugar.Fatalf("failed to close db: %v", err)
		}
	}(db)

	if err := runMigrations(dsn, *migDir, sugar); err != nil {
		sugar.Fatalf("migrations failed: %v", err)
	}
	sugar.Info("migrations applied")

	repos := store.NewRepositories(db, sugar.Desugar())
	svc := service.NewService(repos, sugar.Desugar())
	h := api2.NewHandler(svc, sugar.Desugar())

	r := chi.NewRouter()
	r.Use(api2.RequestIDMiddleware, api2.LoggerMiddleware(logger), api2.Recoverer)
	api2.RegisterRoutes(r, h)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		sugar.Infof("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			sugar.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	sugar.Infof("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		sugar.Fatalf("server forced to shutdown: %v", err)
	}
	sugar.Info("server stopped")
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func connectDBWithRetry(dsn string, attempts int, delay time.Duration, sugar *zap.SugaredLogger) (*sql.DB, error) {
	var db *sql.DB
	var err error

	for i := 0; i < attempts; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				return db, nil
			}
		}
		sugar.Warnf("db ping error: %v (attempt %d/%d)", err, i+1, attempts)
		time.Sleep(delay)
	}
	return nil, fmt.Errorf("db connect failed: %w", err)
}

func runMigrations(dsn, migrationsDir string, sugar *zap.SugaredLogger) error {
	sugar.Infof("running migrations from %s", migrationsDir)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("migration open db: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("migration init: %w", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migration up: %w", err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		sugar.Info("no new migrations — already up to date")
	}

	return nil
}
