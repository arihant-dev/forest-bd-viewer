package geo

import (
	"context"
	"fmt"
)

// validAdminLayers is the set of permitted layer names for AdminTile.
// These map directly to table names in PostGIS.
var validAdminLayers = map[string]bool{
	"regions":      true,
	"departements": true,
	"communes":     true,
}

// layerFields returns the feature properties to include in the MVT for each layer.
func layerFields(layer string) string {
	switch layer {
	case "regions":
		return "id, code, nom"
	case "departements":
		return "id, code, nom, region_code"
	case "communes":
		return "id, code, nom, departement_code, region_code"
	default:
		return "id"
	}
}

// AdminTile returns a Mapbox Vector Tile (MVT) protobuf for the requested
// administrative boundary layer at the given tile coordinates.
//
// layer must be one of: "regions", "departements", "communes".
// Returns nil bytes (and no error) when the tile contains no features.
func (q *Queries) AdminTile(ctx context.Context, layer string, z, x, y int) ([]byte, error) {
	if !validAdminLayers[layer] {
		return nil, fmt.Errorf("invalid admin layer %q: must be regions, departements, or communes", layer)
	}
	if z < 0 || z > 22 || x < 0 || y < 0 {
		return nil, fmt.Errorf("invalid tile coordinates: z=%d x=%d y=%d", z, x, y)
	}

	fields := layerFields(layer)

	// Build the query dynamically — layer is validated against the allowlist above,
	// so this string interpolation is safe.
	query := fmt.Sprintf(`
		SELECT ST_AsMVT(q, '%s', 4096, 'geom')
		FROM (
			SELECT
				%s,
				ST_AsMVTGeom(
					ST_Transform(geom, 3857),
					ST_TileEnvelope($1, $2, $3),
					4096,
					256,
					true
				) AS geom
			FROM %s
			WHERE geom && ST_Transform(ST_TileEnvelope($1, $2, $3), 4326)
		) q
		WHERE geom IS NOT NULL
	`, layer, fields, layer)

	var tile []byte
	err := q.DB.QueryRow(ctx, query, z, x, y).Scan(&tile)
	if err != nil {
		return nil, fmt.Errorf("admin tile query failed (layer=%s z=%d x=%d y=%d): %w", layer, z, x, y, err)
	}
	return tile, nil
}
