package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/kup-piksel/internal/storage"
	_ "github.com/mattn/go-sqlite3"
)

const busyTimeoutMs = 5000

type (
	Pixel              = storage.Pixel
	User               = storage.User
	VerificationToken  = storage.VerificationToken
	PasswordResetToken = storage.PasswordResetToken
	PixelState         = storage.PixelState
)

type Store struct {
	db            *sql.DB
	skipPixelSeed bool
}

var _ storage.Store = (*Store)(nil)

func (s *Store) GetPixelsByOwner(ctx context.Context, ownerID int64) ([]Pixel, error) {
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}

	query := fmt.Sprintf(
		"SELECT id, status, COALESCE(color, ''), COALESCE(url, ''), owner_id, updated_at FROM pixels WHERE owner_id = %d ORDER BY updated_at DESC",
		ownerID,
	)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query pixels by owner: %w", err)
	}
	defer rows.Close()

	pixels := make([]Pixel, 0)
	for rows.Next() {
		var pixel Pixel
		var owner sql.NullInt64
		var updated sql.NullString
		if err := rows.Scan(&pixel.ID, &pixel.Status, &pixel.Color, &pixel.URL, &owner, &updated); err != nil {
			return nil, fmt.Errorf("scan pixel: %w", err)
		}
		if owner.Valid {
			ownerID := owner.Int64
			pixel.OwnerID = &ownerID
		}
		if updated.Valid {
			parsed, err := parseUpdatedAt(updated.String)
			if err != nil {
				return nil, fmt.Errorf("parse pixel %d updated_at: %w", pixel.ID, err)
			}
			pixel.UpdatedAt = parsed
		}
		pixels = append(pixels, pixel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pixels by owner: %w", err)
	}

	return pixels, nil
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

// SetSkipPixelSeed configures the store to skip the initial pixel seeding step during EnsureSchema.
// This is primarily useful for tests where populating the full grid would be prohibitively slow.
func (s *Store) SetSkipPixelSeed(skip bool) {
	if s != nil {
		s.skipPixelSeed = skip
	}
}

// InsertPixel ensures a pixel row exists with the provided attributes. It is intended for tests where
// the full grid is not populated.
func (s *Store) InsertPixel(ctx context.Context, pixel Pixel) error {
	if pixel.ID < 0 || pixel.ID >= storage.TotalPixels {
		return fmt.Errorf("invalid pixel id: %d", pixel.ID)
	}

	status := "free"
	if strings.TrimSpace(pixel.Status) != "" {
		status = pixel.Status
	}
	color := strings.TrimSpace(pixel.Color)
	url := strings.TrimSpace(pixel.URL)

	ownerValue := "NULL"
	if pixel.OwnerID != nil {
		ownerValue = fmt.Sprintf("%d", *pixel.OwnerID)
	}

	query := fmt.Sprintf(
		"INSERT OR IGNORE INTO pixels(id, status, color, url, owner_id, updated_at) VALUES (%d, %s, %s, %s, %s, CURRENT_TIMESTAMP)",
		pixel.ID,
		quoteLiteral(status),
		quoteLiteral(color),
		quoteLiteral(url),
		ownerValue,
	)

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("insert pixel: %w", err)
	}
	return nil
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
                is_verified INTEGER NOT NULL DEFAULT 0,
                verified_at TIMESTAMP,
                user_points INTEGER NOT NULL DEFAULT 0
        )`); execErr != nil {
		err = fmt.Errorf("create users table: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `ALTER TABLE users ADD COLUMN is_verified INTEGER NOT NULL DEFAULT 0`); execErr != nil {
		// ignore - column may already exist
	}

	if _, execErr := tx.ExecContext(ctx, `ALTER TABLE users ADD COLUMN verified_at TIMESTAMP`); execErr != nil {
		// ignore - column may already exist
	}

	if _, execErr := tx.ExecContext(ctx, `ALTER TABLE users ADD COLUMN user_points INTEGER NOT NULL DEFAULT 0`); execErr != nil {
		// ignore - column may already exist
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS activation_codes (
                code TEXT PRIMARY KEY,
                value INTEGER NOT NULL
        )`); execErr != nil {
		err = fmt.Errorf("create activation_codes table: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS verification_tokens (
                token TEXT PRIMARY KEY,
                user_id INTEGER NOT NULL,
                expires_at TIMESTAMP NOT NULL,
                created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
        )`); execErr != nil {
		err = fmt.Errorf("create verification_tokens table: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_verification_tokens_user ON verification_tokens(user_id)`); execErr != nil {
		err = fmt.Errorf("create verification token index: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS password_reset_tokens (
                token TEXT PRIMARY KEY,
                user_id INTEGER NOT NULL,
                expires_at TIMESTAMP NOT NULL,
                created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
        )`); execErr != nil {
		err = fmt.Errorf("create password_reset_tokens table: %w", execErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens(user_id)`); execErr != nil {
		err = fmt.Errorf("create password reset token index: %w", execErr)
		return err
	}

	// Attempt to add missing owner_id column for existing databases. Ignore errors if it already exists.
	if _, execErr := tx.ExecContext(ctx, `ALTER TABLE pixels ADD COLUMN owner_id INTEGER`); execErr != nil {
		// ignore error to keep compatibility with fresh schema
	}

	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM pixels`).Scan(&count); err != nil {
		err = fmt.Errorf("count pixels: %w", err)
		return err
	}

	if count == 0 && !s.skipPixelSeed {
		for i := 0; i < storage.TotalPixels; i++ {
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

	pixels := make([]Pixel, 0, storage.TotalPixels)
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

	return PixelState{Width: storage.GridWidth, Height: storage.GridHeight, Pixels: pixels}, nil
}

func (s *Store) UpdatePixel(ctx context.Context, pixel Pixel) (updated Pixel, err error) {
	if pixel.ID < 0 || pixel.ID >= storage.TotalPixels {
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

func (s *Store) UpdatePixelForUserWithCost(ctx context.Context, userID int64, pixel Pixel, cost int64) (updated Pixel, updatedUser User, err error) {
	if userID <= 0 {
		return Pixel{}, User{}, errors.New("invalid user id")
	}
	if pixel.ID < 0 || pixel.ID >= storage.TotalPixels {
		return Pixel{}, User{}, fmt.Errorf("invalid pixel id: %d", pixel.ID)
	}
	if cost < 0 {
		return Pixel{}, User{}, errors.New("cost must not be negative")
	}

	var tx *sql.Tx
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		return Pixel{}, User{}, fmt.Errorf("begin update pixel for user: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	pointsQuery := fmt.Sprintf("SELECT user_points FROM users WHERE id = %d", userID)
	var currentPoints int64
	if scanErr := tx.QueryRowContext(ctx, pointsQuery).Scan(&currentPoints); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			err = sql.ErrNoRows
			return Pixel{}, User{}, err
		}
		err = fmt.Errorf("load user points: %w", scanErr)
		return Pixel{}, User{}, err
	}

	ownerQuery := fmt.Sprintf("SELECT owner_id FROM pixels WHERE id = %d", pixel.ID)
	row := tx.QueryRowContext(ctx, ownerQuery)
	var currentOwner sql.NullInt64
	if scanErr := row.Scan(&currentOwner); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			err = sql.ErrNoRows
			return Pixel{}, User{}, err
		}
		err = fmt.Errorf("load current pixel state: %w", scanErr)
		return Pixel{}, User{}, err
	}

	updated = Pixel{ID: pixel.ID}
	chargeCost := false

	if strings.EqualFold(pixel.Status, "taken") {
		if pixel.Color == "" || pixel.URL == "" {
			err = errors.New("taken pixels require color and url")
			return Pixel{}, User{}, err
		}
		if currentOwner.Valid && currentOwner.Int64 != userID {
			err = storage.ErrPixelOwnedByAnotherUser
			return Pixel{}, User{}, err
		}
		if !currentOwner.Valid || currentOwner.Int64 != userID {
			chargeCost = cost > 0
		}
		updated.Status = "taken"
		updated.Color = pixel.Color
		updated.URL = pixel.URL
		owner := userID
		updated.OwnerID = &owner
	} else {
		if currentOwner.Valid && currentOwner.Int64 != userID {
			err = storage.ErrPixelOwnedByAnotherUser
			return Pixel{}, User{}, err
		}
		updated.Status = "free"
		updated.Color = ""
		updated.URL = ""
		updated.OwnerID = nil
	}

	if chargeCost {
		if currentPoints < cost {
			err = storage.ErrInsufficientPoints
			return Pixel{}, User{}, err
		}
		chargeQuery := fmt.Sprintf("UPDATE users SET user_points = user_points - %d WHERE id = %d AND user_points >= %d", cost, userID, cost)
		res, execErr := tx.ExecContext(ctx, chargeQuery)
		if execErr != nil {
			err = fmt.Errorf("deduct user points: %w", execErr)
			return Pixel{}, User{}, err
		}
		affected, affErr := res.RowsAffected()
		if affErr != nil {
			err = fmt.Errorf("deduct user points rows affected: %w", affErr)
			return Pixel{}, User{}, err
		}
		if affected == 0 {
			err = storage.ErrInsufficientPoints
			return Pixel{}, User{}, err
		}
		currentPoints -= cost
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

	res, execErr := tx.ExecContext(ctx, updateQuery)
	if execErr != nil {
		err = fmt.Errorf("update pixel for user: %w", execErr)
		return Pixel{}, User{}, err
	}

	affected, affErr := res.RowsAffected()
	if affErr != nil {
		err = fmt.Errorf("rows affected: %w", affErr)
		return Pixel{}, User{}, err
	}
	if affected == 0 {
		err = sql.ErrNoRows
		return Pixel{}, User{}, err
	}

	userQuery := fmt.Sprintf("SELECT id, email, password_hash, created_at, is_verified, verified_at, user_points FROM users WHERE id = %d", userID)
	userRow := tx.QueryRowContext(ctx, userQuery)
	updatedUser, scanErr := scanUser(userRow)
	if scanErr != nil {
		err = scanErr
		return Pixel{}, User{}, err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = fmt.Errorf("commit update pixel for user: %w", commitErr)
		return Pixel{}, User{}, err
	}

	return updated, updatedUser, nil
}

func (s *Store) UpdatePixelForUser(ctx context.Context, userID int64, pixel Pixel) (Pixel, error) {
	updated, _, err := s.UpdatePixelForUserWithCost(ctx, userID, pixel, 0)
	return updated, err
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
		"INSERT INTO users(email, password_hash, created_at, is_verified) VALUES (%s, %s, %s, 0)",
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

	user := User{ID: id, Email: email, PasswordHash: passwordHash, CreatedAt: time.Now().UTC(), IsVerified: false, Points: 0}
	return user, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return User{}, errors.New("email must not be empty")
	}

	query := fmt.Sprintf(
		"SELECT id, email, password_hash, created_at, is_verified, verified_at, user_points FROM users WHERE email = %s",
		quoteLiteral(email),
	)

	row := s.db.QueryRowContext(ctx, query)
	user, err := scanUser(row)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	if id <= 0 {
		return User{}, errors.New("invalid user id")
	}

	query := fmt.Sprintf("SELECT id, email, password_hash, created_at, is_verified, verified_at, user_points FROM users WHERE id = %d", id)
	row := s.db.QueryRowContext(ctx, query)
	user, err := scanUser(row)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	var created string
	var isVerified int64
	var verified sql.NullString
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &created, &isVerified, &verified, &user.Points); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, sql.ErrNoRows
		}
		return User{}, fmt.Errorf("scan user: %w", err)
	}

	parsed, err := parseUpdatedAt(created)
	if err != nil {
		return User{}, fmt.Errorf("parse created_at: %w", err)
	}
	user.CreatedAt = parsed
	user.IsVerified = isVerified != 0
	if verified.Valid && strings.TrimSpace(verified.String) != "" {
		parsedVerified, parseErr := parseUpdatedAt(verified.String)
		if parseErr != nil {
			return User{}, fmt.Errorf("parse verified_at: %w", parseErr)
		}
		user.VerifiedAt = &parsedVerified
	}
	return user, nil
}

func (s *Store) CreateActivationCode(ctx context.Context, code string, value int64) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("activation code must not be empty")
	}
	if value <= 0 {
		return errors.New("activation code value must be positive")
	}

	query := fmt.Sprintf(
		"INSERT INTO activation_codes(code, value) VALUES (%s, %d)",
		quoteLiteral(strings.ToUpper(code)),
		value,
	)

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("activation code already exists: %w", err)
		}
		return fmt.Errorf("insert activation code: %w", err)
	}
	return nil
}

func (s *Store) RedeemActivationCode(ctx context.Context, userID int64, code string) (User, int64, error) {
	if userID <= 0 {
		return User{}, 0, errors.New("invalid user id")
	}
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "" {
		return User{}, 0, errors.New("activation code must not be empty")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, 0, fmt.Errorf("begin redeem activation code: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	selectQuery := fmt.Sprintf("SELECT value FROM activation_codes WHERE code = %s", quoteLiteral(normalized))
	row := tx.QueryRowContext(ctx, selectQuery)
	var value int64
	if scanErr := row.Scan(&value); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			err = sql.ErrNoRows
			return User{}, 0, err
		}
		err = fmt.Errorf("load activation code: %w", scanErr)
		return User{}, 0, err
	}

	deleteQuery := fmt.Sprintf("DELETE FROM activation_codes WHERE code = %s", quoteLiteral(normalized))
	res, execErr := tx.ExecContext(ctx, deleteQuery)
	if execErr != nil {
		err = fmt.Errorf("delete activation code: %w", execErr)
		return User{}, 0, err
	}
	affected, affErr := res.RowsAffected()
	if affErr != nil {
		err = fmt.Errorf("activation code rows affected: %w", affErr)
		return User{}, 0, err
	}
	if affected == 0 {
		err = sql.ErrNoRows
		return User{}, 0, err
	}

	updateQuery := fmt.Sprintf("UPDATE users SET user_points = user_points + %d WHERE id = %d", value, userID)
	res, execErr = tx.ExecContext(ctx, updateQuery)
	if execErr != nil {
		err = fmt.Errorf("update user points: %w", execErr)
		return User{}, 0, err
	}
	affected, affErr = res.RowsAffected()
	if affErr != nil {
		err = fmt.Errorf("user points rows affected: %w", affErr)
		return User{}, 0, err
	}
	if affected == 0 {
		err = sql.ErrNoRows
		return User{}, 0, err
	}

	userQuery := fmt.Sprintf("SELECT id, email, password_hash, created_at, is_verified, verified_at, user_points FROM users WHERE id = %d", userID)
	userRow := tx.QueryRowContext(ctx, userQuery)
	updatedUser, scanErr := scanUser(userRow)
	if scanErr != nil {
		err = scanErr
		return User{}, 0, err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = fmt.Errorf("commit redeem activation code: %w", commitErr)
		return User{}, 0, err
	}

	return updatedUser, value, nil
}

func (s *Store) CreateVerificationToken(ctx context.Context, token string, userID int64, expiresAt time.Time) (VerificationToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return VerificationToken{}, errors.New("token must not be empty")
	}
	if userID <= 0 {
		return VerificationToken{}, errors.New("invalid user id")
	}

	created := time.Now().UTC()
	query := fmt.Sprintf(
		"INSERT INTO verification_tokens(token, user_id, expires_at, created_at) VALUES (%s, %d, %s, %s)",
		quoteLiteral(token),
		userID,
		quoteLiteral(expiresAt.UTC().Format(time.RFC3339Nano)),
		quoteLiteral(created.Format(time.RFC3339Nano)),
	)

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return VerificationToken{}, fmt.Errorf("token already exists: %w", err)
		}
		return VerificationToken{}, fmt.Errorf("insert verification token: %w", err)
	}

	return VerificationToken{Token: token, UserID: userID, ExpiresAt: expiresAt.UTC(), CreatedAt: created}, nil
}

func (s *Store) GetVerificationToken(ctx context.Context, token string) (VerificationToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return VerificationToken{}, errors.New("token must not be empty")
	}

	query := fmt.Sprintf(
		"SELECT token, user_id, expires_at, created_at FROM verification_tokens WHERE token = %s",
		quoteLiteral(token),
	)

	row := s.db.QueryRowContext(ctx, query)
	var vt VerificationToken
	var expires string
	var created string
	if err := row.Scan(&vt.Token, &vt.UserID, &expires, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return VerificationToken{}, sql.ErrNoRows
		}
		return VerificationToken{}, fmt.Errorf("scan verification token: %w", err)
	}

	parsedExpires, err := parseUpdatedAt(expires)
	if err != nil {
		return VerificationToken{}, fmt.Errorf("parse expires_at: %w", err)
	}
	vt.ExpiresAt = parsedExpires

	parsedCreated, err := parseUpdatedAt(created)
	if err != nil {
		return VerificationToken{}, fmt.Errorf("parse created_at: %w", err)
	}
	vt.CreatedAt = parsedCreated

	return vt, nil
}

func (s *Store) DeleteVerificationToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token must not be empty")
	}

	query := fmt.Sprintf("DELETE FROM verification_tokens WHERE token = %s", quoteLiteral(token))
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("delete verification token: %w", err)
	}
	return nil
}

func (s *Store) DeleteVerificationTokensForUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}

	query := fmt.Sprintf("DELETE FROM verification_tokens WHERE user_id = %d", userID)
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("delete tokens for user: %w", err)
	}
	return nil
}

func (s *Store) MarkUserVerified(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}

	query := fmt.Sprintf(
		"UPDATE users SET is_verified = 1, verified_at = %s WHERE id = %d",
		quoteLiteral(time.Now().UTC().Format(time.RFC3339Nano)),
		userID,
	)

	res, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("mark user verified: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected mark user verified: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *Store) CreatePasswordResetToken(ctx context.Context, token string, userID int64, expiresAt time.Time) (PasswordResetToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return PasswordResetToken{}, errors.New("token must not be empty")
	}
	if userID <= 0 {
		return PasswordResetToken{}, errors.New("invalid user id")
	}

	created := time.Now().UTC()
	query := fmt.Sprintf(
		"INSERT INTO password_reset_tokens(token, user_id, expires_at, created_at) VALUES (%s, %d, %s, %s)",
		quoteLiteral(token),
		userID,
		quoteLiteral(expiresAt.UTC().Format(time.RFC3339Nano)),
		quoteLiteral(created.Format(time.RFC3339Nano)),
	)

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return PasswordResetToken{}, fmt.Errorf("token already exists: %w", err)
		}
		return PasswordResetToken{}, fmt.Errorf("insert password reset token: %w", err)
	}

	return PasswordResetToken{Token: token, UserID: userID, ExpiresAt: expiresAt.UTC(), CreatedAt: created}, nil
}

func (s *Store) GetPasswordResetToken(ctx context.Context, token string) (PasswordResetToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return PasswordResetToken{}, errors.New("token must not be empty")
	}

	query := fmt.Sprintf(
		"SELECT token, user_id, expires_at, created_at FROM password_reset_tokens WHERE token = %s",
		quoteLiteral(token),
	)

	row := s.db.QueryRowContext(ctx, query)
	var prt PasswordResetToken
	var expires string
	var created string
	if err := row.Scan(&prt.Token, &prt.UserID, &expires, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasswordResetToken{}, sql.ErrNoRows
		}
		return PasswordResetToken{}, fmt.Errorf("scan password reset token: %w", err)
	}

	parsedExpires, err := parseUpdatedAt(expires)
	if err != nil {
		return PasswordResetToken{}, fmt.Errorf("parse expires_at: %w", err)
	}
	prt.ExpiresAt = parsedExpires

	parsedCreated, err := parseUpdatedAt(created)
	if err != nil {
		return PasswordResetToken{}, fmt.Errorf("parse created_at: %w", err)
	}
	prt.CreatedAt = parsedCreated

	return prt, nil
}

func (s *Store) DeletePasswordResetToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token must not be empty")
	}

	query := fmt.Sprintf("DELETE FROM password_reset_tokens WHERE token = %s", quoteLiteral(token))
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("delete password reset token: %w", err)
	}
	return nil
}

func (s *Store) DeletePasswordResetTokensForUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}

	query := fmt.Sprintf("DELETE FROM password_reset_tokens WHERE user_id = %d", userID)
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("delete password reset tokens for user: %w", err)
	}
	return nil
}

func (s *Store) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	if strings.TrimSpace(passwordHash) == "" {
		return errors.New("password hash must not be empty")
	}

	query := fmt.Sprintf(
		"UPDATE users SET password_hash = %s WHERE id = %d",
		quoteLiteral(passwordHash),
		userID,
	)
	res, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected update user password: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func quoteLiteral(value string) string {
	escaped := strings.ReplaceAll(value, "'", "''")
	return "'" + escaped + "'"
}
