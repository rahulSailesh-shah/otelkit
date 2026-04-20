package db

import (
	"context"
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

const sqlitePath = "/Users/rahulshah/Workspace/Go/otelkit/otelkit.db"

type DB interface {
	Connect() error
	Close() error
	GetDB() *sql.DB
}

type sqliteDB struct {
	ctx context.Context
	db  *sql.DB
	dsn string
}

func NewSQLiteDB(ctx context.Context) DB {
	return &sqliteDB{
		ctx: ctx,
		dsn: sqlitePath,
	}
}

func (s *sqliteDB) Connect() error {
	db, err := sql.Open("sqlite", s.dsn)
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(s.ctx, "PRAGMA journal_mode = WAL;"); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.ExecContext(s.ctx, "PRAGMA busy_timeout = 5000;"); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.ExecContext(s.ctx, "PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return err
	}

	if err := db.PingContext(s.ctx); err != nil {
		_ = db.Close()
		return err
	}

	s.db = db

	log.Printf("Connected to SQLite database successfully: %s", s.dsn)

	return nil
}

func (s *sqliteDB) Close() error {
	if s.db == nil {
		log.Println("SQLite database connection is already closed")
		return nil
	}

	if err := s.db.Close(); err != nil {
		return err
	}

	log.Println("SQLite database connection closed")
	return nil
}

func (s *sqliteDB) GetDB() *sql.DB {
	return s.db
}
