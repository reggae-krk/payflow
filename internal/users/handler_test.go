package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeService struct {
	registerCalled bool
    loginCalled    bool
	registerUser   *User
	registerErr    error
}

func (s *fakeService) Register(ctx context.Context, email, password string) (*User, error) {
	s.registerCalled = true
	return s.registerUser, s.registerErr
}

func (s *fakeService) Login(ctx context.Context, email, password string) (*User, error) {
	s.loginCalled = true
	return s.registerUser, s.registerErr
}

func TestHandlerRegisterValidRequest(t *testing.T) {
    fakeSvc := &fakeService{
        registerUser: &User{
            Id:    1,
            Email: "test@example.com",
        },
        registerErr: nil,
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
}

func TestHandlerRegisterInvalidEmail(t *testing.T) {
    fakeSvc := &fakeService{}

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
    fakeSvc := &fakeService{
        registerUser: nil,
        registerErr:  ErrEmailTaken,
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
    fakeSvc := &fakeService{}
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

func TestHandlerLoginValidRequest(t *testing.T) {
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