package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	SkillLevel   string    `json:"skill_level"`
	PreferredLangs []string `json:"preferred_languages"`
	HardwareLimit  string   `json:"hardware_limit"`
	CreatedAt    time.Time `json:"created_at"`
}

type Conversation struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Role           string    `json:"role"` // user | assistant | system
	Content        string    `json:"content"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type Requirements struct {
	Domain       string   `json:"domain"`
	Task         string   `json:"task"`
	Modality     string   `json:"modality"`
	Requirements []string `json:"requirements"`
	Query        string   `json:"query"`
	SkillLevel   string   `json:"skill_level,omitempty"`
	Languages    []string `json:"languages,omitempty"`
	Hardware     string   `json:"hardware,omitempty"`
}

type Repository struct {
	Name        string   `json:"name"`
	FullName    string   `json:"full_name"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Stars       int      `json:"stars"`
	Forks       int      `json:"forks"`
	Language    string   `json:"language"`
	Topics      []string `json:"topics"`
	UpdatedAt   string   `json:"last_update"`
	README      string   `json:"readme,omitempty"`
	Score       float64  `json:"score,omitempty"`
	Why         string   `json:"why,omitempty"`
}

type Model struct {
	Name         string  `json:"name"`
	Task         string  `json:"task"`
	Downloads    int     `json:"downloads"`
	Likes        int     `json:"likes"`
	Architecture string  `json:"architecture"`
	License      string  `json:"license"`
	Framework    string  `json:"framework"`
	URL          string  `json:"url"`
	Score        float64 `json:"score,omitempty"`
	Hardware     string  `json:"hardware,omitempty"`
	Why          string  `json:"why,omitempty"`
}

type SearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score,omitempty"`
}

type Recommendation struct {
	Requirements   Requirements  `json:"requirements"`
	Repositories   []Repository  `json:"repositories"`
	Models         []Model       `json:"models"`
	SearchResults  []SearchResult `json:"search_results,omitempty"`
	Architecture   string        `json:"architecture"`
	MermaidDiagram string        `json:"mermaid_diagram,omitempty"`
	TechStack      []string      `json:"tech_stack"`
	Roadmap        []string      `json:"roadmap"`
	Deployment     string        `json:"deployment"`
	Hardware       string        `json:"hardware"`
	CostEstimate   string        `json:"cost_estimate,omitempty"`
	Summary        string        `json:"summary"`
}

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type CreateConversationRequest struct {
	Title string `json:"title"`
}

type SendMessageRequest struct {
	Content string `json:"content"`
	Stream  bool   `json:"stream,omitempty"`
}

type AnalyzeRequest struct {
	Idea           string   `json:"idea"`
	SkillLevel     string   `json:"skill_level,omitempty"`
	PreferredLangs []string `json:"preferred_languages,omitempty"`
	HardwareLimit  string   `json:"hardware_limit,omitempty"`
}
