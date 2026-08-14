package users

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
    Register(ctx context.Context, email, password string) (*User, error)
}

type service struct {
	repo UserRepository
}

func NewService(repo UserRepository) *service {
	return &service{repo: repo}
}

func (s *service) Register(ctx context.Context, email, password string) (*User, error) {
	if err := ValidateEmail(email); err != nil {
		return nil, err
	}
	
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, ErrNoRows) {
			return nil, err
		}
	} else if existing != nil {
		return nil, ErrEmailTaken
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err != nil {
		return nil, err
	}

	user, err := s.repo.CreateUser(ctx, email, string(hashedPass))

	if err != nil {
		return nil, err
	}
	return user, nil
}
