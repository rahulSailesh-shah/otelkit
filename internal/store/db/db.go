package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

type DB interface {
	Connect() error
	Close() error
	GetWriteDB() *sql.DB
	GetReadDB() *sql.DB
}

type sqliteDB struct {
	ctx     context.Context
	writeDB *sql.DB
	readDB  *sql.DB
	dsn     string
}

func NewSQLiteDB(ctx context.Context, dsn string) DB {
	return &sqliteDB{
		ctx: ctx,
		dsn: dsn,
	}
}

func (s *sqliteDB) Connect() error {
	writeDB, err := openConn(s.ctx, s.dsn, 1)
	if err != nil {
		return fmt.Errorf("open write pool: %w", err)
	}

	readDB, err := openConn(s.ctx, s.dsn, 4)
	if err != nil {
		_ = writeDB.Close()
		return fmt.Errorf("open read pool: %w", err)
	}

	// Prevent accidental writes through the read pool.
	if _, err := readDB.ExecContext(s.ctx, "PRAGMA query_only = ON;"); err != nil {
		_ = writeDB.Close()
		_ = readDB.Close()
		return fmt.Errorf("set query_only on read pool: %w", err)
	}

	s.writeDB = writeDB
	s.readDB = readDB

	log.Printf("Connected to SQLite database successfully: %s (writer=1 conn, reader=4 conns)", s.dsn)
	return nil
}

// openConn opens a connection pool with shared PRAGMAs.
func openConn(ctx context.Context, dsn string, maxOpen int) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen)

	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA foreign_keys = ON;",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func (s *sqliteDB) Close() error {
	var firstErr error

	if s.readDB != nil {
		if err := s.readDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.writeDB != nil {
		if err := s.writeDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if firstErr != nil {
		return firstErr
	}

	log.Println("SQLite database connections closed")
	return nil
}

func (s *sqliteDB) GetWriteDB() *sql.DB {
	return s.writeDB
}

func (s *sqliteDB) GetReadDB() *sql.DB {
	return s.readDB
}
