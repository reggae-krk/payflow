package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/users"
	"github.com/reggae-krk/payflow/internal/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRegistrationService struct {
	registerCalled  bool
	registerUser    *users.User
	registerAccount *wallet.Account
	err             error
}

func (s *fakeRegistrationService) RegisterWithDefaultAccount(ctx context.Context, email string, password string) (*users.User, *wallet.Account, error) {
	s.registerCalled = true
	return s.registerUser, s.registerAccount, s.err
}

func TestHandlerRegisterValidRequest(t *testing.T) {
	fakeSvc := &fakeRegistrationService{
		registerUser: &users.User{
			Id:    1,
			Email: "test@example.com",
		},
		registerAccount: &wallet.Account{
			Id:           1,
			UserId:       1,
			Currency:     wallet.PLN,
			BalanceMinor: 0,
		},
	}

	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	body := `{"email": "test@example.com", "password": "StrongP@ss1"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	assert.Equal(t, http.StatusCreated, responseRecorder.Code)
	assert.True(t, fakeSvc.registerCalled)

	var resp map[string]any
	err := json.Unmarshal(responseRecorder.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "test@example.com", resp["email"])
	assert.Equal(t, "User was successfully created", resp["message"])
	assert.Equal(t, 5, len(resp))
	assert.Equal(t, "PLN", resp["currency"])
	assert.Equal(t, float64(1), resp["user_id"])
	assert.Equal(t, float64(1), resp["account_id"])
}

func TestHandlerRegisterInvalidEmail(t *testing.T) {
	fakeSvc := &fakeRegistrationService{}

	h := NewHandler(fakeSvc)

	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)

	body := `{"email": "invalid", "password": "StrongP@ss1"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assert.False(t, fakeSvc.registerCalled)

	var resp map[string]any
	err := json.Unmarshal(responseRecorder.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp["error"], "Email")
}

func TestHandlerRegisterEmailTaken(t *testing.T) {
    fakeSvc := &fakeRegistrationService{
        registerUser: nil,
        err:  users.ErrEmailTaken,
    }

    h := NewHandler(fakeSvc)

    responseRecorder := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(responseRecorder)

    body := `{"email": "taken@example.com", "password": "StrongP@ss1"}`
    c.Request = httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
    c.Request.Header.Set("Content-Type", "application/json")

    h.Register(c)

    assert.Equal(t, http.StatusConflict, responseRecorder.Code)
    assert.True(t, fakeSvc.registerCalled)

    var resp map[string]any
    err := json.Unmarshal(responseRecorder.Body.Bytes(), &resp)
    require.NoError(t, err)

    assert.Equal(t, "email already registered", resp["error"])
}

func TestHandlerRegisterInvalidJSON(t *testing.T) {
    fakeSvc := &fakeRegistrationService{}
    h := NewHandler(fakeSvc)

    responseRecorder := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(responseRecorder)

    body := `{invalid json}`
    c.Request = httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
    c.Request.Header.Set("Content-Type", "application/json")

    h.Register(c)

    assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
    assert.False(t, fakeSvc.registerCalled)
}
