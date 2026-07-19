package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amin/web-based-ai-project-advisor/internal/cache"
	"github.com/amin/web-based-ai-project-advisor/internal/config"
	"github.com/amin/web-based-ai-project-advisor/internal/database"
	"github.com/amin/web-based-ai-project-advisor/internal/handlers"
	"github.com/amin/web-based-ai-project-advisor/internal/middleware"
	"github.com/amin/web-based-ai-project-advisor/internal/services/agent"
	"github.com/amin/web-based-ai-project-advisor/internal/services/auth"
	"github.com/amin/web-based-ai-project-advisor/internal/services/conversation"
	"github.com/amin/web-based-ai-project-advisor/internal/services/embedding"
	githubsvc "github.com/amin/web-based-ai-project-advisor/internal/services/githubsvc"
	"github.com/amin/web-based-ai-project-advisor/internal/services/huggingface"
	"github.com/amin/web-based-ai-project-advisor/internal/services/llm"
	"github.com/amin/web-based-ai-project-advisor/internal/services/ranking"
	"github.com/amin/web-based-ai-project-advisor/internal/services/requirements"
	"github.com/amin/web-based-ai-project-advisor/internal/services/search"
	"github.com/gofiber/fiber/v2"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
	fiberlimiter "github.com/gofiber/fiber/v2/middleware/limiter"
)

func main() {
	migrateOnly := flag.Bool("migrate-only", false, "run migrations and exit")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		log.Error("migration failed", "err", err)
		os.Exit(1)
	}
	if *migrateOnly {
		log.Info("migrations complete")
		return
	}

	redisCache := cache.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err := redisCache.Ping(ctx); err != nil {
		log.Warn("redis unavailable; continuing without cache guarantees", "err", err)
	}

	llmClient := llm.New(cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.LLMModel, cfg.EmbeddingModel)
	embedSvc := embedding.New(llmClient, cfg.QdrantURL, cfg.QdrantColl)
	gh := githubsvc.New(cfg.GitHubToken, redisCache)
	hf := huggingface.New(cfg.HFToken, redisCache)
	web := search.New(cfg.TavilyAPIKey, redisCache)
	agentSvc := agent.New(
		requirements.New(llmClient),
		gh, hf, web, embedSvc, ranking.New(), llmClient,
	)

	api := &handlers.API{
		Auth:  auth.New(pool, cfg.JWTSecret),
		Conv:  conversation.New(pool),
		Agent: agentSvc,
		Log:   log,
	}

	app := fiber.New(fiber.Config{
		AppName:      "AI Solution Architect",
		ErrorHandler: errorHandler(log),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	})
	app.Use(fiberrecover.New())
	app.Use(middleware.CORS())
	app.Use(middleware.RequestLogger())
	app.Use(fiberlogger.New())
	app.Use(fiberlimiter.New(fiberlimiter.Config{
		Max:        120,
		Expiration: time.Minute,
	}))

	app.Get("/health", api.Health)
	app.Get("/metrics", api.Metrics())

	v1 := app.Group("/api/v1")
	v1.Post("/auth/register", api.Register)
	v1.Post("/auth/login", api.Login)

	jwt := middleware.AuthRequired(cfg.JWTSecret, cfg.APIKey)
	v1.Get("/me", jwt, api.Me)
	v1.Patch("/me/preferences", jwt, api.UpdatePreferences)
	v1.Get("/conversations", jwt, api.ListConversations)
	v1.Post("/conversations", jwt, api.CreateConversation)
	v1.Get("/conversations/:id", jwt, api.GetConversation)
	v1.Post("/conversations/:id/messages", jwt, api.SendMessage)

	v1.Post("/analyze", middleware.OptionalAuth(cfg.JWTSecret, cfg.APIKey), api.Analyze)

	go func() {
		<-ctx.Done()
		_ = app.Shutdown()
	}()

	log.Info("server starting", "port", cfg.Port, "env", cfg.Env, "llm", llmClient.Available())
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func errorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		msg := "internal server error"
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
			msg = e.Message
		} else {
			log.Error("unhandled error", "err", err, "path", c.Path())
		}
		return c.Status(code).JSON(fiber.Map{"error": msg})
	}
}
