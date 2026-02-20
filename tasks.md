
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
- [ ] Verify: full auth cycle in browser

## Stage 3 — Data Import & Forest Layer

- [ ] `scripts/download-data.sh` — BD Forêt V2 for depts 77, 78, 91, 95
- [ ] `scripts/import-data.sh` — ogr2ogr import, GIST indexes
- [ ] Backend: `/tiles/foret/:z/:x/:y.mvt` endpoint using ST_AsMVT
- [ ] Frontend: forest layer with color coding, click popup, legend
- [ ] Verify: colored forest polygons visible on map

## Stage 4 — Drill-down, Cadastre, Map State

- [ ] Import admin boundaries (régions, départements, communes)
- [ ] Backend: GraphQL queries for boundaries + map state CRUD
- [ ] Frontend: hierarchical click drill-down
- [ ] Frontend: cadastre parcelle layer at zoom ≥ 15
- [ ] Frontend + Backend: map state save/restore
- [ ] Verify: drill-down works, cadastre visible, state persists

## Stage 5 — Polygon Analysis (Bonus A)

- [ ] Frontend: polygon drawing with mapbox-gl-draw
- [ ] Backend: `analyzePolygon` mutation (area, species breakdown)
- [ ] Frontend: results panel
- [ ] Verify: draw polygon → see stats

## Final

- [x] README with setup, API docs, assumptions
- [ ] Polish UI, dark theme, animations
