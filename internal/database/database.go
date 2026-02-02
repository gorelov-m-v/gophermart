// Package database provides database connection and management.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// DB wraps sql.DB with additional functionality.
type DB struct {
	conn *sql.DB
}

// Config holds database configuration.
type Config struct {
	DSN            string
	MigrationsPath string
}

// NewDB creates a new database connection and runs migrations.
func NewDB(cfg Config) (*DB, error) {
	conn, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	const (
		dbMaxOpenConns    = 25
		dbMaxIdleConns    = 5
		dbConnMaxLifetime = 5 * time.Minute
		dbConnMaxIdleTime = 1 * time.Minute
	)
	conn.SetMaxOpenConns(dbMaxOpenConns)
	conn.SetMaxIdleConns(dbMaxIdleConns)
	conn.SetConnMaxLifetime(dbConnMaxLifetime)
	conn.SetConnMaxIdleTime(dbConnMaxIdleTime)

	if err := conn.PingContext(context.Background()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}

	if cfg.MigrationsPath != "" {
		if err := db.runMigrations(cfg.MigrationsPath); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	return db, nil
}

// Ping verifies the database connection.
func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// GetConn returns the underlying sql.DB connection.
func (db *DB) GetConn() *sql.DB {
	return db.conn
}
