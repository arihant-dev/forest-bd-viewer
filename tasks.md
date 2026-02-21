
# Forest BD Viewer — Task Checklist

## Stage 1 — Skeleton & Infrastructure

- [x] Docker Compose (postgres + postgis, backend, frontend)
- [x] Go backend: module init, chi router, healthcheck, pgxpool DB connection
- [x] Next.js frontend: scaffolding + Mapbox GL JS blank map at Paris
- [x] .env.example with all config vars
- [x] Verify: `docker-compose up` -> map renders, `/health` returns 200

## Stage 2 — Authentication

- [x] DB migration: `users` table
- [x] Backend: gqlgen schema + resolvers for register/login/me
- [x] Backend: JWT middleware, bcrypt passwords
- [x] Frontend: login + register pages, auth context, protected routes
- [x] Verify: full auth cycle in browser

## Stage 3 — Data Import & Forest Layer

- [x] `scripts/download-data.sh` — admin boundaries (geo.api.gouv.fr) + cadastre parcelles per commune (etalab) + BD Foret manual download instructions
- [x] `scripts/import-data.sh` + `import_data.py` — ogr2ogr import into unified tables (regions, departements, communes, cadastre_parcelles, forest_parcels) with GIST indexes
- [x] Backend: `/tiles/foret/:z/:x/:y.mvt` endpoint using ST_AsMVT
- [x] Frontend: forest layer with color coding, click popup, legend
- [x] Verify: colored forest polygons visible on map

## Stage 4 — Drill-down, Cadastre, Map State

- [x] DB migrations 000003 + 000004: regions, departements, communes, cadastre_parcelles tables with indexes
- [x] Backend: `/tiles/admin/:layer/:z/:x/:y.mvt` — MVT tiles for regions/departements/communes (public, 7-day cache)
- [x] Backend: `/tiles/cadastre/:z/:x/:y.mvt` — MVT tiles for cadastre parcelles (auth required, 24h cache)
- [x] Frontend: zoom-based layer switching (regions 5-7, depts 8-10, communes 11-13, BD Foret 14+, cadastre 15+)
- [x] Frontend: hierarchical click drill-down (click region/dept/commune -> fitBounds to zoom in)
- [x] Frontend: cadastre parcelle layer at zoom >= 15 with popup (section, numero, commune)
- [x] Frontend: dynamic legend updates with current zoom tier
- [x] Backend: GraphQL queries for admin boundary metadata (list regions/depts/communes)
- [x] Frontend + Backend: map state save/restore
- [x] Verify: drill-down works end-to-end with real data, cadastre visible, state persists

## Stage 5 — Polygon Analysis

- [x] Frontend: polygon drawing with mapbox-gl-draw (custom Finish/Cancel buttons)
- [x] Backend: `analyzePolygon` mutation (area, TFV breakdown, species breakdown)
- [x] Backend: TFV code normalization (hierarchical BD Foret V2 + legacy TFIFN -> 9 canonical categories)
- [x] Frontend: analysis results panel with TFV and species breakdown bars
- [x] Frontend: immovable polygon after creation (converted to static GeoJSON layer)
- [x] Frontend: click suppression during drawing mode (prevents popup interference)
- [x] Verify: draw polygon -> see stats with translated labels

## Stage 6 — Internationalization & Polish

- [x] i18n system: React Context-based EN/FR dictionary with 60+ keys
- [x] English as default language
- [x] All Map.tsx strings translated (legend, popups, buttons)
- [x] AnalysisPanel TFV labels translated via i18n dictionary
- [x] Language toggle (EN/FR)
- [x] README with full setup guide, data pipeline, API reference, troubleshooting
- [x] agents.md updated with i18n, TFV normalization, polygon UX docs
- [ ] Polish UI, dark theme, animations

## Stage 7 — Bonus B: LiDAR CHM Analysis

- [x] Research IGN LIDAR HD data sources (WFS tile index, MNS/MNT tile formats, coverage gaps)
- [x] Create `feature/lidar-chm` branch
- [x] GraphQL schema: `analyzeLidar` mutation + `LidarAnalysis` type (`lidar.graphql`)
- [x] Pure Go GeoTIFF float32 reader (`internal/geo/geotiff.go`)
- [x] LiDAR processing pipeline: WFS query, tile download/cache, CHM computation, PNG generation (`internal/geo/lidar.go`)
- [x] GraphQL resolver (`lidar.resolvers.go`) + CHM image HTTP endpoint (`/lidar/chm/:id`)
- [x] gqlgen code generation
- [x] Frontend Redux: lidar state in `analysisSlice.ts` + `analyzeLidarThunk`
- [x] Frontend i18n: 10 LiDAR translation keys (EN/FR)
- [x] Frontend `AnalysisPanel.tsx`: LiDAR CHM section (stats, loading, error, no-coverage)
- [x] Frontend CSS: LiDAR stats styles in `AnalysisPanel.module.css`
- [x] Frontend `Map.tsx`: dispatch `analyzeLidarThunk` + Mapbox ImageSource CHM overlay
- [x] Backend + frontend build verification
- [x] Documentation updated (agents.md, tasks.md, README.md)
