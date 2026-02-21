-- 000002_create_forest_parcels.up.sql
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS forest_parcels (
    id SERIAL PRIMARY KEY,
    code_tfv VARCHAR(10),
    lib_tfv VARCHAR(255),
    essence1 VARCHAR(10),
    essence2 VARCHAR(10),
    departement VARCHAR(3) NOT NULL,  -- 77, 78, 91, 95
    commune VARCHAR(5),               -- 5-digit commune code (nullable: forest patches may span multiple communes)
    geom GEOMETRY(MULTIPOLYGON, 4326) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_forest_parcels_dept ON forest_parcels(departement);
CREATE INDEX idx_forest_parcels_commune ON forest_parcels(commune);
CREATE INDEX idx_forest_parcels_geom ON forest_parcels USING GIST(geom);