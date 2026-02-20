# AGENTS.md — Forest BD Viewer

> This file is the authoritative context document for AI coding agents working on this project.
> Read it in full before making any changes. Keep it up to date as the project evolves.

---

## Project Purpose

Full-stack geospatial application for visualizing French forest data (BD Forêt® V2) and Cadastre parcels for Île-de-France (departments 77, 78, 91, 95). Users authenticate, then explore an interactive map with drill-down, species inspection, and polygon analysis.

---

## Current Status

| Stage | Description | Status |
|---|---|---|
| 1 | Skeleton & Infrastructure | Complete |
| 2 | Authentication (register/login/me) | Complete |
| 3 | Forest Data Import & MVT Tiles | Not started |
| 4 | Drill-down, Cadastre, Map State | Not started |
| 5 | Polygon Analysis | Not started |

See `tasks.md` for the full task checklist.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.24+, Echo v4, gqlgen, pgx/v5, golang-migrate |
| Frontend | Next.js 14, React 19, TypeScript 5, Redux Toolkit, Mapbox GL JS |
| Database | PostgreSQL 16 + PostGIS 3.4 |
| Cache | Redis 7 |
| Auth | JWT (HS256) via httpOnly cookie `auth_token` |
| Infrastructure | Docker Compose |

---

## Repository Layout

```
forest_bd_viewer/
├── backend/
│   ├── cmd/server/main.go          # Entry point: Echo setup, DB/Redis init, routes
│   ├── internal/
│   │   ├── auth/auth.go            # JWT generation/validation, bcrypt, Echo middleware
│   │   ├── cache/redis.go          # Redis client init
│   │   ├── config/config.go        # Viper config struct, helper methods
│   │   ├── database/database.go    # pgxpool init, migration runner
│   │   ├── geo/                    # Spatial queries (ST_AsMVT, etc.) — not yet implemented
│   │   ├── graph/                  # gqlgen resolvers/schema — not yet implemented
│   │   └── tiles/                  # MVT tile HTTP handlers — not yet implemented
│   ├── migrations/                 # golang-migrate SQL files: NNNNNN_name.{up,down}.sql
│   ├── Dockerfile                  # Multi-stage: golang:1.24-alpine → alpine:3.19
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── app/
│   │   │   ├── layout.tsx          # Root layout, StoreProvider wrapper
│   │   │   └── page.tsx            # Home (dynamic import of Map, no SSR)
│   │   ├── components/
│   │   │   ├── Map.tsx             # Mapbox GL map, client-side only
│   │   │   └── Map.module.css
│   │   ├── lib/graphql.ts          # graphql-request client (sends credentials)
│   │   └── store/
│   │       ├── index.ts            # Redux store
│   │       └── StoreProvider.tsx   # Client-side Redux Provider
│   ├── Dockerfile                  # Multi-stage: node:20-alpine, output: standalone
│   └── next.config.ts              # output: 'standalone'
├── scripts/
│   ├── download-data.sh            # Fetch BD Forêt SHP from IGN
│   └── import-data.sh              # ogr2ogr → PostGIS import
├── docker-compose.yml
├── .env.example
├── tasks.md
└── agents.md                       # This file
```

---

## Environment Variables

Copy `.env.example` to `.env`. All values are consumed by Docker Compose and forwarded to services.

| Variable | Used by | Notes |
|---|---|---|
| `POSTGRES_USER` | backend, compose | DB user |
| `POSTGRES_PASSWORD` | backend, compose | DB password |
| `POSTGRES_DB` | backend, compose | Database name: `forest_bd` |
| `POSTGRES_HOST` | backend | `postgres` inside Docker, `localhost` locally |
| `POSTGRES_PORT` | backend | 5432 |
| `REDIS_HOST` | backend | `redis` inside Docker, `localhost` locally |
| `REDIS_PORT` | backend | 6379 |
| `BACKEND_PORT` | backend | 8080 |
| `JWT_SECRET` | backend | Must be set; min 32 chars recommended |
| `JWT_EXPIRY_HOURS` | backend | Default: 24 |
| `NEXT_PUBLIC_MAPBOX_TOKEN` | frontend | Required for map to render |
| `NEXT_PUBLIC_API_URL` | frontend | `http://localhost:8080` |

---

## Running the Project

```bash
# Full stack (recommended)
docker-compose up --build

# Frontend: http://localhost:3000
# Backend:  http://localhost:8080/health
```

Migrations run automatically on backend startup via `database.go`.

---

## Backend Conventions

### Package structure

- Each `internal/` package owns its own initialization and exported interface.
- Configuration is passed via `*config.Config`, not global variables.
- Database access uses `*pgxpool.Pool` passed as a dependency.

### Adding a new route

1. Define the handler function (or method on a struct) in the appropriate `internal/` package.
2. Register it in `cmd/server/main.go` on the Echo instance.
3. Apply `auth.Middleware` to protect routes that require authentication.

### Adding a migration

Name files strictly: `NNNNNN_description.up.sql` / `NNNNNN_description.down.sql`.
The next number after the existing `000001` migration is `000002`.
Migrations run automatically at startup; always provide a corresponding `.down.sql`.

### GraphQL (gqlgen)

- Schema files go in `internal/graph/schema/`.
- Run `go generate ./...` from `backend/` after editing `.graphql` schema files to regenerate resolver stubs.
- Resolvers live in `internal/graph/resolver.go` (split into multiple files as needed).

### Authentication flow

- `auth.Middleware` (Echo middleware) extracts JWT from `auth_token` cookie, falls back to `Authorization: Bearer` header.
- Invalid/missing token = guest (does **not** abort the request).
- Downstream handlers check for an authenticated user via `auth.GetUser(ctx)`.
- For protected resolvers/handlers, explicitly return `401` if `auth.GetUser(ctx)` is nil.

### Error handling

- Return structured JSON errors from Echo handlers: `c.JSON(http.StatusXXX, echo.Map{"error": "..."})`.
- GraphQL errors use gqlgen's standard error interface.

---

## Frontend Conventions

### Mapbox components

- The `Map` component must be dynamically imported with `{ ssr: false }` — Mapbox GL JS requires the browser DOM.
- Map initialization fires inside a `useEffect` with an empty dependency array.
- Attach layers/sources after the `map.on('load', ...)` event.

### State management

- Use Redux Toolkit slices in `src/store/`. One slice per domain (e.g., `authSlice`, `mapSlice`).
- Keep server state (GraphQL responses) separate from UI state (Redux).

### GraphQL client

- The client is initialized in `src/lib/graphql.ts` and sends `credentials: 'include'` for cookie-based auth.
- Import and use this shared instance; do not instantiate new clients inline.

### Path aliases

- Use `@/` for imports from `src/`. Configured in `tsconfig.json`.

### Routing

- Use Next.js App Router (`src/app/`). New pages go in `src/app/<route>/page.tsx`.
- Auth-protected pages should redirect to `/login` if no session exists.

---

## Database Schema Reference

### `users` (migration 000001)

| Column | Type | Notes |
|---|---|---|
| `id` | UUID (PK) | `uuid_generate_v4()` default |
| `email` | TEXT UNIQUE NOT NULL | Indexed |
| `password_hash` | TEXT NOT NULL | bcrypt |
| `name` | TEXT NOT NULL | |
| `created_at` | TIMESTAMPTZ | `NOW()` default |
| `updated_at` | TIMESTAMPTZ | `NOW()` default |

### Planned tables (not yet created)

- `forest_parcels` — BD Forêt polygons with species/cover attributes (geometry: MULTIPOLYGON SRID 2154 → reproject to 4326)
- `communes`, `departements`, `regions` — admin boundaries
- `cadastre_parcelles` — Cadastre parcel polygons

---

## Key Interfaces & APIs

### Backend HTTP

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | Public | Returns JSON with DB and Redis status |
| POST | `/graphql` | Mixed | GraphQL endpoint (gqlgen) |
| GET | `/tiles/foret/:z/:x/:y.mvt` | Required | MVT forest tiles (Stage 3) |

### GraphQL (planned schema)

```graphql
type Mutation {
  register(email: String!, password: String!, name: String!): AuthPayload!
  login(email: String!, password: String!): AuthPayload!
}

type Query {
  me: User
  forestStats(polygon: GeoJSONInput!): ForestStats!
}

type AuthPayload {
  user: User!
}
```

---

## Important Notes for Agents

- **Do not modify `docker-compose.yml`** unless changing service topology — environment variable names are referenced by multiple services.
- **Do not regenerate** the entire `go.sum` or run `go mod tidy` unless you have added/removed a dependency. The existing lockfile is correct.
- **PostGIS spatial data** uses SRID 2154 (RGF93 Lambert 93) in source shapefiles. Convert to EPSG:4326 on import, or handle the projection in ST_AsMVT queries.
- **Mapbox tokens** are public-facing (prefixed `NEXT_PUBLIC_`). Never expose `JWT_SECRET` or DB credentials to the frontend.
- **Migrations are irreversible in production** — always write a correct `.down.sql` before committing a `.up.sql`.
- **Redis** is used for MVT tile caching and session data. Default TTL for tiles: use 24h unless data is dynamic.
- **gqlgen** requires code generation after schema changes. Note that in `go.mod` gqlgen is listed under `require` — run `go generate ./internal/graph/...` to regenerate.
- The frontend `Dockerfile` uses `output: 'standalone'` — the production entrypoint is `node server.js`, not `npm start`.
