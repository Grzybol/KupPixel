package storage

import (
	"context"
	"errors"
	"time"
)

const (
	GridWidth   = 1000
	GridHeight  = 1000
	TotalPixels = GridWidth * GridHeight
)

type Pixel struct {
	ID        int       `json:"id"`
	Status    string    `json:"status"`
	Color     string    `json:"color,omitempty"`
	URL       string    `json:"url,omitempty"`
	OwnerID   *int64    `json:"owner_id,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type User struct {
	ID           int64      `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	IsVerified   bool       `json:"is_verified"`
	VerifiedAt   *time.Time `json:"verified_at,omitempty"`
	Points       int64      `json:"points"`
}

type VerificationToken struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type PasswordResetToken struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type PixelState struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Pixels []Pixel `json:"pixels"`
}

var (
	ErrPixelOwnedByAnotherUser = errors.New("pixel owned by another user")
	ErrInsufficientPoints      = errors.New("insufficient points")
)

type Store interface {
	Close() error
	EnsureSchema(ctx context.Context) error
	SetSkipPixelSeed(skip bool)
	InsertPixel(ctx context.Context, pixel Pixel) error
	GetAllPixels(ctx context.Context) (PixelState, error)
	UpdatePixel(ctx context.Context, pixel Pixel) (Pixel, error)
	UpdatePixelForUserWithCost(ctx context.Context, userID int64, pixel Pixel, cost int64) (Pixel, User, error)
	UpdatePixelForUser(ctx context.Context, userID int64, pixel Pixel) (Pixel, error)
	GetPixelsByOwner(ctx context.Context, ownerID int64) ([]Pixel, error)
	CreateUser(ctx context.Context, email, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, id int64) (User, error)
	CreateActivationCode(ctx context.Context, code string, value int64) error
	RedeemActivationCode(ctx context.Context, userID int64, code string) (User, int64, error)
	CreateVerificationToken(ctx context.Context, token string, userID int64, expiresAt time.Time) (VerificationToken, error)
	GetVerificationToken(ctx context.Context, token string) (VerificationToken, error)
	DeleteVerificationToken(ctx context.Context, token string) error
	DeleteVerificationTokensForUser(ctx context.Context, userID int64) error
	MarkUserVerified(ctx context.Context, userID int64) error
	CreatePasswordResetToken(ctx context.Context, token string, userID int64, expiresAt time.Time) (PasswordResetToken, error)
	GetPasswordResetToken(ctx context.Context, token string) (PasswordResetToken, error)
	DeletePasswordResetToken(ctx context.Context, token string) error
	DeletePasswordResetTokensForUser(ctx context.Context, userID int64) error
	UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error
}
