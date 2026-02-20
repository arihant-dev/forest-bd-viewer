package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type contextKey string

const UserContextKey contextKey = "user"

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type Service struct {
	secretKey   []byte
	expiryHours int
}

func NewService(secret string, expiryHours int) *Service {
	return &Service{
		secretKey:   []byte(secret),
		expiryHours: expiryHours,
	}
}

func (s *Service) ExpirySeconds() int {
	return s.expiryHours * 3600
}

func (s *Service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *Service) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *Service) GenerateToken(userID, email string) (string, error) {
	expirationTime := time.Now().Add(time.Duration(s.expiryHours) * time.Hour)
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "forest-bd-viewer",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// Middleware extracts JWT from cookie or header and sets user context
func (s *Service) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenString := ""

			// Try cookie first
			cookie, err := c.Cookie("auth_token")
			if err == nil {
				tokenString = cookie.Value
			}

			// Fallback to Header
			if tokenString == "" {
				authHeader := c.Request().Header.Get("Authorization")
				if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
					tokenString = after
				}
			}

			if tokenString == "" {
				// No token, continue without user context (public access)
				return next(c)
			}

			claims, err := s.ValidateToken(tokenString)
			if err != nil {
				// Invalid token but present -> maybe clear cookie?
				// For now, just continue as guest
				return next(c)
			}

			// Set user in Echo context
			c.Set("user", claims)

			// Also set in Request context for GraphQL resolver
			ctx := context.WithValue(c.Request().Context(), UserContextKey, claims)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// GetUser from context
func GetUser(ctx context.Context) *Claims {
	user, _ := ctx.Value(UserContextKey).(*Claims)
	return user
}
