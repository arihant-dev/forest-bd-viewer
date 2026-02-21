-- 000004_create_cadastre_parcelles.up.sql
-- Unified cadastre parcel table covering all target departments.
-- Populated by scripts/import-data.sh from etalab GeoJSON files.

CREATE TABLE IF NOT EXISTS cadastre_parcelles (
    id              SERIAL PRIMARY KEY,
    commune         VARCHAR(5),
    departement     VARCHAR(3),
    section         VARCHAR(10),
    numero          VARCHAR(10),
    geom            GEOMETRY(MULTIPOLYGON, 4326) NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cadastre_parcelles_departement
    ON cadastre_parcelles(departement);

CREATE INDEX IF NOT EXISTS idx_cadastre_parcelles_commune
    ON cadastre_parcelles(commune);

-- Composite index for the common dept + commune filter
CREATE INDEX IF NOT EXISTS idx_cadastre_parcelles_dept_commune
    ON cadastre_parcelles(departement, commune);

CREATE INDEX IF NOT EXISTS idx_cadastre_parcelles_geom
    ON cadastre_parcelles USING GIST(geom);
