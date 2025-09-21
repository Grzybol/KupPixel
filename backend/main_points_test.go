package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"

	"github.com/example/kup-piksel/internal/storage/sqlite"
)

type redeemResponse struct {
	User struct {
		Points int64 `json:"points"`
	} `json:"user"`
	AddedPoints int64 `json:"added_points"`
}

type updatePixelResponse struct {
	User struct {
		Points int64 `json:"points"`
	} `json:"user"`
	Pixel struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	} `json:"pixel"`
}

func setupTestServer(t *testing.T) (*Server, *sqlite.Store) {
	t.Helper()

	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	store.SetSkipPixelSeed(true)

	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if err := store.InsertPixel(context.Background(), sqlite.Pixel{ID: 1, Status: "free"}); err != nil {
		t.Fatalf("insert pixel: %v", err)
	}

	server := &Server{
		store:                store,
		sessions:             NewSessionManager(),
		mailer:               &fakeMailer{},
		verificationBaseURL:  "http://example.com",
		verificationTokenTTL: time.Hour,
		pixelCostPoints:      10,
	}
	return server, store
}

func TestHandleRedeemActivationCode_Success(t *testing.T) {
	server, store := setupTestServer(t)

	user, err := store.CreateUser(context.Background(), "user@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := store.CreateActivationCode(context.Background(), "ABCD-EFGH-IJKL-MNOP", 25); err != nil {
		t.Fatalf("create activation code: %v", err)
	}

	sessionID, err := server.sessions.Create(user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	body := bytes.NewBufferString(`{"code":"ABCD-EFGH-IJKL-MNOP"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/activation-codes/redeem", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	c := &gin.Context{Writer: w, Request: req}

	server.handleRedeemActivationCode(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp redeemResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.AddedPoints != 25 {
		t.Fatalf("expected added points 25, got %d", resp.AddedPoints)
	}
	if resp.User.Points != 25 {
		t.Fatalf("expected user points 25, got %d", resp.User.Points)
	}

	refreshed, err := store.GetUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if refreshed.Points != 25 {
		t.Fatalf("expected stored user points 25, got %d", refreshed.Points)
	}
}

func TestHandleUpdatePixelRequiresPoints(t *testing.T) {
	server, store := setupTestServer(t)

	user, err := store.CreateUser(context.Background(), "user@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	sessionID, err := server.sessions.Create(user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Attempt to purchase without points.
	purchaseBody := bytes.NewBufferString(`{"id":1,"status":"taken","color":"#ffffff","url":"https://example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/pixels", purchaseBody)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	c := &gin.Context{Writer: w, Request: req}

	server.handleUpdatePixel(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for insufficient points, got %d", w.Code)
	}
	t.Log("insufficient points handled")

	// Redeem activation code to gain points.
	if err := store.CreateActivationCode(context.Background(), "WXYZ-1234-5678-90AB", 40); err != nil {
		t.Fatalf("create activation code: %v", err)
	}

	redeemBody := bytes.NewBufferString(`{"code":"WXYZ-1234-5678-90AB"}`)
	redeemReq := httptest.NewRequest(http.MethodPost, "/api/activation-codes/redeem", redeemBody)
	redeemReq.Header.Set("Content-Type", "application/json")
	redeemReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	redeemW := httptest.NewRecorder()
	redeemCtx := &gin.Context{Writer: redeemW, Request: redeemReq}
	server.handleRedeemActivationCode(redeemCtx)
	if redeemW.Code != http.StatusOK {
		t.Fatalf("expected status 200 when redeeming code, got %d", redeemW.Code)
	}
	t.Log("activation code redeemed")

	// Try to purchase again with points.
	purchaseBody = bytes.NewBufferString(`{"id":1,"status":"taken","color":"#0000ff","url":"https://example.com"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/pixels", purchaseBody)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w = httptest.NewRecorder()
	c = &gin.Context{Writer: w, Request: req}

	server.handleUpdatePixel(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	t.Log("pixel purchase succeeded")

	var resp updatePixelResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Pixel.ID != 1 {
		t.Fatalf("expected pixel id 1, got %d", resp.Pixel.ID)
	}
	if resp.Pixel.Status != "taken" {
		t.Fatalf("expected pixel status taken, got %s", resp.Pixel.Status)
	}
	if resp.User.Points != 30 {
		t.Fatalf("expected remaining points 30, got %d", resp.User.Points)
	}
}
