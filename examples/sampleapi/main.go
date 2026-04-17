package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"

	"github.com/rahulSailesh-shah/otelkit/pkg/otelkitdb"
	"github.com/rahulSailesh-shah/otelkit/pkg/otelkitmw"
	"github.com/rahulSailesh-shah/otelkit/pkg/sdk"
)

type Item struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type server struct {
	db *sql.DB
	mu sync.Mutex
}

func main() {
	shutdown, err := sdk.Init(
		sdk.WithServiceName("sampleapi"),
		sdk.WithServiceVersion("0.1.0"),
	)
	if err != nil {
		log.Fatalf("sdk init: %v", err)
	}
	defer shutdown(context.Background())

	db, err := otelkitdb.Open("sqlite3", "app.db")
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS items (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL
	)`); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	s := &server{db: db}
	mux := http.NewServeMux()
	mux.Handle("/items", otelkitmw.Wrap(http.HandlerFunc(s.items)))
	mux.Handle("/items/", otelkitmw.Wrap(http.HandlerFunc(s.itemByID)))
	mux.Handle("/healthz", otelkitmw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})))

	log.Println("sampleapi listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) items(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(`SELECT id, name FROM items ORDER BY id`)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer rows.Close()
		out := []Item{}
		for rows.Next() {
			var it Item
			if err := rows.Scan(&it.ID, &it.Name); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			out = append(out, it)
		}
		writeJSON(w, 200, out)
	case http.MethodPost:
		var in Item
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		res, err := s.db.Exec(`INSERT INTO items(name) VALUES(?)`, in.Name)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		in.ID, _ = res.LastInsertId()
		writeJSON(w, 201, in)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *server) itemByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/items/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	switch r.Method {
	case http.MethodGet:
		var it Item
		err := s.db.QueryRow(`SELECT id, name FROM items WHERE id=?`, id).Scan(&it.ID, &it.Name)
		if err == sql.ErrNoRows {
			http.Error(w, "not found", 404)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 200, it)
	case http.MethodDelete:
		if _, err := s.db.Exec(`DELETE FROM items WHERE id=?`, id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
