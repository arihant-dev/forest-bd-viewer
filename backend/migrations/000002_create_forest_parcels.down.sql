-- 000002_create_forest_parcels.down.sql
DROP INDEX IF EXISTS forest_parcels_tfv_idx;
DROP INDEX IF EXISTS forest_parcels_geom_idx;
DROP TABLE IF EXISTS forest_parcels;
