package users

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServiceRegisterValidUser(t *testing.T) {
	repo := NewFakeUserRepository()
	service := NewService(repo)

	before := time.Now()
	user, err := service.Register(context.Background(), "test@example.com", "StrongP@ss1")
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}

	assert.Equal(t, user.Email, "test@example.com")
	assert.NotEqual(t, "StrongP@ss1", user.PasswordHash) // Password should be hashed
	assert.Greater(t, user.Id, int64(0))

	margin := 5 * time.Second
	assert.False(t, user.CreatedAt.Before(before.Add(-margin)),
		"CreatedAt too far in the past: %v", user.CreatedAt)
	assert.False(t, user.CreatedAt.After(after.Add(margin)),
		"CreatedAt too far in the future: %v", user.CreatedAt)
}

func TestServiceRegisterNotValidUser(t *testing.T) {
	repo := NewFakeUserRepository()
	service := NewService(repo)

	user, err := service.Register(context.Background(), "test@example.com", "StrongP@ss1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}

	_, err = service.Register(context.Background(), "test@example.com", "StrongP@ss1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	assert.Equal(t, err, ErrEmailTaken)
}

func TestServiceRegisterNotValidEmail(t *testing.T) {
	repo := NewFakeUserRepository()
	service := NewService(repo)

	_, err := service.Register(context.Background(), "invalid-email", "StrongP@ss1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	assert.Equal(t, err.Error(), "invalid email address")
}

func TestServiceRegisterNotValidPassword(t *testing.T) {
	repo := NewFakeUserRepository()
	service := NewService(repo)
	email := "test@example.com"

	_, err := service.Register(context.Background(), email, "weak")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assert.Equal(t, "password is too short", err.Error())

	_, err = service.Register(context.Background(), email, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assert.Equal(t, "password cannot be empty or contain only spaces", err.Error())

	_, err = service.Register(context.Background(), email, "   ")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assert.Equal(t, "password cannot be empty or contain only spaces", err.Error())

	_, err = service.Register(context.Background(), email, "verylongpasswordthatexceedsthirtytwocharacters")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assert.Equal(t, "password is too long", err.Error())

	_, err = service.Register(context.Background(), email, "passwithoutdigit")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assert.Equal(t, "password must contain at least 1 digit", err.Error())

	_, err = service.Register(context.Background(), email, "passwithoutuppercase1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assert.Equal(t, "password must contain at least 1 uppercase letter", err.Error())

	_, err = service.Register(context.Background(), email, "Passwithoutspecial1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assert.Equal(t, "password must contain at least 1 special character", err.Error())
}

func TestServiceLoginValidUser(t *testing.T) {
	repo := NewFakeUserRepository()
	service := NewService(repo)

	_, err := service.Register(context.Background(), "test@example.com", "StrongP@ss1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = service.Login(context.Background(), "test@example.com", "StrongP@ss1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceLoginInvalidUser(t *testing.T) {
	repo := NewFakeUserRepository()
	service := NewService(repo)

	_, err := service.Register(context.Background(), "test@example.com", "StrongP@ss1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = service.Login(context.Background(), "test@example.com", "WrongP@ss1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
