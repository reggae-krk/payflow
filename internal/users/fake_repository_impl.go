package users

import (
	"context"
	"time"
)

type fakeUserRepository struct {
	users   map[int64]*User
	byEmail map[string]*User
	nextID  int64
}

func NewFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:   make(map[int64]*User),
		byEmail: make(map[string]*User),
		nextID:  1,
	}
}

func (r *fakeUserRepository) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	if _, exists := r.byEmail[email]; exists {
		return nil, ErrEmailTaken
	}

	user := &User{
		Id:           r.nextID,
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}
	r.nextID++

	r.users[user.Id] = user
	r.byEmail[email] = user

	return user, nil
}

func (r *fakeUserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	user, exists := r.users[id]
	if !exists {
		return nil, ErrNoRows
	}
	return user, nil
}

func (r *fakeUserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	user, exists := r.byEmail[email]
	if !exists {
		return nil, ErrNoRows
	}
	return user, nil
}

func (r *fakeUserRepository) UpdatePassword(ctx context.Context, id int64, newPasswordHash string) error {
	user, exists := r.users[id]
	if !exists {
		return ErrNoRows
	}
	user.PasswordHash = newPasswordHash
	return nil
}

func (r *fakeUserRepository) UpdateEmail(ctx context.Context, id int64, newEmail string) error {
	user, exists := r.users[id]
	if !exists {
		return ErrNoRows
	}
	if _, exists := r.byEmail[newEmail]; exists {
		return ErrEmailTaken
	}
	delete(r.byEmail, user.Email)
	user.Email = newEmail
	r.byEmail[newEmail] = user
	return nil
}

func (r *fakeUserRepository) DeleteByID(ctx context.Context, id int64) error {
	user, exists := r.users[id]
	if !exists {
		return ErrNoRows
	}
	delete(r.byEmail, user.Email)
	delete(r.users, id)
	return nil
}
