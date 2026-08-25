package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUserHandler struct {
	registerCalled bool
}

type fakeAccountHandler struct {
	getBalanceCalled bool
	depositCalled    bool
	withdrawCalled   bool
	transferCalled   bool
	historyCalled    bool
}

type fakeRegistrationHandler struct {
	createdCalled bool
}

func (h *fakeUserHandler) Register(c *gin.Context) {
	h.registerCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeUserHandler) Login(c *gin.Context) {
	h.registerCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeAccountHandler) GetBalance(c *gin.Context) {
	h.getBalanceCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeAccountHandler) Deposit(c *gin.Context) {
	h.depositCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeAccountHandler) Withdraw(c *gin.Context) {
	h.withdrawCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeAccountHandler) Transfer(c *gin.Context) {
	h.transferCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeRegistrationHandler) Register(c *gin.Context) {
	h.createdCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func fakeHealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeAccountHandler) GetHistory(c *gin.Context) {
	h.historyCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func TestRoutesHealthEndpoint(t *testing.T) {
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.Contains(t, responseRecorder.Body.String(), "ok")
}

func TestRoutesRegisterEndpoint(t *testing.T) {
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	body := `{"email": "test@example.com", "password": "StrongP@ss1"}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fr.createdCalled)
}

func TestRoutesLoginEndpoint(t *testing.T) {
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	body := `{"email": "test@example.com", "password": "StrongP@ss1"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fu.registerCalled)
}

func validAuthHeader(t *testing.T) string {
	t.Setenv("JWT_SECRET", "test-pass")
	token, err := auth.GenerateToken(1)
	require.NoError(t, err)
	return fmt.Sprintf("Bearer %s", token)
}

func TestRoutesBalanceEndpoint(t *testing.T) {
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts/1/balance", nil)
	req.Header.Set("Authorization", validAuthHeader(t))
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fa.getBalanceCalled)
}

func TestRoutesDepositEndpoint(t *testing.T) {
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	body := `{"amount_minor": 500}`
	req := httptest.NewRequest(http.MethodPost, "/accounts/1/deposit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", validAuthHeader(t))
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fa.depositCalled)
}

func TestRoutesWithdrawEndpoint(t *testing.T) {
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	body := `{"amount_minor": 500}`
	req := httptest.NewRequest(http.MethodPost, "/accounts/1/withdraw", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", validAuthHeader(t))
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fa.withdrawCalled)
}

func TestRoutesTransferEndpoint(t *testing.T) {
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	body := `{"amount_minor": 500, "destination_account_id": 2}`
	req := httptest.NewRequest(http.MethodPost, "/accounts/1/transfer", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "route-test-key")
	req.Header.Set("Authorization", validAuthHeader(t))
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fa.transferCalled)
}

func TestRoutesHistoryEndpoint(t *testing.T) {
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts/1/history", nil)
	req.Header.Set("Authorization", validAuthHeader(t))
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fa.historyCalled)
}

func TestRoutesWalletEndpointsRequireAuth(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"balance", http.MethodGet, "/accounts/1/balance"},
		{"deposit", http.MethodPost, "/accounts/1/deposit"},
		{"withdraw", http.MethodPost, "/accounts/1/withdraw"},
		{"transfer", http.MethodPost, "/accounts/1/transfer"},
		{"history", http.MethodGet, "/accounts/1/history"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fu := &fakeUserHandler{}
			fa := &fakeAccountHandler{}
			fr := &fakeRegistrationHandler{}
			router := SetupRouter(fu, fa, fr, fakeHealthHandler)

			responseRecorder := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(responseRecorder, req)

			assert.Equal(t, http.StatusUnauthorized, responseRecorder.Code)
			assert.False(t, fa.getBalanceCalled)
			assert.False(t, fa.depositCalled)
			assert.False(t, fa.withdrawCalled)
			assert.False(t, fa.transferCalled)
			assert.False(t, fa.historyCalled)
		})
	}
}
