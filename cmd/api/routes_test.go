package main

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

type fakeUserHandler struct {
    registerCalled bool
}

func (h *fakeUserHandler) Register(c *gin.Context) {
    h.registerCalled = true
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func fakeHealthHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func TestRoutesHealthEndpoint(t *testing.T) {
    fh := &fakeUserHandler{}
    router := SetupRouter(fh, fakeHealthHandler)

    responseRecorder := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/health", nil)
    router.ServeHTTP(responseRecorder, req)

    assert.Equal(t, http.StatusOK, responseRecorder.Code)
    assert.Contains(t, responseRecorder.Body.String(), "ok")
}

func TestRoutesRegisterEndpoint(t *testing.T) {
    fh := &fakeUserHandler{}
    router := SetupRouter(fh, fakeHealthHandler)

    responseRecorder := httptest.NewRecorder()
    body := `{"email": "test@example.com", "password": "StrongP@ss1"}`
    req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    router.ServeHTTP(responseRecorder, req)

    assert.Equal(t, http.StatusOK, responseRecorder.Code)
    assert.True(t, fh.registerCalled)
}