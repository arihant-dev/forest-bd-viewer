-- 000003_create_admin_boundaries.down.sql
DROP INDEX IF EXISTS idx_communes_geom;
DROP INDEX IF EXISTS idx_communes_region_code;
DROP INDEX IF EXISTS idx_communes_departement_code;
DROP INDEX IF EXISTS idx_communes_code;
DROP TABLE IF EXISTS communes;

DROP INDEX IF EXISTS idx_departements_geom;
DROP INDEX IF EXISTS idx_departements_region_code;
DROP INDEX IF EXISTS idx_departements_code;
DROP TABLE IF EXISTS departements;

DROP INDEX IF EXISTS idx_regions_geom;
DROP INDEX IF EXISTS idx_regions_code;
DROP TABLE IF EXISTS regions;
