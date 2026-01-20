package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func Init(ctx context.Context) (*sql.DB, error) {
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}

	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "clubhouse"
	}

	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		password = "changeme"
	}

	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		dbName = "clubhouse"
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbName,
	)

	driverName, err := otelsql.Register("postgres",
		otelsql.WithAttributes(attribute.String("db.system", "postgresql")),
		otelsql.WithMeterProvider(otel.GetMeterProvider()),
		otelsql.WithTracerProvider(otel.GetTracerProvider()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register otel driver: %w", err)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	ctxTest, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctxTest); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if _, err := otelsql.RegisterDBStatsMetrics(db, otelsql.WithMeterProvider(otel.GetMeterProvider())); err != nil {
		return nil, fmt.Errorf("failed to register db stats metrics: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}
