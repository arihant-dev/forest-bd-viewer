package geo

import (
	"context"
	"fmt"
)

// CadastreTile returns a Mapbox Vector Tile (MVT) protobuf for cadastre parcelles
// at the given tile coordinates.
//
// Returns nil bytes (and no error) when the tile contains no features.
func (q *Queries) CadastreTile(ctx context.Context, z, x, y int) ([]byte, error) {
	if z < 0 || z > 22 || x < 0 || y < 0 {
		return nil, fmt.Errorf("invalid tile coordinates: z=%d x=%d y=%d", z, x, y)
	}

	const query = `
		SELECT ST_AsMVT(q, 'cadastre', 4096, 'geom')
		FROM (
			SELECT
				id,
				commune,
				departement,
				section,
				numero,
				ST_AsMVTGeom(
					ST_Transform(geom, 3857),
					ST_TileEnvelope($1, $2, $3),
					4096,
					256,
					true
				) AS geom
			FROM cadastre_parcelles
			WHERE geom && ST_Transform(ST_TileEnvelope($1, $2, $3), 4326)
		) q
		WHERE geom IS NOT NULL
	`

	var tile []byte
	err := q.DB.QueryRow(ctx, query, z, x, y).Scan(&tile)
	if err != nil {
		return nil, fmt.Errorf("cadastre tile query failed (z=%d x=%d y=%d): %w", z, x, y, err)
	}
	return tile, nil
}
