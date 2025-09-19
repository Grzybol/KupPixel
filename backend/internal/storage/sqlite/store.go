package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	GridWidth     = 1000
	GridHeight    = 1000
	TotalPixels   = GridWidth * GridHeight
	busyTimeoutMs = 5000
)

type Store struct {
	db *sql.DB
}

type Pixel struct {
	ID        int       `json:"id"`
	Status    string    `json:"status"`
	Color     string    `json:"color,omitempty"`
	URL       string    `json:"url,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type PixelState struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Pixels []Pixel `json:"pixels"`
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("sqlite path must not be empty")
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", busyTimeoutMs)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure busy timeout: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) EnsureSchema(ctx context.Context) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ensure schema: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, execErr := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS pixels (
                id INTEGER PRIMARY KEY,
                status TEXT,
                color TEXT,
                url TEXT,
                updated_at TIMESTAMP
        )`); execErr != nil {
		err = fmt.Errorf("create pixels table: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_pixels_status ON pixels(status)`); execErr != nil {
		err = fmt.Errorf("create status index: %w", execErr)
		return err
	}

	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM pixels`).Scan(&count); err != nil {
		err = fmt.Errorf("count pixels: %w", err)
		return err
	}

	if count == 0 {
		for i := 0; i < TotalPixels; i++ {
			if ctx.Err() != nil {
				err = ctx.Err()
				return err
			}
			query := fmt.Sprintf("INSERT OR IGNORE INTO pixels(id, status, color, url, updated_at) VALUES (%d, 'free', '', '', CURRENT_TIMESTAMP)", i)
			if _, execErr := tx.ExecContext(ctx, query); execErr != nil {
				err = fmt.Errorf("fill pixels: %w", execErr)
				return err
			}
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = fmt.Errorf("commit ensure schema: %w", commitErr)
		return err
	}
	return nil
}

func parseUpdatedAt(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}

	var parseErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
		parseErr = err
	}

	return time.Time{}, fmt.Errorf("unsupported time format %q: %w", value, parseErr)
}

func (s *Store) GetAllPixels(ctx context.Context) (PixelState, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, status, COALESCE(color, ''), COALESCE(url, ''), updated_at FROM pixels ORDER BY id`)
	if err != nil {
		return PixelState{}, fmt.Errorf("query pixels: %w", err)
	}
	defer rows.Close()

	pixels := make([]Pixel, 0, TotalPixels)
	for rows.Next() {
		var pixel Pixel
		var updated sql.NullString
		if err := rows.Scan(&pixel.ID, &pixel.Status, &pixel.Color, &pixel.URL, &updated); err != nil {
			return PixelState{}, fmt.Errorf("scan pixel: %w", err)
		}
		if updated.Valid {
			parsed, err := parseUpdatedAt(updated.String)
			if err != nil {
				return PixelState{}, fmt.Errorf("parse pixel %d updated_at: %w", pixel.ID, err)
			}
			pixel.UpdatedAt = parsed
		}
		pixels = append(pixels, pixel)
	}

	if err := rows.Err(); err != nil {
		return PixelState{}, fmt.Errorf("iterate pixels: %w", err)
	}

	return PixelState{Width: GridWidth, Height: GridHeight, Pixels: pixels}, nil
}

func (s *Store) UpdatePixel(ctx context.Context, pixel Pixel) (updated Pixel, err error) {
	if pixel.ID < 0 || pixel.ID >= TotalPixels {
		return Pixel{}, fmt.Errorf("invalid pixel id: %d", pixel.ID)
	}

	updated = Pixel{ID: pixel.ID}
	if strings.EqualFold(pixel.Status, "taken") {
		if pixel.Color == "" || pixel.URL == "" {
			return Pixel{}, errors.New("taken pixels require color and url")
		}
		updated.Status = "taken"
		updated.Color = pixel.Color
		updated.URL = pixel.URL
	} else {
		updated.Status = "free"
	}

	updated.UpdatedAt = time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Pixel{}, fmt.Errorf("begin update pixel: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	query := fmt.Sprintf(
		"UPDATE pixels SET status = %s, color = %s, url = %s, updated_at = %s WHERE id = %d",
		quoteLiteral(updated.Status),
		quoteLiteral(updated.Color),
		quoteLiteral(updated.URL),
		quoteLiteral(updated.UpdatedAt.Format(time.RFC3339Nano)),
		updated.ID,
	)

	res, err := tx.ExecContext(ctx, query)
	if err != nil {
		return Pixel{}, fmt.Errorf("update pixel: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return Pixel{}, fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return Pixel{}, sql.ErrNoRows
	}

	if err = tx.Commit(); err != nil {
		return Pixel{}, fmt.Errorf("commit update pixel: %w", err)
	}

	return updated, nil
}

func quoteLiteral(value string) string {
	escaped := strings.ReplaceAll(value, "'", "''")
	return "'" + escaped + "'"
}
