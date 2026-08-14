package users

import (
    "errors"
    "net/mail"
    "strings"
)

func ValidateEmail(email string) error {
    _, err := mail.ParseAddress(email)
    if err != nil {
        return errors.New("invalid email address")
    }
    return nil
}

func ValidatePassword(pass string) error {
    if len(pass) == 0 || len(strings.TrimSpace(pass)) == 0 {
        return errors.New("password cannot be empty or contain only spaces")
    }
    if len(pass) < 8 {
        return errors.New("password is too short")
    }
    if len(pass) > 32 {
        return errors.New("password is too long")
    }
    if !strings.ContainsAny(pass, "0123456789") {
        return errors.New("password must contain at least 1 digit")
    }
    if !strings.ContainsAny(pass, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
        return errors.New("password must contain at least 1 uppercase letter")
    }
    if !strings.ContainsAny(pass, "!@#$%^&*()-_=+[]{}|;:',.<>?/") {
        return errors.New("password must contain at least 1 special character")
    }
    return nil
}