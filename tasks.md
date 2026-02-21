
# Forest BD Viewer — Task Checklist

## Stage 1 — Skeleton & Infrastructure

- [x] Docker Compose (postgres + postgis, backend, frontend)
- [x] Go backend: module init, chi router, healthcheck, pgxpool DB connection
- [x] Next.js frontend: scaffolding + Mapbox GL JS blank map at Paris
- [x] [.env.example](file:///Users/arihant/Documents/challenges/forest_bd_viewer/.env.example) with all config vars
- [x] Verify: `docker-compose up` → map renders, `/health` returns 200

## Stage 2 — Authentication

- [x] DB migration: `users` table
- [x] Backend: gqlgen schema + resolvers for register/login/me
- [x] Backend: JWT middleware, bcrypt passwords
- [x] Frontend: login + register pages, auth context, protected routes
- [x] Verify: full auth cycle in browser

## Stage 3 — Data Import & Forest Layer

- [x] `scripts/download-data.sh` — admin boundaries (geo.api.gouv.fr) + cadastre parcelles per commune (etalab) + BD Forêt manual download instructions
- [x] `scripts/import-data.sh` — ogr2ogr import into unified tables (regions, departements, communes, cadastre_parcelles, forest_parcels) with GIST indexes
- [x] Backend: `/tiles/foret/:z/:x/:y.mvt` endpoint using ST_AsMVT
- [x] Frontend: forest layer with color coding, click popup, legend
- [x] Verify: colored forest polygons visible on map

## Stage 4 — Drill-down, Cadastre, Map State

- [x] DB migrations 000003 + 000004: regions, departements, communes, cadastre_parcelles tables with indexes
- [x] Backend: `/tiles/admin/:layer/:z/:x/:y.mvt` — MVT tiles for regions/departements/communes (public, 7-day cache)
- [x] Backend: `/tiles/cadastre/:z/:x/:y.mvt` — MVT tiles for cadastre parcelles (auth required, 24h cache)
- [x] Frontend: zoom-based layer switching (regions 5–7 → depts 8–10 → communes 11–13 → BD Forêt 14+ → cadastre 15+)
- [x] Frontend: hierarchical click drill-down (click region/dept/commune → fitBounds to zoom in)
- [x] Frontend: cadastre parcelle layer at zoom ≥ 15 with popup (section, numéro, commune)
- [x] Frontend: dynamic legend updates with current zoom tier
- [x] Backend: GraphQL queries for admin boundary metadata (list regions/depts/communes)
- [x] Frontend + Backend: map state save/restore
- [x] Verify: drill-down works end-to-end with real data, cadastre visible, state persists

## Stage 5 — Polygon Analysis (Bonus A)

- [x] Frontend: polygon drawing with mapbox-gl-draw
- [x] Backend: `analyzePolygon` mutation (area, species breakdown)
- [x] Frontend: results panel
- [x] Verify: draw polygon → see stats

## Final

- [x] README with setup, API docs, assumptions
- [ ] Polish UI, dark theme, animations
