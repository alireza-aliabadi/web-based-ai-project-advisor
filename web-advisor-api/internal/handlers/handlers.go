package handlers

import (
	"errors"
	"log/slog"

	"github.com/amin/web-based-ai-project-advisor/internal/models"
	"github.com/amin/web-based-ai-project-advisor/internal/services/agent"
	"github.com/amin/web-based-ai-project-advisor/internal/services/auth"
	"github.com/amin/web-based-ai-project-advisor/internal/services/conversation"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type API struct {
	Auth  *auth.Service
	Conv  *conversation.Service
	Agent *agent.Service
	Log   *slog.Logger
}

func (a *API) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func (a *API) Metrics() fiber.Handler {
	return adaptor.HTTPHandler(promhttp.Handler())
}

func (a *API) Register(c *fiber.Ctx) error {
	var req models.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	res, err := a.Auth.Register(c.Context(), req)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			return fiber.NewError(fiber.StatusConflict, err.Error())
		}
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

func (a *API) Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	res, err := a.Auth.Login(c.Context(), req)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return fiber.NewError(fiber.StatusUnauthorized, err.Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(res)
}

func (a *API) Me(c *fiber.Ctx) error {
	userID, err := requireUser(c)
	if err != nil {
		return err
	}
	user, err := a.Auth.GetUser(c.Context(), userID)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user not found")
	}
	return c.JSON(user)
}

func (a *API) UpdatePreferences(c *fiber.Ctx) error {
	userID, err := requireUser(c)
	if err != nil {
		return err
	}
	var body struct {
		SkillLevel         string   `json:"skill_level"`
		PreferredLanguages []string `json:"preferred_languages"`
		HardwareLimit      string   `json:"hardware_limit"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if err := a.Auth.UpdatePreferences(c.Context(), userID, body.SkillLevel, body.PreferredLanguages, body.HardwareLimit); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (a *API) CreateConversation(c *fiber.Ctx) error {
	userID, err := requireUser(c)
	if err != nil {
		return err
	}
	var req models.CreateConversationRequest
	_ = c.BodyParser(&req)
	conv, err := a.Conv.Create(c.Context(), userID, req.Title)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(conv)
}

func (a *API) ListConversations(c *fiber.Ctx) error {
	userID, err := requireUser(c)
	if err != nil {
		return err
	}
	list, err := a.Conv.List(c.Context(), userID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(list)
}

func (a *API) GetConversation(c *fiber.Ctx) error {
	userID, err := requireUser(c)
	if err != nil {
		return err
	}
	convID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid conversation id")
	}
	conv, err := a.Conv.Get(c.Context(), userID, convID)
	if err != nil {
		if errors.Is(err, conversation.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	msgs, err := a.Conv.ListMessages(c.Context(), convID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"conversation": conv, "messages": msgs})
}

func (a *API) SendMessage(c *fiber.Ctx) error {
	userID, err := requireUser(c)
	if err != nil {
		return err
	}
	convID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid conversation id")
	}
	if _, err := a.Conv.Get(c.Context(), userID, convID); err != nil {
		if errors.Is(err, conversation.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var req models.SendMessageRequest
	if err := c.BodyParser(&req); err != nil || req.Content == "" {
		return fiber.NewError(fiber.StatusBadRequest, "content required")
	}

	userMsg, err := a.Conv.AddMessage(c.Context(), convID, "user", req.Content, nil)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	prefs := &models.AnalyzeRequest{Idea: req.Content}
	if user, err := a.Auth.GetUser(c.Context(), userID); err == nil {
		prefs.SkillLevel = user.SkillLevel
		prefs.PreferredLangs = user.PreferredLangs
		prefs.HardwareLimit = user.HardwareLimit
	}

	rec, err := a.Agent.Recommend(c.Context(), req.Content, prefs)
	if err != nil {
		a.Log.Error("recommend failed", "err", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to generate recommendation")
	}

	content := a.Agent.FormatMarkdown(rec)
	meta := map[string]any{"recommendation": rec}
	assistantMsg, err := a.Conv.AddMessage(c.Context(), convID, "assistant", content, meta)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(fiber.Map{
		"user_message":      userMsg,
		"assistant_message": assistantMsg,
		"recommendation":    rec,
	})
}

func (a *API) Analyze(c *fiber.Ctx) error {
	var req models.AnalyzeRequest
	if err := c.BodyParser(&req); err != nil || req.Idea == "" {
		return fiber.NewError(fiber.StatusBadRequest, "idea required")
	}
	if userID, ok := c.Locals("user_id").(uuid.UUID); ok && userID != uuid.Nil {
		if user, err := a.Auth.GetUser(c.Context(), userID); err == nil {
			if req.SkillLevel == "" {
				req.SkillLevel = user.SkillLevel
			}
			if len(req.PreferredLangs) == 0 {
				req.PreferredLangs = user.PreferredLangs
			}
			if req.HardwareLimit == "" {
				req.HardwareLimit = user.HardwareLimit
			}
		}
	}
	rec, err := a.Agent.Recommend(c.Context(), req.Idea, &req)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(rec)
}

func requireUser(c *fiber.Ctx) (uuid.UUID, error) {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return uuid.Nil, fiber.NewError(fiber.StatusUnauthorized, "JWT authentication required")
	}
	return userID, nil
}
