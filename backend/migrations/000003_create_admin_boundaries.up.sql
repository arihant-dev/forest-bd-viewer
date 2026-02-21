-- 000003_create_admin_boundaries.up.sql
-- Creates tables for the administrative hierarchy: regions → departements → communes

CREATE TABLE IF NOT EXISTS regions (
    id          SERIAL PRIMARY KEY,
    code        VARCHAR(3)   NOT NULL,
    nom         TEXT         NOT NULL,
    geom        GEOMETRY(MULTIPOLYGON, 4326),
    created_at  TIMESTAMPTZ  DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_regions_code ON regions(code);
CREATE INDEX IF NOT EXISTS idx_regions_geom ON regions USING GIST(geom);


CREATE TABLE IF NOT EXISTS departements (
    id              SERIAL PRIMARY KEY,
    code            VARCHAR(3)   NOT NULL,
    nom             TEXT         NOT NULL,
    region_code     VARCHAR(3),
    geom            GEOMETRY(MULTIPOLYGON, 4326),
    created_at      TIMESTAMPTZ  DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_departements_code ON departements(code);
CREATE INDEX IF NOT EXISTS idx_departements_region_code ON departements(region_code);
CREATE INDEX IF NOT EXISTS idx_departements_geom ON departements USING GIST(geom);


CREATE TABLE IF NOT EXISTS communes (
    id                  SERIAL PRIMARY KEY,
    code                VARCHAR(5)   NOT NULL,
    nom                 TEXT         NOT NULL,
    departement_code    VARCHAR(3),
    region_code         VARCHAR(3),
    geom                GEOMETRY(MULTIPOLYGON, 4326),
    created_at          TIMESTAMPTZ  DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_communes_code ON communes(code);
CREATE INDEX IF NOT EXISTS idx_communes_departement_code ON communes(departement_code);
CREATE INDEX IF NOT EXISTS idx_communes_region_code ON communes(region_code);
CREATE INDEX IF NOT EXISTS idx_communes_geom ON communes USING GIST(geom);
