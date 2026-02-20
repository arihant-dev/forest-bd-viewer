package graph

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

import (
	"context"

	"forest-bd-viewer/internal/auth"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type echoContextKeyType string

const EchoContextKey echoContextKeyType = "echoContext"

// GetEchoContext extracts the Echo context stored in the request context.
func GetEchoContext(ctx context.Context) echo.Context {
	ec, _ := ctx.Value(EchoContextKey).(echo.Context)
	return ec
}

type Resolver struct {
	DB      *pgxpool.Pool
	AuthSvc *auth.Service
}
