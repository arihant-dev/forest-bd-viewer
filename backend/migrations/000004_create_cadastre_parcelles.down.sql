-- 000004_create_cadastre_parcelles.down.sql
DROP INDEX IF EXISTS idx_cadastre_parcelles_geom;
DROP INDEX IF EXISTS idx_cadastre_parcelles_dept_commune;
DROP INDEX IF EXISTS idx_cadastre_parcelles_commune;
DROP INDEX IF EXISTS idx_cadastre_parcelles_departement;
DROP TABLE IF EXISTS cadastre_parcelles;
