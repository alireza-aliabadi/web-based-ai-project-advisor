package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/amin/web-based-ai-project-advisor/internal/middleware"
	"github.com/amin/web-based-ai-project-advisor/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken       = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type Service struct {
	db        *pgxpool.Pool
	jwtSecret string
}

func New(db *pgxpool.Pool, jwtSecret string) *Service {
	return &Service{db: db, jwtSecret: jwtSecret}
}

func (s *Service) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" || len(req.Password) < 6 {
		return nil, errors.New("email and password (min 6 chars) required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	var user models.User
	err = s.db.QueryRow(ctx, `
INSERT INTO users (email, password_hash, display_name)
VALUES ($1, $2, $3)
RETURNING id, email, display_name, skill_level, preferred_languages, hardware_limit, created_at
`, email, string(hash), name).Scan(
		&user.ID, &user.Email, &user.DisplayName, &user.SkillLevel,
		&user.PreferredLangs, &user.HardwareLimit, &user.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	token, err := middleware.GenerateToken(s.jwtSecret, user.ID, user.Email, 72*time.Hour)
	if err != nil {
		return nil, err
	}
	return &models.AuthResponse{Token: token, User: user}, nil
}

func (s *Service) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	var user models.User
	err := s.db.QueryRow(ctx, `
SELECT id, email, password_hash, display_name, skill_level, preferred_languages, hardware_limit, created_at
FROM users WHERE email = $1
`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.SkillLevel,
		&user.PreferredLangs, &user.HardwareLimit, &user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	token, err := middleware.GenerateToken(s.jwtSecret, user.ID, user.Email, 72*time.Hour)
	if err != nil {
		return nil, err
	}
	return &models.AuthResponse{Token: token, User: user}, nil
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := s.db.QueryRow(ctx, `
SELECT id, email, display_name, skill_level, preferred_languages, hardware_limit, created_at
FROM users WHERE id = $1
`, id).Scan(
		&user.ID, &user.Email, &user.DisplayName, &user.SkillLevel,
		&user.PreferredLangs, &user.HardwareLimit, &user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Service) UpdatePreferences(ctx context.Context, id uuid.UUID, skill string, langs []string, hardware string) error {
	_, err := s.db.Exec(ctx, `
UPDATE users SET skill_level = COALESCE(NULLIF($2, ''), skill_level),
                   preferred_languages = COALESCE($3, preferred_languages),
                   hardware_limit = COALESCE(NULLIF($4, ''), hardware_limit)
WHERE id = $1
`, id, skill, langs, hardware)
	return err
}
