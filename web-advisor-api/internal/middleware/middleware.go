package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const bearerPrefix = "Bearer "

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

func GenerateToken(secret string, userID uuid.UUID, email string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func AuthRequired(jwtSecret, apiKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if key := c.Get("X-API-Key"); key != "" && apiKey != "" && key == apiKey {
			c.Locals("auth_type", "api_key")
			c.Locals("user_id", uuid.Nil)
			return c.Next()
		}

		auth := c.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, bearerPrefix) {
			return fiber.NewError(fiber.StatusUnauthorized, "missing or invalid authorization")
		}
		tokenStr := strings.TrimPrefix(auth, bearerPrefix)
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "unexpected signing method")
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid token")
		}
		c.Locals("auth_type", "jwt")
		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		return c.Next()
	}
}

func OptionalAuth(jwtSecret, apiKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if key := c.Get("X-API-Key"); key != "" && apiKey != "" && key == apiKey {
			c.Locals("auth_type", "api_key")
			c.Locals("user_id", uuid.Nil)
			return c.Next()
		}
		auth := c.Get("Authorization")
		if strings.HasPrefix(auth, bearerPrefix) {
			tokenStr := strings.TrimPrefix(auth, bearerPrefix)
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				return []byte(jwtSecret), nil
			})
			if err == nil && token.Valid {
				c.Locals("auth_type", "jwt")
				c.Locals("user_id", claims.UserID)
				c.Locals("email", claims.Email)
			}
		}
		return c.Next()
	}
}

func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		c.Set("X-Response-Time", time.Since(start).String())
		return err
	}
}

func CORS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization,X-API-Key")
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}
