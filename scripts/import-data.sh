#!/usr/bin/env bash
# scripts/import-data.sh
# Imports all downloaded data into PostGIS:
#   1. Admin boundaries → regions, departements, communes tables
#   2. Cadastre parcelles → cadastre_parcelles (unified table)
#   3. BD Forêt V2 (IGN SHP) → forest_parcels table

set -euo pipefail

DATA_DIR="/app/data/raw"

DB_HOST="${POSTGRES_HOST:-postgres}"
DB_PORT="${POSTGRES_PORT:-5432}"
DB_NAME="${POSTGRES_DB:-forest_bd}"
DB_USER="${POSTGRES_USER:-forestviewer}"
export PGPASSWORD="${POSTGRES_PASSWORD:-forestviewer_secret}"

PG_DSN="PG:host=${DB_HOST} port=${DB_PORT} dbname=${DB_NAME} user=${DB_USER}"

DEPTS=("77" "78" "91" "95")

echo "==================================================================="
echo " Forest BD Viewer — PostGIS Data Importer"
echo "==================================================================="
echo "Data dir: ${DATA_DIR}"
echo "DB: ${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo ""

# ─── Prerequisite check ───────────────────────────────────────────────────────
if ! command -v ogr2ogr &>/dev/null; then
    echo "ERROR: ogr2ogr not found."
    exit 1
fi

if ! command -v psql &>/dev/null; then
    echo "ERROR: psql not found."
    exit 1
fi

# ─── Helper: run a psql command ───────────────────────────────────────────────
run_sql() {
    psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -c "$1"
}

# ─── Verify database connection ───────────────────────────────────────────────
echo "Testing database connection..."
if ! run_sql "SELECT 1;" > /dev/null 2>&1; then
    echo "ERROR: Cannot connect to PostgreSQL at ${DB_HOST}:${DB_PORT}/${DB_NAME}"
    exit 1
fi
echo "✓ Database connected"
echo ""

# ─── Verify tables exist ──────────────────────────────────────────────────────
echo "Checking required tables..."
for table in regions departements communes cadastre_parcelles forest_parcels; do
    if ! run_sql "SELECT 1 FROM information_schema.tables WHERE table_name='${table}';" | grep -q 1; then
        echo "ERROR: Table '${table}' does not exist. Run migrations first."
        exit 1
    fi
done
echo "✓ All required tables exist"
echo ""

# ─── 1. Admin Boundaries ─────────────────────────────────────────────────────
echo "[1/3] Importing admin boundaries"
echo ""

# regions
REGIONS_FILE="${DATA_DIR}/admin/regions.geojson"
if [[ -f "${REGIONS_FILE}" ]]; then
    echo "  → regions table..."
    run_sql "TRUNCATE TABLE regions;" 2>/dev/null || true
    if ogr2ogr \
        -f PostgreSQL "${PG_DSN}" \
        "${REGIONS_FILE}" \
        -nln regions \
        -nlt PROMOTE_TO_MULTI \
        -lco GEOMETRY_NAME=geom \
        -lco FID=id \
        -lco OVERWRITE=YES; then
        echo "    ✓ OK"
    else
        echo "    ✗ FAILED — see error above"
    fi
else
    echo "  ✗ SKIP regions: file not found at ${REGIONS_FILE}"
fi

# departements
DEPTS_FILE="${DATA_DIR}/admin/departements.geojson"
if [[ -f "${DEPTS_FILE}" ]]; then
    echo "  → departements table..."
    run_sql "TRUNCATE TABLE departements;" 2>/dev/null || true
    if ogr2ogr \
        -f PostgreSQL "${PG_DSN}" \
        "${DEPTS_FILE}" \
        -nln departements \
        -nlt PROMOTE_TO_MULTI \
        -lco GEOMETRY_NAME=geom \
        -lco FID=id \
        -lco OVERWRITE=YES; then
        echo "    ✓ OK"
    else
        echo "    ✗ FAILED — see error above"
    fi
else
    echo "  ✗ SKIP departements: file not found at ${DEPTS_FILE}"
fi

# communes
echo "  → communes table..."
run_sql "TRUNCATE TABLE communes;" 2>/dev/null || true
COMM_IMPORTED=0
for dept in "${DEPTS[@]}"; do
    comm_file="${DATA_DIR}/admin/communes/${dept}-communes.geojson"
    if [[ ! -f "${comm_file}" ]]; then
        echo "    ✗ dept ${dept}: file not found"
        continue
    fi
    echo "    Importing dept ${dept}..."
    if ogr2ogr \
        -f PostgreSQL "${PG_DSN}" \
        "${comm_file}" \
        -nln communes \
        -nlt PROMOTE_TO_MULTI \
        -lco GEOMETRY_NAME=geom \
        -lco FID=id \
        -append; then
        COMM_IMPORTED=$((COMM_IMPORTED + 1))
    else
        echo "    ✗ dept ${dept}: import failed"
    fi
done
echo "    Imported ${COMM_IMPORTED}/${#DEPTS[@]} departments"
echo ""

# ─── 2. Cadastre Parcelles ────────────────────────────────────────────────────
echo "[2/3] Importing cadastre parcelles"
echo ""
run_sql "TRUNCATE TABLE cadastre_parcelles;" 2>/dev/null || true

CAD_OK=0
CAD_FAIL=0

for dept in "${DEPTS[@]}"; do
    cad_file="${DATA_DIR}/cadastre/${dept}_cadastre.geojson"
    if [[ ! -f "${cad_file}" ]]; then
        echo "  ✗ Dept ${dept}: file not found at ${cad_file}"
        CAD_FAIL=$((CAD_FAIL + 1))
        continue
    fi
    echo "  Importing dept ${dept}..."
    if ogr2ogr \
        -f PostgreSQL "${PG_DSN}" \
        "${cad_file}" \
        -nln cadastre_parcelles \
        -nlt PROMOTE_TO_MULTI \
        -lco GEOMETRY_NAME=geom \
        -lco FID=id \
        -append; then
        CAD_OK=$((CAD_OK + 1))
    else
        echo "    ✗ Failed for dept ${dept}"
        CAD_FAIL=$((CAD_FAIL + 1))
    fi
done

echo "  Cadastre import — OK: ${CAD_OK}, Failed: ${CAD_FAIL}"
echo ""

# ─── 3. BD Forêt V2 ───────────────────────────────────────────────────────────
echo "[3/3] Importing BD Forêt V2 (IGN shapefiles)"
echo ""
run_sql "TRUNCATE TABLE forest_parcels;" 2>/dev/null || true

BDFORET_IMPORTED=0

for dept in "${DEPTS[@]}"; do
    bdforet_dir="${DATA_DIR}/bdforet/${dept}"
    shp_file="${bdforet_dir}/FORMATION_VEGETALE.shp"
    
    if [[ ! -f "${shp_file}" ]]; then
        echo "  ✗ Dept ${dept}: shapefile not found at ${shp_file}"
        continue
    fi
    
    echo "  Importing BD Forêt dept ${dept}..."
    if ogr2ogr \
        -f PostgreSQL "${PG_DSN}" \
        "${shp_file}" \
        -nln forest_parcels \
        -nlt PROMOTE_TO_MULTI \
        -lco GEOMETRY_NAME=geom \
        -lco FID=id \
        -append; then
        BDFORET_IMPORTED=$((BDFORET_IMPORTED + 1))
    else
        echo "    ✗ Failed for dept ${dept}"
    fi
done

if [[ "${BDFORET_IMPORTED}" -eq 0 ]]; then
    echo "  No BD Forêt data imported. See download-data.sh for manual steps."
fi
echo ""

# ─── Summary ──────────────────────────────────────────────────────────────────
echo "==================================================================="
echo " Import Summary"
echo "==================================================================="
for table in regions departements communes cadastre_parcelles forest_parcels; do
    count=$(run_sql "SELECT COUNT(*) FROM ${table};" | tail -1)
    printf "  %-25s %s rows\n" "${table}" "${count}"
done

echo ""
echo "  Next step: docker-compose up --build"
echo ""