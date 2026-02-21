package geo

import (
	"context"
	"encoding/json"
	"fmt"
)

// PolygonStats holds the aggregate results of a spatial polygon analysis
// against the forest_parcels table.
type PolygonStats struct {
	AreaHa           float64
	ForestCoverHa    float64
	ParcelCount      int64
	TFVBreakdown     []TFVRow
	SpeciesBreakdown []SpeciesRow
}

// TFVRow is one row of the type-of-forest-vegetation breakdown.
type TFVRow struct {
	CodeTFV string
	LibTFV  string
	AreaHa  float64
}

// SpeciesRow is one row of the dominant-species breakdown.
type SpeciesRow struct {
	Essence string
	AreaHa  float64
}

// AnalyzePolygon runs three PostGIS queries against forest_parcels using the
// supplied GeoJSON geometry string (Polygon or MultiPolygon, EPSG:4326) and
// returns aggregate statistics.
//
// All area calculations use ST_Transform to EPSG:2154 (RGF93 Lambert 93),
// the French official metric projection, so areas are in square metres and
// converted to hectares (÷ 10000).
func (q *Queries) AnalyzePolygon(ctx context.Context, geojsonGeom string) (*PolygonStats, error) {
	// Validate that geojson is parseable JSON before sending to PostGIS.
	if !json.Valid([]byte(geojsonGeom)) {
		return nil, fmt.Errorf("invalid GeoJSON: not valid JSON")
	}
	// Basic type check — must be Polygon or MultiPolygon.
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(geojsonGeom), &peek); err != nil {
		return nil, fmt.Errorf("invalid GeoJSON: %w", err)
	}
	if peek.Type != "Polygon" && peek.Type != "MultiPolygon" {
		return nil, fmt.Errorf("invalid GeoJSON: type must be Polygon or MultiPolygon, got %q", peek.Type)
	}

	// ── 1. Polygon area + forest cover summary ────────────────────────────────
	const summarySQL = `
		WITH poly AS (
			SELECT ST_GeomFromGeoJSON($1) AS geom
		)
		SELECT
			ST_Area(ST_Transform(poly.geom, 2154)) / 10000.0                          AS polygon_area_ha,
			COALESCE(SUM(
				ST_Area(ST_Transform(ST_Intersection(fp.geom, poly.geom), 2154)) / 10000.0
			), 0)                                                                      AS forest_cover_ha,
			COUNT(fp.id)                                                               AS parcel_count
		FROM poly
		LEFT JOIN forest_parcels fp
			ON fp.geom && poly.geom
			AND ST_Intersects(fp.geom, poly.geom)
		GROUP BY poly.geom
	`

	var stats PolygonStats
	if err := q.DB.QueryRow(ctx, summarySQL, geojsonGeom).Scan(
		&stats.AreaHa,
		&stats.ForestCoverHa,
		&stats.ParcelCount,
	); err != nil {
		return nil, fmt.Errorf("polygon summary query failed: %w", err)
	}

	// No forest in this polygon — return early with empty breakdowns.
	if stats.ParcelCount == 0 {
		return &stats, nil
	}

	// ── 2. TFV breakdown ──────────────────────────────────────────────────────
	// Normalize granular BD Forêt V2 codes (e.g. FF1-09-09, FF1G01-01) and
	// legacy TFIFN codes (e.g. AFJ, CPV, 30) to the 9 top-level TFV categories
	// so the frontend can translate and display them consistently.
	const tfvSQL = `
		WITH poly AS (SELECT ST_GeomFromGeoJSON($1) AS geom),
		intersected AS (
			SELECT
				CASE
					WHEN fp.code_tfv LIKE 'FF1%' OR fp.code_tfv = 'FF0'
						OR fp.code_tfv IN ('AFJ','AFV','HFW','HFZ','QF') THEN 'FF1'
					WHEN fp.code_tfv LIKE 'FF2%'
						OR fp.code_tfv IN ('CPJ','CPV','CRJ','CRV')      THEN 'FF2'
					WHEN fp.code_tfv LIKE 'FF3%'
						OR fp.code_tfv IN ('FR','MR')                    THEN 'FF3'
					WHEN fp.code_tfv LIKE 'FO1%'
						OR fp.code_tfv = '30'                            THEN 'FO1'
					WHEN fp.code_tfv LIKE 'FO2%'                         THEN 'FO2'
					WHEN fp.code_tfv LIKE 'FO3%'                         THEN 'FO3'
					WHEN fp.code_tfv LIKE 'LA%'
						OR fp.code_tfv = '40'                            THEN 'LA'
					WHEN fp.code_tfv = 'FP'
						OR fp.code_tfv = '50'                            THEN 'FP'
					ELSE 'FF4'
				END AS norm_code,
				ST_Area(ST_Transform(ST_Intersection(fp.geom, poly.geom), 2154)) / 10000.0 AS area_ha
			FROM poly
			JOIN forest_parcels fp
				ON fp.geom && poly.geom
				AND ST_Intersects(fp.geom, poly.geom)
		)
		SELECT
			norm_code AS code_tfv,
			CASE norm_code
				WHEN 'FF1' THEN 'Forêt fermée feuillus'
				WHEN 'FF2' THEN 'Forêt fermée conifères'
				WHEN 'FF3' THEN 'Forêt fermée mixte'
				WHEN 'FF4' THEN 'Forêt fermée autre'
				WHEN 'FO1' THEN 'Forêt ouverte feuillus'
				WHEN 'FO2' THEN 'Forêt ouverte conifères'
				WHEN 'FO3' THEN 'Forêt ouverte mixte'
				WHEN 'LA'  THEN 'Lande'
				WHEN 'FP'  THEN 'Peupleraie'
				ELSE norm_code
			END AS lib_tfv,
			SUM(area_ha) AS area_ha
		FROM intersected
		GROUP BY norm_code
		ORDER BY area_ha DESC
	`

	tfvRows, err := q.DB.Query(ctx, tfvSQL, geojsonGeom)
	if err != nil {
		return nil, fmt.Errorf("TFV breakdown query failed: %w", err)
	}
	defer tfvRows.Close()

	for tfvRows.Next() {
		var row TFVRow
		if err := tfvRows.Scan(&row.CodeTFV, &row.LibTFV, &row.AreaHa); err != nil {
			return nil, fmt.Errorf("scanning TFV row: %w", err)
		}
		stats.TFVBreakdown = append(stats.TFVBreakdown, row)
	}
	if err := tfvRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating TFV rows: %w", err)
	}

	// ── 3. Species breakdown ──────────────────────────────────────────────────
	const speciesSQL = `
		WITH poly AS (SELECT ST_GeomFromGeoJSON($1) AS geom)
		SELECT
			COALESCE(NULLIF(TRIM(fp.essence1), ''), '—') AS essence,
			SUM(
				ST_Area(ST_Transform(ST_Intersection(fp.geom, poly.geom), 2154)) / 10000.0
			) AS area_ha
		FROM poly
		JOIN forest_parcels fp
			ON fp.geom && poly.geom
			AND ST_Intersects(fp.geom, poly.geom)
		GROUP BY fp.essence1
		ORDER BY area_ha DESC
	`

	specRows, err := q.DB.Query(ctx, speciesSQL, geojsonGeom)
	if err != nil {
		return nil, fmt.Errorf("species breakdown query failed: %w", err)
	}
	defer specRows.Close()

	for specRows.Next() {
		var row SpeciesRow
		if err := specRows.Scan(&row.Essence, &row.AreaHa); err != nil {
			return nil, fmt.Errorf("scanning species row: %w", err)
		}
		stats.SpeciesBreakdown = append(stats.SpeciesBreakdown, row)
	}
	if err := specRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating species rows: %w", err)
	}

	return &stats, nil
}
