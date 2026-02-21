package tiles

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"forest-bd-viewer/internal/auth"
	"forest-bd-viewer/internal/geo"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

const (
	tileCacheTTL      = 24 * time.Hour
	adminTileCacheTTL = 7 * 24 * time.Hour // admin boundaries rarely change
)

// Handler serves MVT tile endpoints.
type Handler struct {
	geo   *geo.Queries
	redis *redis.Client
}

// NewHandler creates a Handler with the given geo queries and Redis client.
func NewHandler(geoQueries *geo.Queries, redisClient *redis.Client) *Handler {
	return &Handler{geo: geoQueries, redis: redisClient}
}

// parseTileParams extracts and validates z, x, y from Echo path parameters.
// The y parameter may carry a ".mvt" suffix which is stripped automatically.
func parseTileParams(c echo.Context) (z, x, y int, err error) {
	z, err = strconv.Atoi(c.Param("z"))
	if err != nil {
		return
	}
	x, err = strconv.Atoi(c.Param("x"))
	if err != nil {
		return
	}

	yRaw := c.Param("y")
	if len(yRaw) > 4 && yRaw[len(yRaw)-4:] == ".mvt" {
		yRaw = yRaw[:len(yRaw)-4]
	}
	y, err = strconv.Atoi(yRaw)
	if err != nil {
		return
	}

	if z < 0 || z > 22 || x < 0 || y < 0 {
		err = fmt.Errorf("tile coordinates out of range")
	}
	return
}

// serveTile checks the Redis cache, falls back to the provided fetch function,
// caches the result, and writes the MVT response.
func (h *Handler) serveTile(c echo.Context, cacheKey string, ttl time.Duration,
	fetch func(ctx context.Context) ([]byte, error)) error {

	ctx := c.Request().Context()

	// Cache hit
	cached, err := h.redis.Get(ctx, cacheKey).Bytes()
	if err == nil {
		if len(cached) == 0 {
			return c.NoContent(http.StatusNoContent)
		}
		return c.Blob(http.StatusOK, "application/x-protobuf", cached)
	}

	// Cache miss — query PostGIS
	tile, err := fetch(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "tile generation failed"})
	}

	// Cache result (empty tiles stored as empty byte slice to avoid thundering herd)
	storeBytes := tile
	if storeBytes == nil {
		storeBytes = []byte{}
	}
	_ = h.redis.Set(context.Background(), cacheKey, storeBytes, ttl).Err()

	if len(tile) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	return c.Blob(http.StatusOK, "application/x-protobuf", tile)
}

// ForestTile handles GET /tiles/foret/:z/:x/:y.mvt
// Authentication required.
func (h *Handler) ForestTile(c echo.Context) error {
	if auth.GetUser(c.Request().Context()) == nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "authentication required"})
	}

	z, x, y, err := parseTileParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid tile coordinates"})
	}

	cacheKey := fmt.Sprintf("tile:foret:%d:%d:%d", z, x, y)
	return h.serveTile(c, cacheKey, tileCacheTTL, func(ctx context.Context) ([]byte, error) {
		return h.geo.ForestTile(ctx, z, x, y)
	})
}

// AdminTile handles GET /tiles/admin/:layer/:z/:x/:y.mvt
// No authentication required — admin boundaries are public data.
// layer must be one of: regions, departements, communes.
func (h *Handler) AdminTile(c echo.Context) error {
	layer := c.Param("layer")

	z, x, y, err := parseTileParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid tile coordinates"})
	}

	cacheKey := fmt.Sprintf("tile:admin:%s:%d:%d:%d", layer, z, x, y)
	return h.serveTile(c, cacheKey, adminTileCacheTTL, func(ctx context.Context) ([]byte, error) {
		return h.geo.AdminTile(ctx, layer, z, x, y)
	})
}

// CadastreTile handles GET /tiles/cadastre/:z/:x/:y.mvt
// Authentication required.
func (h *Handler) CadastreTile(c echo.Context) error {
	if auth.GetUser(c.Request().Context()) == nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "authentication required"})
	}

	z, x, y, err := parseTileParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid tile coordinates"})
	}

	cacheKey := fmt.Sprintf("tile:cadastre:%d:%d:%d", z, x, y)
	return h.serveTile(c, cacheKey, tileCacheTTL, func(ctx context.Context) ([]byte, error) {
		return h.geo.CadastreTile(ctx, z, x, y)
	})
}
