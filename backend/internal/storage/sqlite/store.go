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
	OwnerID   *int64    `json:"owner_id,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type User struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
}

func (u User) IsEmailVerified() bool {
	return u.EmailVerifiedAt != nil
}

var (
	ErrPixelOwnedByAnotherUser  = errors.New("pixel owned by another user")
	ErrVerificationTokenInvalid = errors.New("verification token invalid")
	ErrVerificationTokenExpired = errors.New("verification token expired")
)

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
                owner_id INTEGER,
                updated_at TIMESTAMP
        )`); execErr != nil {
		err = fmt.Errorf("create pixels table: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_pixels_status ON pixels(status)`); execErr != nil {
		err = fmt.Errorf("create status index: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                email TEXT NOT NULL UNIQUE,
                password_hash TEXT NOT NULL,
                created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                email_verified_at TIMESTAMP
        )`); execErr != nil {
		err = fmt.Errorf("create users table: %w", execErr)
		return err
	}

	// Attempt to add missing owner_id column for existing databases. Ignore errors if it already exists.
	if _, execErr := tx.ExecContext(ctx, `ALTER TABLE pixels ADD COLUMN owner_id INTEGER`); execErr != nil {
		// ignore error to keep compatibility with fresh schema
	}

	if _, execErr := tx.ExecContext(ctx, `ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMP`); execErr != nil {
		// ignore error for databases that already contain the column
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS email_verification_tokens (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                user_id INTEGER NOT NULL,
                token TEXT NOT NULL UNIQUE,
                created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                expires_at TIMESTAMP NOT NULL,
                FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
        )`); execErr != nil {
		err = fmt.Errorf("create verification tokens table: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_email_tokens_user_id ON email_verification_tokens(user_id)`); execErr != nil {
		err = fmt.Errorf("create verification token index: %w", execErr)
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
			query := fmt.Sprintf("INSERT OR IGNORE INTO pixels(id, status, color, url, owner_id, updated_at) VALUES (%d, 'free', '', '', NULL, CURRENT_TIMESTAMP)", i)
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
	rows, err := s.db.QueryContext(ctx, `SELECT id, status, COALESCE(color, ''), COALESCE(url, ''), owner_id, updated_at FROM pixels ORDER BY id`)
	if err != nil {
		return PixelState{}, fmt.Errorf("query pixels: %w", err)
	}
	defer rows.Close()

	pixels := make([]Pixel, 0, TotalPixels)
	for rows.Next() {
		var pixel Pixel
		var owner sql.NullInt64
		var updated sql.NullString
		if err := rows.Scan(&pixel.ID, &pixel.Status, &pixel.Color, &pixel.URL, &owner, &updated); err != nil {
			return PixelState{}, fmt.Errorf("scan pixel: %w", err)
		}
		if owner.Valid {
			ownerID := owner.Int64
			pixel.OwnerID = &ownerID
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
		updated.OwnerID = pixel.OwnerID
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

	var ownerValue string
	if updated.OwnerID != nil {
		ownerValue = fmt.Sprintf("%d", *updated.OwnerID)
	} else {
		ownerValue = "NULL"
	}

	query := fmt.Sprintf(
		"UPDATE pixels SET status = %s, color = %s, url = %s, owner_id = %s, updated_at = %s WHERE id = %d",
		quoteLiteral(updated.Status),
		quoteLiteral(updated.Color),
		quoteLiteral(updated.URL),
		ownerValue,
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

func (s *Store) UpdatePixelForUser(ctx context.Context, userID int64, pixel Pixel) (Pixel, error) {
	if userID <= 0 {
		return Pixel{}, errors.New("invalid user id")
	}
	if pixel.ID < 0 || pixel.ID >= TotalPixels {
		return Pixel{}, fmt.Errorf("invalid pixel id: %d", pixel.ID)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Pixel{}, fmt.Errorf("begin update pixel for user: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	query := fmt.Sprintf("SELECT owner_id FROM pixels WHERE id = %d", pixel.ID)
	row := tx.QueryRowContext(ctx, query)
	var currentOwner sql.NullInt64
	if err := row.Scan(&currentOwner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Pixel{}, sql.ErrNoRows
		}
		return Pixel{}, fmt.Errorf("load current pixel state: %w", err)
	}

	updated := Pixel{ID: pixel.ID}

	if strings.EqualFold(pixel.Status, "taken") {
		if pixel.Color == "" || pixel.URL == "" {
			return Pixel{}, errors.New("taken pixels require color and url")
		}
		if currentOwner.Valid && currentOwner.Int64 != userID {
			return Pixel{}, ErrPixelOwnedByAnotherUser
		}
		updated.Status = "taken"
		updated.Color = pixel.Color
		updated.URL = pixel.URL
		owner := userID
		updated.OwnerID = &owner
	} else {
		if currentOwner.Valid && currentOwner.Int64 != userID {
			return Pixel{}, ErrPixelOwnedByAnotherUser
		}
		updated.Status = "free"
		updated.Color = ""
		updated.URL = ""
		updated.OwnerID = nil
	}

	updated.UpdatedAt = time.Now().UTC()

	var ownerValue string
	if updated.OwnerID != nil {
		ownerValue = fmt.Sprintf("%d", *updated.OwnerID)
	} else {
		ownerValue = "NULL"
	}

	updateQuery := fmt.Sprintf(
		"UPDATE pixels SET status = %s, color = %s, url = %s, owner_id = %s, updated_at = %s WHERE id = %d",
		quoteLiteral(updated.Status),
		quoteLiteral(updated.Color),
		quoteLiteral(updated.URL),
		ownerValue,
		quoteLiteral(updated.UpdatedAt.Format(time.RFC3339Nano)),
		updated.ID,
	)

	res, err := tx.ExecContext(ctx, updateQuery)
	if err != nil {
		return Pixel{}, fmt.Errorf("update pixel for user: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return Pixel{}, fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return Pixel{}, sql.ErrNoRows
	}

	if err = tx.Commit(); err != nil {
		return Pixel{}, fmt.Errorf("commit update pixel for user: %w", err)
	}

	return updated, nil
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return User{}, errors.New("email must not be empty")
	}
	if passwordHash == "" {
		return User{}, errors.New("password hash must not be empty")
	}

	query := fmt.Sprintf(
		"INSERT INTO users(email, password_hash, created_at) VALUES (%s, %s, %s)",
		quoteLiteral(email),
		quoteLiteral(passwordHash),
		"CURRENT_TIMESTAMP",
	)

	res, err := s.db.ExecContext(ctx, query)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return User{}, fmt.Errorf("email already exists: %w", err)
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("last insert id: %w", err)
	}

	createdUser, err := s.GetUserByID(ctx, id)
	if err != nil {
		return User{}, fmt.Errorf("reload created user: %w", err)
	}
	return createdUser, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return User{}, errors.New("email must not be empty")
	}

	query := fmt.Sprintf(
		"SELECT id, email, password_hash, created_at, email_verified_at FROM users WHERE email = %s",
		quoteLiteral(email),
	)

	row := s.db.QueryRowContext(ctx, query)
	var user User
	var created sql.NullString
	var verified sql.NullString
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &created, &verified); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, sql.ErrNoRows
		}
		return User{}, fmt.Errorf("scan user by email: %w", err)
	}

	if !created.Valid || strings.TrimSpace(created.String) == "" {
		return User{}, errors.New("user missing created_at")
	}
	parsedCreated, err := parseUpdatedAt(created.String)
	if err != nil {
		return User{}, fmt.Errorf("parse created_at: %w", err)
	}
	user.CreatedAt = parsedCreated

	if verified.Valid && strings.TrimSpace(verified.String) != "" {
		parsedVerified, parseErr := parseUpdatedAt(verified.String)
		if parseErr != nil {
			return User{}, fmt.Errorf("parse email_verified_at: %w", parseErr)
		}
		user.EmailVerifiedAt = &parsedVerified
	}
	return user, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	if id <= 0 {
		return User{}, errors.New("invalid user id")
	}

	query := fmt.Sprintf("SELECT id, email, password_hash, created_at, email_verified_at FROM users WHERE id = %d", id)
	row := s.db.QueryRowContext(ctx, query)
	var user User
	var created sql.NullString
	var verified sql.NullString
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &created, &verified); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, sql.ErrNoRows
		}
		return User{}, fmt.Errorf("scan user by id: %w", err)
	}

	if !created.Valid || strings.TrimSpace(created.String) == "" {
		return User{}, errors.New("user missing created_at")
	}
	parsedCreated, err := parseUpdatedAt(created.String)
	if err != nil {
		return User{}, fmt.Errorf("parse created_at: %w", err)
	}
	user.CreatedAt = parsedCreated

	if verified.Valid && strings.TrimSpace(verified.String) != "" {
		parsedVerified, parseErr := parseUpdatedAt(verified.String)
		if parseErr != nil {
			return User{}, fmt.Errorf("parse email_verified_at: %w", parseErr)
		}
		user.EmailVerifiedAt = &parsedVerified
	}
	return user, nil
}

func (s *Store) CreateEmailVerificationToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token must not be empty")
	}
	if expiresAt.IsZero() {
		return errors.New("expires at must not be zero")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create verification token: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	deleteQuery := fmt.Sprintf("DELETE FROM email_verification_tokens WHERE user_id = %d", userID)
	if _, execErr := tx.ExecContext(ctx, deleteQuery); execErr != nil {
		err = fmt.Errorf("delete existing verification tokens: %w", execErr)
		return err
	}

	insertQuery := fmt.Sprintf(
		"INSERT INTO email_verification_tokens(user_id, token, expires_at) VALUES (%d, %s, %s)",
		userID,
		quoteLiteral(token),
		quoteLiteral(expiresAt.UTC().Format(time.RFC3339Nano)),
	)
	if _, execErr := tx.ExecContext(ctx, insertQuery); execErr != nil {
		if strings.Contains(strings.ToLower(execErr.Error()), "unique") {
			err = fmt.Errorf("verification token already exists: %w", execErr)
			return err
		}
		err = fmt.Errorf("insert verification token: %w", execErr)
		return err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("commit verification token: %w", commitErr)
	}
	return nil
}

func (s *Store) VerifyUserByToken(ctx context.Context, token string) (user User, err error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return User{}, errors.New("token must not be empty")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin verify token: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	query := fmt.Sprintf(
		"SELECT user_id, expires_at FROM email_verification_tokens WHERE token = %s",
		quoteLiteral(token),
	)

	var userID int64
	var expiresRaw string
	if scanErr := tx.QueryRowContext(ctx, query).Scan(&userID, &expiresRaw); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			err = ErrVerificationTokenInvalid
			return User{}, err
		}
		err = fmt.Errorf("load verification token: %w", scanErr)
		return User{}, err
	}

	expiresAt, parseErr := parseUpdatedAt(expiresRaw)
	if parseErr != nil {
		err = fmt.Errorf("parse token expiration: %w", parseErr)
		return User{}, err
	}

	if time.Now().UTC().After(expiresAt) {
		deleteQuery := fmt.Sprintf("DELETE FROM email_verification_tokens WHERE token = %s", quoteLiteral(token))
		if _, execErr := tx.ExecContext(ctx, deleteQuery); execErr != nil {
			err = fmt.Errorf("delete expired verification token: %w", execErr)
			return User{}, err
		}
		err = ErrVerificationTokenExpired
		return User{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	updateQuery := fmt.Sprintf(
		"UPDATE users SET email_verified_at = %s WHERE id = %d",
		quoteLiteral(now),
		userID,
	)
	if _, execErr := tx.ExecContext(ctx, updateQuery); execErr != nil {
		err = fmt.Errorf("update user verification: %w", execErr)
		return User{}, err
	}

	cleanupQuery := fmt.Sprintf("DELETE FROM email_verification_tokens WHERE user_id = %d", userID)
	if _, execErr := tx.ExecContext(ctx, cleanupQuery); execErr != nil {
		err = fmt.Errorf("delete user verification tokens: %w", execErr)
		return User{}, err
	}

	selectQuery := fmt.Sprintf(
		"SELECT id, email, password_hash, created_at, email_verified_at FROM users WHERE id = %d",
		userID,
	)
	var created sql.NullString
	var verified sql.NullString
	if scanErr := tx.QueryRowContext(ctx, selectQuery).Scan(&user.ID, &user.Email, &user.PasswordHash, &created, &verified); scanErr != nil {
		err = fmt.Errorf("reload verified user: %w", scanErr)
		return User{}, err
	}

	if !created.Valid || strings.TrimSpace(created.String) == "" {
		err = errors.New("verified user missing created_at")
		return User{}, err
	}
	parsedCreated, parseCreatedErr := parseUpdatedAt(created.String)
	if parseCreatedErr != nil {
		err = fmt.Errorf("parse verified user created_at: %w", parseCreatedErr)
		return User{}, err
	}
	user.CreatedAt = parsedCreated

	if verified.Valid && strings.TrimSpace(verified.String) != "" {
		parsedVerified, parseVerifiedErr := parseUpdatedAt(verified.String)
		if parseVerifiedErr != nil {
			err = fmt.Errorf("parse verified user email_verified_at: %w", parseVerifiedErr)
			return User{}, err
		}
		user.EmailVerifiedAt = &parsedVerified
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return User{}, fmt.Errorf("commit verify token: %w", commitErr)
	}
	return user, nil
}

func quoteLiteral(value string) string {
	escaped := strings.ReplaceAll(value, "'", "''")
	return "'" + escaped + "'"
}
