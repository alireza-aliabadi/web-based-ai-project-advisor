package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/amin/web-based-ai-project-advisor/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("conversation not found")

type Service struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, title string) (*models.Conversation, error) {
	if strings.TrimSpace(title) == "" {
		title = "New conversation"
	}
	var c models.Conversation
	err := s.db.QueryRow(ctx, `
INSERT INTO conversations (user_id, title) VALUES ($1, $2)
RETURNING id, user_id, title, created_at, updated_at
`, userID, title).Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]models.Conversation, error) {
	rows, err := s.db.Query(ctx, `
SELECT id, user_id, title, created_at, updated_at
FROM conversations WHERE user_id = $1
ORDER BY updated_at DESC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Conversation
	for rows.Next() {
		var c models.Conversation
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []models.Conversation{}
	}
	return out, rows.Err()
}

func (s *Service) Get(ctx context.Context, userID, convID uuid.UUID) (*models.Conversation, error) {
	var c models.Conversation
	err := s.db.QueryRow(ctx, `
SELECT id, user_id, title, created_at, updated_at
FROM conversations WHERE id = $1 AND user_id = $2
`, convID, userID).Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) ListMessages(ctx context.Context, convID uuid.UUID) ([]models.Message, error) {
	rows, err := s.db.Query(ctx, `
SELECT id, conversation_id, role, content, metadata, created_at
FROM messages WHERE conversation_id = $1
ORDER BY created_at ASC
`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Message
	for rows.Next() {
		var m models.Message
		var meta []byte
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &meta, &m.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(meta, &m.Metadata)
		out = append(out, m)
	}
	if out == nil {
		out = []models.Message{}
	}
	return out, rows.Err()
}

func (s *Service) AddMessage(ctx context.Context, convID uuid.UUID, role, content string, metadata map[string]any) (*models.Message, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	var m models.Message
	var meta []byte
	err = s.db.QueryRow(ctx, `
INSERT INTO messages (conversation_id, role, content, metadata)
VALUES ($1, $2, $3, $4)
RETURNING id, conversation_id, role, content, metadata, created_at
`, convID, role, content, metaBytes).Scan(
		&m.ID, &m.ConversationID, &m.Role, &m.Content, &meta, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(meta, &m.Metadata)
	_, _ = s.db.Exec(ctx, `UPDATE conversations SET updated_at = $1 WHERE id = $2`, time.Now().UTC(), convID)

	// Auto-title from first user message
	if role == "user" {
		_, _ = s.db.Exec(ctx, `
UPDATE conversations SET title = $1
WHERE id = $2 AND title = 'New conversation'
`, truncate(content, 60), convID)
	}
	return &m, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
