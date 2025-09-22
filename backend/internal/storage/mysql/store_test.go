package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/example/kup-piksel/internal/storage"
)

type stubDBState struct {
	records map[int]struct{}
}

func newStubDBState(existing []int) *stubDBState {
	state := &stubDBState{records: make(map[int]struct{}, storage.TotalPixels)}
	for _, id := range existing {
		state.records[id] = struct{}{}
	}
	return state
}

func (s *stubDBState) insert(id int) {
	s.records[id] = struct{}{}
}

func (s *stubDBState) count() int {
	return len(s.records)
}

func (s *stubDBState) has(id int) bool {
	_, ok := s.records[id]
	return ok
}

type stubConnector struct {
	state *stubDBState
}

func (c *stubConnector) Connect(context.Context) (driver.Conn, error) {
	return &stubConn{state: c.state}, nil
}

func (c *stubConnector) Driver() driver.Driver {
	return &stubDriver{state: c.state}
}

type stubDriver struct {
	state *stubDBState
}

func (d *stubDriver) Open(string) (driver.Conn, error) {
	return &stubConn{state: d.state}, nil
}

type stubConn struct {
	state *stubDBState
}

func (c *stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c *stubConn) Close() error { return nil }

func (c *stubConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *stubConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return &stubTx{conn: c}, nil
}

func (c *stubConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(query)), "INSERT INTO PIXELS") {
		if len(args) == 0 {
			return nil, errors.New("missing pixel id")
		}
		if len(args)%2 != 0 {
			return nil, errors.New("unexpected argument count")
		}

		for i := 0; i < len(args); i += 2 {
			id, err := asInt(args[i].Value)
			if err != nil {
				return nil, err
			}
			c.state.insert(id)
		}
		return driver.RowsAffected(len(args) / 2), nil
	}
	return driver.RowsAffected(0), nil
}

func (c *stubConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.TrimSpace(strings.ToUpper(query))
	if strings.HasPrefix(normalized, "SELECT COUNT(1) FROM PIXELS") {
		count := c.state.count()
		return &stubRows{
			columns: []string{"count"},
			values:  [][]driver.Value{{int64(count)}},
		}, nil
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

func (c *stubConn) PrepareContext(_ context.Context, query string) (driver.Stmt, error) {
	return &stubStmt{conn: c, query: query}, nil
}

type stubTx struct {
	conn *stubConn
}

func (t *stubTx) Commit() error   { return nil }
func (t *stubTx) Rollback() error { return nil }

type stubStmt struct {
	conn  *stubConn
	query string
}

func (s *stubStmt) Close() error { return nil }

func (s *stubStmt) NumInput() int { return -1 }

func (s *stubStmt) Exec(args []driver.Value) (driver.Result, error) {
	named := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	return s.conn.ExecContext(context.Background(), s.query, named)
}

func (s *stubStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.conn.ExecContext(ctx, s.query, args)
}

func (s *stubStmt) Query([]driver.Value) (driver.Rows, error) {
	return nil, driver.ErrSkip
}

func (s *stubStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.conn.QueryContext(ctx, s.query, args)
}

type stubRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *stubRows) Columns() []string { return r.columns }

func (r *stubRows) Close() error { return nil }

func (r *stubRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	row := r.values[r.index]
	r.index++
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		} else {
			dest[i] = nil
		}
	}
	return nil
}

func asInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case int32:
		return int(v), nil
	case int16:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint64:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint16:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", value)
	}
}

func TestEnsureSchemaSeedsMissingPixels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		existing    []int
		skip        bool
		wantCount   int
		mustContain []int
	}{
		{
			name:        "noExisting",
			existing:    nil,
			wantCount:   storage.TotalPixels,
			mustContain: []int{0, storage.TotalPixels - 1, 500500},
		},
		{
			name:        "someExisting",
			existing:    []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			wantCount:   storage.TotalPixels,
			mustContain: []int{0, storage.TotalPixels - 1, 500500},
		},
		{
			name:      "skipSeeding",
			existing:  []int{1, 2, 3},
			skip:      true,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			state := newStubDBState(tt.existing)

			connector := &stubConnector{state: state}
			db := sql.OpenDB(connector)
			t.Cleanup(func() { db.Close() })

			store := &Store{db: db}
			store.SetSkipPixelSeed(tt.skip)

			if err := store.EnsureSchema(context.Background()); err != nil {
				t.Fatalf("EnsureSchema() error = %v", err)
			}

			if got := state.count(); got != tt.wantCount {
				t.Fatalf("unexpected pixel count: got %d want %d", got, tt.wantCount)
			}

			for _, existing := range tt.existing {
				if !state.has(existing) {
					t.Fatalf("expected pre-existing pixel %d to remain", existing)
				}
			}

			for _, id := range tt.mustContain {
				if !state.has(id) {
					t.Fatalf("expected pixel %d to be present", id)
				}
			}
		})
	}
}
