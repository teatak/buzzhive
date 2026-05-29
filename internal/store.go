package buzzhive

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func OpenStore(cfg DatabaseConfig) (*Store, error) {
	driver := strings.ToLower(cfg.Driver)
	if driver == "" {
		driver = "sqlite"
	}
	var dsn string
	switch driver {
	case "sqlite", "sqlite3":
		driver = "sqlite"
		if cfg.Path == "" {
			cfg.Path = "data/buzzhive.db"
		}
		dsn = cfg.Path + "?_busy_timeout=5000&_journal_mode=WAL"
	case "postgres", "postgresql", "pg":
		driver = "postgres"
		dsn = cfg.URL
		if dsn == "" {
			return nil, errors.New("database.url is required for postgres")
		}
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}

	sqlDriver := driver
	if driver == "sqlite" {
		sqlDriver = "sqlite3"
	}
	db, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, dialect: driver}
	if err := store.Migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(s.rebind(query), args...)
}

func (s *Store) query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(s.rebind(query), args...)
}

func (s *Store) queryRow(query string, args ...any) *sql.Row {
	return s.db.QueryRow(s.rebind(query), args...)
}

func (s *Store) prepareTx(tx *sql.Tx, query string) (*sql.Stmt, error) {
	return tx.Prepare(s.rebind(query))
}

func (s *Store) insertReturningID(query string, args ...any) (int64, error) {
	if s.dialect == "postgres" {
		var id int64
		err := s.queryRow(query+" RETURNING id", args...).Scan(&id)
		return id, err
	}
	res, err := s.exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) rebind(query string) string {
	if s.dialect != "postgres" {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	arg := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(arg))
			arg++
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
