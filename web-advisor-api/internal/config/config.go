package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Env            string
	Port           string
	JWTSecret      string
	APIKey         string
	DatabaseURL    string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	QdrantURL      string
	QdrantColl     string
	LLMAPIKey      string
	LLMBaseURL     string
	LLMModel       string
	EmbeddingModel string
	GitHubToken    string
	HFToken        string
	TavilyAPIKey   string
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	cfg := &Config{
		Env:            getEnv("APP_ENV", "development"),
		Port:           getEnv("APP_PORT", "8080"),
		JWTSecret:      getEnv("APP_JWT_SECRET", "dev-secret-change-me"),
		APIKey:         getEnv("APP_API_KEY", "dev-api-key"),
		DatabaseURL:    buildDatabaseURL(),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		QdrantURL:      strings.TrimRight(getEnv("QDRANT_URL", "http://localhost:6333"), "/"),
		QdrantColl:     getEnv("QDRANT_COLLECTION", "knowledge"),
		LLMAPIKey:      getEnv("LLM_API_KEY", ""),
		LLMBaseURL:     strings.TrimRight(getEnv("LLM_BASE_URL", "https://api.openai.com/v1"), "/"),
		LLMModel:       getEnv("LLM_MODEL", "gpt-4o-mini"),
		EmbeddingModel: getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
		GitHubToken:    getEnv("GITHUB_TOKEN", ""),
		HFToken:        getEnv("HUGGINGFACE_TOKEN", ""),
		TavilyAPIKey:   getEnv("TAVILY_API_KEY", ""),
	}
	return cfg, nil
}

func buildDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "architect")
	pass := getEnv("POSTGRES_PASSWORD", "architect")
	db := getEnv("POSTGRES_DB", "architect")
	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + db + "?sslmode=disable"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
