package users

import (
	"context"
	// "encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
)

type fakeService struct {
    loginCalled    bool
	registerUser   *User
	registerErr    error
}

func (s *fakeService) Register(ctx context.Context, email, password string) (*User, error) {
	return s.registerUser, s.registerErr
}

func (s *fakeService) Login(ctx context.Context, email, password string) (*User, error) {
	s.loginCalled = true
	return s.registerUser, s.registerErr
}

func TestHandlerLoginValidRequest(t *testing.T) {
    t.Setenv("JWT_SECRET", "test-pass")
    fakeSvc := &fakeService{
        registerUser: &User{
            Id:    1,
            Email: "test@example.com",
        },
    }
    h := NewHandler(fakeSvc)

    responseRecorder := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(responseRecorder)

    body := `{"email": "test@example.com", "password": "StrongP@ss1"}`
    c.Request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
    c.Request.Header.Set("Content-Type", "application/json")

    h.Login(c)

    assert.Equal(t, http.StatusOK, responseRecorder.Code)
    assert.Contains(t, responseRecorder.Body.String(), "token\":\"ey")
    assert.True(t, fakeSvc.loginCalled)
}

func TestHandlerLoginInvalidJSON(t *testing.T) {
    fakeSvc := &fakeService{}
    h := NewHandler(fakeSvc)

    responseRecorder := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(responseRecorder)

    body := `{invalid json}`
    c.Request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
    c.Request.Header.Set("Content-Type", "application/json")

    h.Login(c)

    assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
    assert.False(t, fakeSvc.loginCalled)
}

func TestHandlerLoginUnauthorized(t *testing.T) {
    fakeSvc := &fakeService{
        registerUser: nil,
        registerErr:  ErrInvalidCredentials,
    }
    h := NewHandler(fakeSvc)

    responseRecorder := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(responseRecorder)

    body := `{"email": "unauthorized@example.com", "password": "WrongP@ss1"}`
    c.Request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
    c.Request.Header.Set("Content-Type", "application/json")

    h.Login(c)

    assert.Equal(t, http.StatusUnauthorized, responseRecorder.Code)
    assert.True(t, fakeSvc.loginCalled)
}