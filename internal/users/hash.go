package users

import "golang.org/x/crypto/bcrypt"

func HashPass(plainPass string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(plainPass), bcrypt.DefaultCost)
}

func VerifyPassword(hashedPass, plainPass string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(plainPass))
	return err == nil
}
