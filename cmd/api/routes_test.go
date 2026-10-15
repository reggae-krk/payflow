package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/auth"
	"github.com/reggae-krk/payflow/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUserHandler struct {
	registerCalled bool
	loginCtx       context.Context
}

type fakeAccountHandler struct {
	getBalanceCtx    context.Context
	getBalanceCalled bool
	depositCalled    bool
	withdrawCalled   bool
	transferCalled   bool
	historyCalled    bool
}

type fakeRegistrationHandler struct {
	createdCalled bool
	registerCtx   context.Context
}

func (h *fakeUserHandler) Register(c *gin.Context) {
	h.registerCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeUserHandler) Login(c *gin.Context) {
	h.registerCalled = true
	h.loginCtx = c.Request.Context()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeAccountHandler) GetBalance(c *gin.Context) {
	h.getBalanceCalled = true
	h.getBalanceCtx = c.Request.Context()
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
	h.registerCtx = c.Request.Context()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func fakeHealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *fakeAccountHandler) GetHistory(c *gin.Context) {
	h.historyCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func getTimeoutCfg() config.TimeoutConfig {
	return config.TimeoutConfig{
		Health:  2 * time.Second,
		LogReg:  2 * time.Second,
		Request: 2 * time.Second,
	}
}

func TestRoutesHealthEndpoint(t *testing.T) {
	ft := getTimeoutCfg()
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(ft, fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.Contains(t, responseRecorder.Body.String(), "ok")
}

func TestRoutesRegisterEndpoint(t *testing.T) {
	ft := getTimeoutCfg()
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(ft, fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	body := `{"email": "test@example.com", "password": "StrongP@ss1"}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fr.createdCalled)
}

func TestRoutesLoginEndpoint(t *testing.T) {
	ft := getTimeoutCfg()
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(ft, fu, fa, fr, fakeHealthHandler)

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
	ft := getTimeoutCfg()
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(ft, fu, fa, fr, fakeHealthHandler)

	responseRecorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts/1/balance", nil)
	req.Header.Set("Authorization", validAuthHeader(t))
	router.ServeHTTP(responseRecorder, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.True(t, fa.getBalanceCalled)
}

func TestRoutesDepositEndpoint(t *testing.T) {
	ft := getTimeoutCfg()
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(ft, fu, fa, fr, fakeHealthHandler)

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
	ft := getTimeoutCfg()
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(ft, fu, fa, fr, fakeHealthHandler)

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
	ft := getTimeoutCfg()
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(ft, fu, fa, fr, fakeHealthHandler)

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
	ft := getTimeoutCfg()
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	router := SetupRouter(ft, fu, fa, fr, fakeHealthHandler)

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
			ft := getTimeoutCfg()
			fu := &fakeUserHandler{}
			fa := &fakeAccountHandler{}
			fr := &fakeRegistrationHandler{}
			router := SetupRouter(ft, fu, fa, fr, fakeHealthHandler)

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

func TestTimeoutMiddlewareSetsDeadlineAndCancelsAfterRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var handlerCtx context.Context
	router := gin.New()
	router.GET("/", timeoutMiddleware(300*time.Millisecond), func(c *gin.Context) {
		handlerCtx = c.Request.Context()
		c.Status(http.StatusOK)
	})

	start := time.Now()
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	require.NotNil(t, handlerCtx)
	deadline, ok := handlerCtx.Deadline()
	require.True(t, ok, "handler context has no deadline")
	assert.WithinDuration(t, start.Add(300*time.Millisecond), deadline, 100*time.Millisecond)
	// cancel runs when the middleware returns, so DB work started by the request can't outlive it.
	assert.ErrorIs(t, handlerCtx.Err(), context.Canceled)
}

func TestRoutesUseTimeoutPerGroup(t *testing.T) {
	timeouts := config.TimeoutConfig{Health: 1 * time.Second, LogReg: 2 * time.Second, Request: 3 * time.Second}
	fu := &fakeUserHandler{}
	fa := &fakeAccountHandler{}
	fr := &fakeRegistrationHandler{}
	var healthCtx context.Context
	health := func(c *gin.Context) {
		healthCtx = c.Request.Context()
		c.Status(http.StatusOK)
	}
	router := SetupRouter(timeouts, fu, fa, fr, health)

	serve := func(method, path string, authorized bool) {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		if authorized {
			req.Header.Set("Authorization", validAuthHeader(t))
		}
		router.ServeHTTP(httptest.NewRecorder(), req)
	}
	serve(http.MethodGet, "/health", false)
	serve(http.MethodPost, "/login", false)
	serve(http.MethodPost, "/register", false)
	serve(http.MethodGet, "/accounts/1/balance", true)

	cases := []struct {
		name string
		ctx  context.Context
		want time.Duration
	}{
		{"health", healthCtx, timeouts.Health},
		{"login", fu.loginCtx, timeouts.LogReg},
		{"register", fr.registerCtx, timeouts.LogReg},
		{"balance", fa.getBalanceCtx, timeouts.Request},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.ctx, "handler was not called")
			deadline, ok := tc.ctx.Deadline()
			require.True(t, ok, "no deadline")
			// Checked right after the requests ran, so the time left is close to the configured timeout.
			assert.InDelta(t, tc.want, time.Until(deadline), float64(500*time.Millisecond))
		})
	}
}
