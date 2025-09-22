package mysql

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/example/kup-piksel/internal/storage"
	_ "github.com/go-sql-driver/mysql"
)

type (
	Pixel             = storage.Pixel
	User              = storage.User
	VerificationToken = storage.VerificationToken
	PixelState        = storage.PixelState
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db            *sql.DB
	skipPixelSeed bool
}

var _ storage.Store = (*Store)(nil)

func Open(dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("mysql dsn must not be empty")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql connection: %w", err)
	}

	db.SetConnMaxIdleTime(2 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) SetSkipPixelSeed(skip bool) {
	if s != nil {
		s.skipPixelSeed = skip
	}
}

func (s *Store) InsertPixel(ctx context.Context, pixel Pixel) error {
	if pixel.ID < 0 || pixel.ID >= storage.TotalPixels {
		return fmt.Errorf("invalid pixel id: %d", pixel.ID)
	}

	status := "free"
	if trimmed := strings.TrimSpace(pixel.Status); trimmed != "" {
		status = trimmed
	}

	color := strings.TrimSpace(pixel.Color)
	url := strings.TrimSpace(pixel.URL)
	var owner any
	if pixel.OwnerID != nil {
		owner = *pixel.OwnerID
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO pixels (id, status, color, url, owner_id, updated_at)
                 VALUES (?, ?, ?, ?, ?, ?)
                 ON DUPLICATE KEY UPDATE id = id`,
		pixel.ID,
		status,
		nullableString(color),
		nullableString(url),
		owner,
		time.Now().UTC(),
	)
	if err != nil {
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

	statements, loadErr := readMigrationStatements()
	if loadErr != nil {
		err = fmt.Errorf("load migrations: %w", loadErr)
		return err
	}

	for _, stmt := range statements {
		if _, execErr := tx.ExecContext(ctx, stmt); execErr != nil {
			err = fmt.Errorf("ensure schema statement failed: %w", execErr)
			return err
		}
	}

	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM pixels`).Scan(&count); err != nil {
		err = fmt.Errorf("count pixels: %w", err)
		return err
	}

	if count < storage.TotalPixels && !s.skipPixelSeed {
		const batchSize = 1000

		now := time.Now().UTC()
		base := "INSERT INTO pixels (id, status, color, url, owner_id, updated_at) VALUES "
		suffix := " ON DUPLICATE KEY UPDATE id = id"

		for start := 0; start < storage.TotalPixels; start += batchSize {
			if ctx.Err() != nil {
				err = ctx.Err()
				return err
			}

			end := start + batchSize
			if end > storage.TotalPixels {
				end = storage.TotalPixels
			}

			var builder strings.Builder
			builder.Grow(len(base) + len(suffix) + (end-start)*len("(?, 'free', '', '', NULL, ?),"))
			builder.WriteString(base)

			args := make([]any, 0, 2*(end-start))
			for i := start; i < end; i++ {
				if i > start {
					builder.WriteByte(',')
				}
				builder.WriteString("(?, 'free', '', '', NULL, ?)")
				args = append(args, i, now)
			}
			builder.WriteString(suffix)

			if _, execErr := tx.ExecContext(ctx, builder.String(), args...); execErr != nil {
				err = fmt.Errorf("seed pixel batch starting at %d: %w", start, execErr)
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

func readMigrationStatements() ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var statements []string
	for _, entry := range entries {
		data, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}
		statements = append(statements, splitStatements(string(data))...)
	}
	return statements, nil
}

func splitStatements(input string) []string {
	raw := strings.Split(input, ";")
	statements := make([]string, 0, len(raw))
	for _, stmt := range raw {
		trimmed := strings.TrimSpace(stmt)
		if trimmed != "" {
			statements = append(statements, trimmed)
		}
	}
	return statements
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s *Store) GetPixelsByOwner(ctx context.Context, ownerID int64) ([]Pixel, error) {
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, status, COALESCE(color, ''), COALESCE(url, ''), owner_id, updated_at FROM pixels WHERE owner_id = ? ORDER BY updated_at DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("query pixels by owner: %w", err)
	}
	defer rows.Close()

	pixels := make([]Pixel, 0)
	for rows.Next() {
		var pixel Pixel
		var owner sql.NullInt64
		var updated sql.NullTime
		if err := rows.Scan(&pixel.ID, &pixel.Status, &pixel.Color, &pixel.URL, &owner, &updated); err != nil {
			return nil, fmt.Errorf("scan pixel: %w", err)
		}
		if owner.Valid {
			oid := owner.Int64
			pixel.OwnerID = &oid
		}
		if updated.Valid {
			pixel.UpdatedAt = updated.Time.UTC()
		}
		pixels = append(pixels, pixel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pixels by owner: %w", err)
	}

	return pixels, nil
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
		var updated sql.NullTime
		if err := rows.Scan(&pixel.ID, &pixel.Status, &pixel.Color, &pixel.URL, &owner, &updated); err != nil {
			return PixelState{}, fmt.Errorf("scan pixel: %w", err)
		}
		if owner.Valid {
			oid := owner.Int64
			pixel.OwnerID = &oid
		}
		if updated.Valid {
			pixel.UpdatedAt = updated.Time.UTC()
		}
		pixels = append(pixels, pixel)
	}

	if err := rows.Err(); err != nil {
		return PixelState{}, fmt.Errorf("iterate pixels: %w", err)
	}

	return PixelState{Width: storage.GridWidth, Height: storage.GridHeight, Pixels: pixels}, nil
}

func (s *Store) UpdatePixel(ctx context.Context, pixel Pixel) (Pixel, error) {
	updated, _, err := s.UpdatePixelForUserWithCost(ctx, 0, pixel, 0)
	if err != nil {
		return Pixel{}, err
	}
	return updated, nil
}

func (s *Store) UpdatePixelForUserWithCost(ctx context.Context, userID int64, pixel Pixel, cost int64) (Pixel, User, error) {
	if pixel.ID < 0 || pixel.ID >= storage.TotalPixels {
		return Pixel{}, User{}, fmt.Errorf("invalid pixel id: %d", pixel.ID)
	}
	if cost < 0 {
		return Pixel{}, User{}, errors.New("cost must not be negative")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Pixel{}, User{}, fmt.Errorf("begin update pixel for user: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var currentPoints int64
	if userID > 0 {
		if err = tx.QueryRowContext(ctx, `SELECT user_points FROM users WHERE id = ?`, userID).Scan(&currentPoints); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Pixel{}, User{}, sql.ErrNoRows
			}
			return Pixel{}, User{}, fmt.Errorf("load user points: %w", err)
		}
	}

	var currentOwner sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT owner_id FROM pixels WHERE id = ?`, pixel.ID).Scan(&currentOwner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Pixel{}, User{}, sql.ErrNoRows
		}
		return Pixel{}, User{}, fmt.Errorf("load pixel owner: %w", err)
	}

	updated := Pixel{ID: pixel.ID}
	chargeCost := false

	if strings.EqualFold(pixel.Status, "taken") {
		if pixel.Color == "" || pixel.URL == "" {
			return Pixel{}, User{}, errors.New("taken pixels require color and url")
		}
		if currentOwner.Valid && userID > 0 && currentOwner.Int64 != userID {
			return Pixel{}, User{}, storage.ErrPixelOwnedByAnotherUser
		}
		if (!currentOwner.Valid && cost > 0) || (currentOwner.Valid && userID > 0 && currentOwner.Int64 != userID) {
			chargeCost = cost > 0
		}
		updated.Status = "taken"
		updated.Color = pixel.Color
		updated.URL = pixel.URL
		if userID > 0 {
			owner := userID
			updated.OwnerID = &owner
		}
	} else {
		if currentOwner.Valid && userID > 0 && currentOwner.Int64 != userID {
			return Pixel{}, User{}, storage.ErrPixelOwnedByAnotherUser
		}
		updated.Status = "free"
		updated.Color = ""
		updated.URL = ""
		updated.OwnerID = nil
	}

	if chargeCost {
		if currentPoints < cost {
			return Pixel{}, User{}, storage.ErrInsufficientPoints
		}
		res, execErr := tx.ExecContext(ctx, `UPDATE users SET user_points = user_points - ? WHERE id = ? AND user_points >= ?`, cost, userID, cost)
		if execErr != nil {
			return Pixel{}, User{}, fmt.Errorf("deduct user points: %w", execErr)
		}
		affected, affErr := res.RowsAffected()
		if affErr != nil {
			return Pixel{}, User{}, fmt.Errorf("deduct user points rows affected: %w", affErr)
		}
		if affected == 0 {
			return Pixel{}, User{}, storage.ErrInsufficientPoints
		}
		currentPoints -= cost
	}

	updated.UpdatedAt = time.Now().UTC()
	var owner any
	if updated.OwnerID != nil {
		owner = *updated.OwnerID
	}

	res, execErr := tx.ExecContext(
		ctx,
		`UPDATE pixels SET status = ?, color = ?, url = ?, owner_id = ?, updated_at = ? WHERE id = ?`,
		updated.Status,
		updated.Color,
		updated.URL,
		owner,
		updated.UpdatedAt,
		updated.ID,
	)
	if execErr != nil {
		return Pixel{}, User{}, fmt.Errorf("update pixel: %w", execErr)
	}
	affected, affErr := res.RowsAffected()
	if affErr != nil {
		return Pixel{}, User{}, fmt.Errorf("update pixel rows affected: %w", affErr)
	}
	if affected == 0 {
		return Pixel{}, User{}, sql.ErrNoRows
	}

	var updatedUser User
	if userID > 0 {
		row := tx.QueryRowContext(ctx, `SELECT id, email, password_hash, created_at, is_verified, verified_at, user_points FROM users WHERE id = ?`, userID)
		updatedUser, err = scanUser(row)
		if err != nil {
			return Pixel{}, User{}, err
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return Pixel{}, User{}, fmt.Errorf("commit update pixel for user: %w", commitErr)
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

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO users (email, password_hash, created_at, is_verified) VALUES (?, ?, ?, FALSE)`, email, passwordHash, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return User{}, fmt.Errorf("email already exists: %w", err)
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("last insert id: %w", err)
	}

	return User{ID: id, Email: email, PasswordHash: passwordHash, CreatedAt: now, IsVerified: false, Points: 0}, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return User{}, errors.New("email must not be empty")
	}

	row := s.db.QueryRowContext(ctx, `SELECT id, email, password_hash, created_at, is_verified, verified_at, user_points FROM users WHERE email = ?`, email)
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	if id <= 0 {
		return User{}, errors.New("invalid user id")
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, email, password_hash, created_at, is_verified, verified_at, user_points FROM users WHERE id = ?`, id)
	return scanUser(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	var created time.Time
	var verified sql.NullTime
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &created, &user.IsVerified, &verified, &user.Points); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, sql.ErrNoRows
		}
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	user.CreatedAt = created.UTC()
	if verified.Valid {
		t := verified.Time.UTC()
		user.VerifiedAt = &t
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

	_, err := s.db.ExecContext(ctx, `INSERT INTO activation_codes (code, value) VALUES (?, ?)`, strings.ToUpper(code), value)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
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

	var value int64
	if err = tx.QueryRowContext(ctx, `SELECT value FROM activation_codes WHERE code = ?`, normalized).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, 0, sql.ErrNoRows
		}
		return User{}, 0, fmt.Errorf("load activation code: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM activation_codes WHERE code = ?`, normalized); err != nil {
		return User{}, 0, fmt.Errorf("delete activation code: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `UPDATE users SET user_points = user_points + ? WHERE id = ?`, value, userID); err != nil {
		return User{}, 0, fmt.Errorf("add user points: %w", err)
	}

	row := tx.QueryRowContext(ctx, `SELECT id, email, password_hash, created_at, is_verified, verified_at, user_points FROM users WHERE id = ?`, userID)
	user, scanErr := scanUser(row)
	if scanErr != nil {
		return User{}, 0, scanErr
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return User{}, 0, fmt.Errorf("commit redeem activation code: %w", commitErr)
	}

	return user, value, nil
}

func (s *Store) CreateVerificationToken(ctx context.Context, token string, userID int64, expiresAt time.Time) (VerificationToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return VerificationToken{}, errors.New("token must not be empty")
	}
	if userID <= 0 {
		return VerificationToken{}, errors.New("invalid user id")
	}

	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO verification_tokens (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, token, userID, expiresAt.UTC(), now)
	if err != nil {
		return VerificationToken{}, fmt.Errorf("insert verification token: %w", err)
	}

	return VerificationToken{Token: token, UserID: userID, ExpiresAt: expiresAt.UTC(), CreatedAt: now}, nil
}

func (s *Store) GetVerificationToken(ctx context.Context, token string) (VerificationToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return VerificationToken{}, errors.New("token must not be empty")
	}

	var record VerificationToken
	if err := s.db.QueryRowContext(ctx, `SELECT token, user_id, expires_at, created_at FROM verification_tokens WHERE token = ?`, token).
		Scan(&record.Token, &record.UserID, &record.ExpiresAt, &record.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return VerificationToken{}, sql.ErrNoRows
		}
		return VerificationToken{}, fmt.Errorf("get verification token: %w", err)
	}
	record.ExpiresAt = record.ExpiresAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	return record, nil
}

func (s *Store) DeleteVerificationToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token must not be empty")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM verification_tokens WHERE token = ?`, token); err != nil {
		return fmt.Errorf("delete verification token: %w", err)
	}
	return nil
}

func (s *Store) DeleteVerificationTokensForUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM verification_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete verification tokens for user: %w", err)
	}
	return nil
}

func (s *Store) MarkUserVerified(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `UPDATE users SET is_verified = TRUE, verified_at = ? WHERE id = ?`, now, userID)
	if err != nil {
		return fmt.Errorf("mark user verified: %w", err)
	}
	affected, affErr := res.RowsAffected()
	if affErr != nil {
		return fmt.Errorf("mark user verified rows affected: %w", affErr)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
