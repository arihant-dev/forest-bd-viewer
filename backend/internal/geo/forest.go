package geo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Queries holds the database pool for spatial queries.
type Queries struct {
	DB *pgxpool.Pool
}

// ForestTile returns a Mapbox Vector Tile (MVT) protobuf for the given tile coordinates.
// Returns nil bytes (and no error) when the tile contains no forest features.
func (q *Queries) ForestTile(ctx context.Context, z, x, y int) ([]byte, error) {
	if z < 0 || z > 22 || x < 0 || y < 0 {
		return nil, fmt.Errorf("invalid tile coordinates: z=%d x=%d y=%d", z, x, y)
	}

	// ST_TileEnvelope returns the tile bounding box in EPSG:3857 (Web Mercator).
	// We transform the stored 4326 geometries to 3857 for ST_AsMVTGeom,
	// and use the inverse transform for the spatial filter (&&).
	const query = `
		SELECT ST_AsMVT(q, 'forest', 4096, 'geom')
		FROM (
			SELECT
				id,
				code_tfv,
				lib_tfv,
				essence1,
				departement,
				ST_AsMVTGeom(
					ST_Transform(geom, 3857),
					ST_TileEnvelope($1, $2, $3),
					4096,
					256,
					true
				) AS geom
			FROM forest_parcels
			WHERE geom && ST_Transform(ST_TileEnvelope($1, $2, $3), 4326)
		) q
		WHERE geom IS NOT NULL
	`

	var tile []byte
	err := q.DB.QueryRow(ctx, query, z, x, y).Scan(&tile)
	if err != nil {
		return nil, fmt.Errorf("forest tile query failed (z=%d x=%d y=%d): %w", z, x, y, err)
	}
	return tile, nil
}
