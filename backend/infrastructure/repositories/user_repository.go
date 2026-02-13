package repositories

import (
	"database/sql"
	"time"

	"ai-pdf-assistant-backend/database"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the database
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserRepository handles user database operations
type UserRepository struct{}

// NewUserRepository creates a new user repository
func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

// Create creates a new user with hashed password
func (r *UserRepository) Create(email, password, name string) (*User, error) {
	if !database.IsConnected() {
		return nil, sql.ErrNoRows
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hashedPassword),
		Name:         name,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = database.DB.Exec(`
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, user.ID, user.Email, user.PasswordHash, user.Name, user.CreatedAt, user.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetByEmail finds a user by email address
func (r *UserRepository) GetByEmail(email string) (*User, error) {
	if !database.IsConnected() {
		return nil, sql.ErrNoRows
	}

	user := &User{}
	err := database.DB.QueryRow(`
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetByID finds a user by ID
func (r *UserRepository) GetByID(id string) (*User, error) {
	if !database.IsConnected() {
		return nil, sql.ErrNoRows
	}

	user := &User{}
	err := database.DB.QueryRow(`
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return user, nil
}

// VerifyPassword checks if the provided password matches the user's hashed password
func (r *UserRepository) VerifyPassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// EmailExists checks if an email is already registered
func (r *UserRepository) EmailExists(email string) (bool, error) {
	if !database.IsConnected() {
		return false, nil
	}

	var count int
	err := database.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
