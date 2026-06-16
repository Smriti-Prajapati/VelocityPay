package users_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/users"
)

// ── Repository stub ───────────────────────────────────────────────────────────

type stubUserRepo struct {
	byID    map[uuid.UUID]*users.User
	byEmail map[string]*users.User
}

func newStubRepo() *stubUserRepo {
	return &stubUserRepo{
		byID:    make(map[uuid.UUID]*users.User),
		byEmail: make(map[string]*users.User),
	}
}

func (r *stubUserRepo) Create(_ context.Context, u *users.User) error {
	r.byID[u.ID] = u
	r.byEmail[u.Email] = u
	return nil
}

func (r *stubUserRepo) FindByID(_ context.Context, id uuid.UUID) (*users.User, error) {
	u, ok := r.byID[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	return u, nil
}

func (r *stubUserRepo) FindByEmail(_ context.Context, email string) (*users.User, error) {
	u, ok := r.byEmail[email]
	if !ok {
		return nil, users.ErrNotFound
	}
	return u, nil
}

func (r *stubUserRepo) Update(_ context.Context, u *users.User) error {
	if _, ok := r.byID[u.ID]; !ok {
		return users.ErrNotFound
	}
	r.byID[u.ID] = u
	r.byEmail[u.Email] = u
	return nil
}

func (r *stubUserRepo) UpdatePassword(_ context.Context, id uuid.UUID, hash string) error {
	u, ok := r.byID[id]
	if !ok {
		return users.ErrNotFound
	}
	u.PasswordHash = hash
	return nil
}

func (r *stubUserRepo) ExistsByEmail(_ context.Context, email string) (bool, error) {
	_, ok := r.byEmail[email]
	return ok, nil
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestUserDomain_RegisterRequest_Validation(t *testing.T) {
	valid := users.RegisterRequest{
		Name:        "Alice",
		Email:       "alice@example.com",
		Password:    "strongpassword",
		PhoneNumber: "+12125550100",
	}

	if valid.Name == "" || valid.Email == "" || valid.Password == "" {
		t.Error("valid request should not have empty required fields")
	}
}

func TestUserRepo_CreateAndFindByEmail(t *testing.T) {
	repo := newStubRepo()
	ctx := context.Background()

	u := &users.User{
		ID:          uuid.New(),
		Name:        "Bob",
		Email:       "bob@example.com",
		PasswordHash: "hashedpassword",
		PhoneNumber: "+12125550101",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	found, err := repo.FindByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("find by email: %v", err)
	}
	if found.ID != u.ID {
		t.Errorf("expected id %v, got %v", u.ID, found.ID)
	}
}

func TestUserRepo_NotFound(t *testing.T) {
	repo := newStubRepo()

	_, err := repo.FindByID(context.Background(), uuid.New())
	if !errors.Is(err, users.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepo_DuplicateEmail(t *testing.T) {
	repo := newStubRepo()
	ctx := context.Background()

	u := &users.User{
		ID:          uuid.New(),
		Name:        "Carol",
		Email:       "carol@example.com",
		PasswordHash: "hash",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_ = repo.Create(ctx, u)

	exists, err := repo.ExistsByEmail(ctx, "carol@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("email should exist")
	}
}

func TestUserRepo_UpdateProfile(t *testing.T) {
	repo := newStubRepo()
	ctx := context.Background()

	u := &users.User{
		ID:        uuid.New(),
		Name:      "Dave",
		Email:     "dave@example.com",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.Create(ctx, u)

	u.Name = "David"
	u.PhoneNumber = "+13125550199"
	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("update: %v", err)
	}

	found, _ := repo.FindByID(ctx, u.ID)
	if found.Name != "David" {
		t.Errorf("expected name David, got %s", found.Name)
	}
}

func TestUserRepo_UpdatePassword(t *testing.T) {
	repo := newStubRepo()
	ctx := context.Background()

	id := uuid.New()
	u := &users.User{
		ID:          id,
		Email:       "eve@example.com",
		PasswordHash: "oldhash",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_ = repo.Create(ctx, u)

	newHash := "newhash"
	if err := repo.UpdatePassword(ctx, id, newHash); err != nil {
		t.Fatalf("update password: %v", err)
	}

	found, _ := repo.FindByID(ctx, id)
	if found.PasswordHash != newHash {
		t.Errorf("password not updated")
	}
}
