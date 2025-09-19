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
	OwnerID   *int      `json:"owner_id,omitempty"`
}

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
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

	if _, execErr := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
                id INTEGER PRIMARY KEY,
                username TEXT UNIQUE NOT NULL,
                password_hash TEXT NOT NULL,
                created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`); execErr != nil {
		err = fmt.Errorf("create users table: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS pixels (
                id INTEGER PRIMARY KEY,
                status TEXT,
                color TEXT,
                url TEXT,
                updated_at TIMESTAMP,
                owner_id INTEGER,
                FOREIGN KEY(owner_id) REFERENCES users(id)
        )`); execErr != nil {
		err = fmt.Errorf("create pixels table: %w", execErr)
		return err
	}

	if err = ensurePixelOwnerColumn(ctx, tx); err != nil {
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
			query := fmt.Sprintf("INSERT OR IGNORE INTO pixels(id, status, color, url, updated_at, owner_id) VALUES (%d, 'free', '', '', CURRENT_TIMESTAMP, NULL)", i)
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
	rows, err := s.db.QueryContext(ctx, `SELECT id, status, COALESCE(color, ''), COALESCE(url, ''), updated_at, owner_id FROM pixels ORDER BY id`)
	if err != nil {
		return PixelState{}, fmt.Errorf("query pixels: %w", err)
	}
	defer rows.Close()

	pixels := make([]Pixel, 0, TotalPixels)
	for rows.Next() {
		var pixel Pixel
		var updated sql.NullString
		var owner sql.NullInt64
		if err := rows.Scan(&pixel.ID, &pixel.Status, &pixel.Color, &pixel.URL, &updated, &owner); err != nil {
			return PixelState{}, fmt.Errorf("scan pixel: %w", err)
		}
		if updated.Valid {
			parsed, err := parseUpdatedAt(updated.String)
			if err != nil {
				return PixelState{}, fmt.Errorf("parse pixel %d updated_at: %w", pixel.ID, err)
			}
			pixel.UpdatedAt = parsed
		}
		if owner.Valid {
			id := int(owner.Int64)
			pixel.OwnerID = &id
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

	updated = Pixel{ID: pixel.ID, OwnerID: pixel.OwnerID}
	if strings.EqualFold(pixel.Status, "taken") {
		if pixel.Color == "" || pixel.URL == "" {
			return Pixel{}, errors.New("taken pixels require color and url")
		}
		updated.Status = "taken"
		updated.Color = pixel.Color
		updated.URL = pixel.URL
	} else {
		updated.Status = "free"
		updated.Color = ""
		updated.URL = ""
		updated.OwnerID = nil
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

	ownerValue := "NULL"
	if updated.OwnerID != nil {
		ownerValue = fmt.Sprintf("%d", *updated.OwnerID)
	}

	query := fmt.Sprintf(
		"UPDATE pixels SET status = %s, color = %s, url = %s, updated_at = %s, owner_id = %s WHERE id = %d",
		quoteLiteral(updated.Status),
		quoteLiteral(updated.Color),
		quoteLiteral(updated.URL),
		quoteLiteral(updated.UpdatedAt.Format(time.RFC3339Nano)),
		ownerValue,
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

func ensurePixelOwnerColumn(ctx context.Context, tx *sql.Tx) error {
	hasColumn, err := tableHasColumn(ctx, tx, "pixels", "owner_id")
	if err != nil {
		return err
	}
	if hasColumn {
		return nil
	}

	if _, execErr := tx.ExecContext(ctx, `ALTER TABLE pixels ADD COLUMN owner_id INTEGER`); execErr != nil {
		return fmt.Errorf("add owner_id column: %w", execErr)
	}
	return nil
}

func tableHasColumn(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, fmt.Errorf("pragma table_info %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return false, fmt.Errorf("scan table info %s: %w", table, err)
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table info %s: %w", table, err)
	}

	return false, nil
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (User, error) {
	if strings.TrimSpace(username) == "" {
		return User{}, errors.New("username must not be empty")
	}
	if passwordHash == "" {
		return User{}, errors.New("password hash must not be empty")
	}

	query := fmt.Sprintf(
		"INSERT INTO users(username, password_hash) VALUES (%s, %s)",
		quoteLiteral(username),
		quoteLiteral(passwordHash),
	)

	res, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("user id: %w", err)
	}

	user, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return User{}, err
	}
	user.ID = id
	return user, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {
	if strings.TrimSpace(username) == "" {
		return User{}, errors.New("username must not be empty")
	}

	query := fmt.Sprintf(
		"SELECT id, username, password_hash, created_at FROM users WHERE username = %s",
		quoteLiteral(username),
	)
	row := s.db.QueryRowContext(ctx, query)

	var (
		user      User
		createdAt string
	)
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, err
		}
		return User{}, fmt.Errorf("get user by username: %w", err)
	}

	parsed, err := parseUpdatedAt(createdAt)
	if err != nil {
		return User{}, fmt.Errorf("parse user created_at: %w", err)
	}
	user.CreatedAt = parsed
	return user, nil
}

func (s *Store) AssignPixelToUser(ctx context.Context, pixelID int, userID int64) error {
	if pixelID < 0 || pixelID >= TotalPixels {
		return fmt.Errorf("invalid pixel id: %d", pixelID)
	}
	query := fmt.Sprintf(
		"UPDATE pixels SET owner_id = %d, status = %s, updated_at = %s WHERE id = %d",
		userID,
		quoteLiteral("taken"),
		quoteLiteral(time.Now().UTC().Format(time.RFC3339Nano)),
		pixelID,
	)

	res, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("assign pixel to user: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("assign pixel rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListUserPixels(ctx context.Context, userID int64) ([]Pixel, error) {
	query := fmt.Sprintf(
		"SELECT id, status, COALESCE(color, ''), COALESCE(url, ''), updated_at, owner_id FROM pixels WHERE owner_id = %d ORDER BY id",
		userID,
	)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list user pixels: %w", err)
	}
	defer rows.Close()

	var pixels []Pixel
	for rows.Next() {
		var (
			pixel   Pixel
			updated sql.NullString
			owner   sql.NullInt64
		)
		if err := rows.Scan(&pixel.ID, &pixel.Status, &pixel.Color, &pixel.URL, &updated, &owner); err != nil {
			return nil, fmt.Errorf("scan user pixel: %w", err)
		}
		if updated.Valid {
			parsed, err := parseUpdatedAt(updated.String)
			if err != nil {
				return nil, fmt.Errorf("parse user pixel %d updated_at: %w", pixel.ID, err)
			}
			pixel.UpdatedAt = parsed
		}
		if owner.Valid {
			oid := int(owner.Int64)
			pixel.OwnerID = &oid
		}
		pixels = append(pixels, pixel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user pixels: %w", err)
	}

	return pixels, nil
}
