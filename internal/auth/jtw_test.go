package auth

import "testing"

func TestGenerateAndVerifyToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-pass")
	userId := int64(12345)
	token, err := GenerateToken(userId)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	userIdReturned, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("Failed to verify token: %v", err)
	}
	if userIdReturned != userId {
		t.Fatalf("Expected userId %d, got %d", userId, userIdReturned)
	}
}

func TestVerifyInvalidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-pass")
	invalidToken := "invalid.token.string"
	_, err := VerifyToken(invalidToken)
	if err == nil {
		t.Fatalf("Expected error for invalid token")
	}
}
