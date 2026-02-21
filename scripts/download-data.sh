#!/usr/bin/env bash
# scripts/download-data.sh
# Downloads all data needed for Forest BD Viewer:
#   1. Admin boundaries (regions, departements, communes) from geo.api.gouv.fr
#   2. Cadastre parcelles per commune from etalab
#
# BD Forêt V2 (IGN) must be downloaded manually — see instructions at the end.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="${SCRIPT_DIR}/../data/raw"

# Target departments for Île-de-France (adjust as needed)
DEPTS=("78" "91" "95")

# ─── Directories ─────────────────────────────────────────────────────────────
mkdir -p "${DATA_DIR}/admin/communes"
mkdir -p "${DATA_DIR}/cadastre"

echo "==================================================================="
echo " Forest BD Viewer — Data Downloader"
echo "==================================================================="
echo ""

# ─── 1. Admin Boundaries ─────────────────────────────────────────────────────
echo "[1/3] Downloading admin boundaries from geo.api.gouv.fr"
echo ""

# Regions (France-wide; the import script will filter IDF = 11)
REGIONS_URL="https://geo.api.gouv.fr/regions?fields=code,nom&geometry=geom&format=geojson"
REGIONS_OUT="${DATA_DIR}/admin/regions.geojson"
echo "  → Regions..."
if curl -L --fail --silent --max-time 60 -o "${REGIONS_OUT}" "${REGIONS_URL}"; then
    size=$(du -h "${REGIONS_OUT}" | cut -f1)
    echo "    OK (${size})"
else
    echo "    WARN: Failed to download regions"
    rm -f "${REGIONS_OUT}"
fi

# Departments (France-wide; import script uses code filter too)
DEPTS_URL="https://geo.api.gouv.fr/departements?fields=code,nom,region&geometry=geom&format=geojson"
DEPTS_OUT="${DATA_DIR}/admin/departements.geojson"
echo "  → Departments..."
if curl -L --fail --silent --max-time 60 -o "${DEPTS_OUT}" "${DEPTS_URL}"; then
    size=$(du -h "${DEPTS_OUT}" | cut -f1)
    echo "    OK (${size})"
else
    echo "    WARN: Failed to download departments"
    rm -f "${DEPTS_OUT}"
fi

# Communes per target department (geometry=contour for full polygons)
echo "  → Communes..."
for dept in "${DEPTS[@]}"; do
    COMM_URL="https://geo.api.gouv.fr/departements/${dept}/communes?fields=code,nom&geometry=contour&format=geojson"
    COMM_OUT="${DATA_DIR}/admin/communes/${dept}-communes.geojson"
    if curl -L --fail --silent --max-time 120 -o "${COMM_OUT}" "${COMM_URL}"; then
        count=$(python3 -c "import json,sys; d=json.load(open('${COMM_OUT}')); print(len(d['features']))" 2>/dev/null || echo "?")
        echo "    Dept ${dept}: ${count} communes"
    else
        echo "    WARN: Failed to download communes for dept ${dept}"
        rm -f "${COMM_OUT}"
    fi
done

echo ""

# ─── 2. Cadastre Parcelles ────────────────────────────────────────────────────
echo "[2/3] Downloading cadastre parcelles from etalab"
echo "      (one file per commune — this will take a while)"
echo ""

BASE_URL="https://cadastre.data.gouv.fr/data/etalab-cadastre/latest/geojson/communes"

total_ok=0
total_skip=0
total_fail=0

for dept in "${DEPTS[@]}"; do
    comm_file="${DATA_DIR}/admin/communes/${dept}-communes.geojson"
    if [[ ! -f "${comm_file}" ]]; then
        echo "  SKIP dept ${dept}: commune list not downloaded"
        continue
    fi

    # Extract commune codes from the GeoJSON properties.code field
    mapfile -t commune_codes < <(
        python3 -c "
import json, sys
with open('${comm_file}') as f:
    data = json.load(f)
for feat in data['features']:
    print(feat['properties']['code'])
" 2>/dev/null
    )

    echo "  Dept ${dept}: ${#commune_codes[@]} communes to process"
    mkdir -p "${DATA_DIR}/cadastre/${dept}"

    for commune in "${commune_codes[@]}"; do
        out_dir="${DATA_DIR}/cadastre/${dept}/${commune}"
        out_file="${out_dir}/parcelles.geojson"
        gz_file="${out_dir}/parcelles.geojson.gz"

        # Skip if already downloaded
        if [[ -f "${out_file}" ]]; then
            total_ok=$((total_ok + 1))
            continue
        fi

        mkdir -p "${out_dir}"
        url="${BASE_URL}/${dept}/${commune}/cadastre-${commune}-parcelles.json.gz"

        http_code=$(curl -L --silent --max-time 30 -o "${gz_file}" \
            --write-out "%{http_code}" "${url}")

        if [[ "${http_code}" == "200" ]] && [[ -s "${gz_file}" ]]; then
            if gunzip -c "${gz_file}" > "${out_file}"; then
                rm -f "${gz_file}"
                total_ok=$((total_ok + 1))
            else
                rm -f "${gz_file}" "${out_file}"
                total_fail=$((total_fail + 1))
                echo "    WARN: commune ${commune} gunzip failed"
            fi
        elif [[ "${http_code}" == "404" ]]; then
            # Many communes have no cadastre data — that's normal
            rm -f "${gz_file}" "${out_file}"
            total_skip=$((total_skip + 1))
        else
            rm -f "${gz_file}" "${out_file}"
            total_fail=$((total_fail + 1))
            echo "    WARN: commune ${commune} HTTP ${http_code}"
        fi
    done

    echo "    Done (ok=${total_ok}, 404=${total_skip}, errors=${total_fail})"
done

echo ""

# ─── 3. BD Forêt V2 — Manual download required ───────────────────────────────
echo "[3/3] BD Forêt V2 (IGN) — MANUAL DOWNLOAD REQUIRED"
echo ""
echo "  BD Forêt V2 is a licensed dataset from IGN (Institut national de"
echo "  l'information géographique et forestière) and cannot be downloaded"
echo "  automatically."
echo ""
echo "  Steps:"
echo "  1. Go to: https://geoservices.ign.fr/bdforet"
echo "  2. Create a free account and accept the open-licence terms."
echo "  3. Download the SHP archive for each department:"
echo "       - 77 (Seine-et-Marne)"
echo "       - 78 (Yvelines)"
echo "       - 91 (Essonne)"
echo "       - 95 (Val-d'Oise)"
echo "  4. Extract and place shapefiles here:"
echo "       data/raw/bdforet/{dept}/FORMATION_VEGETALE.shp"
echo "       (and accompanying .dbf / .prj / .shx)"
echo "  5. Then run: ./scripts/import-data.sh"
echo "     The import script will pick them up automatically."
echo ""

# ─── Summary ──────────────────────────────────────────────────────────────────
echo "==================================================================="
echo " Summary"
echo "==================================================================="
echo ""
echo "  Admin boundaries:"
[[ -f "${DATA_DIR}/admin/regions.geojson" ]]       && echo "    regions.geojson ✓"     || echo "    regions.geojson MISSING"
[[ -f "${DATA_DIR}/admin/departements.geojson" ]]  && echo "    departements.geojson ✓" || echo "    departements.geojson MISSING"
for dept in "${DEPTS[@]}"; do
    f="${DATA_DIR}/admin/communes/${dept}-communes.geojson"
    [[ -f "${f}" ]] && echo "    communes/${dept}-communes.geojson ✓" || echo "    communes/${dept}-communes.geojson MISSING"
done

echo ""
echo "  Cadastre (parcelles downloaded): ${total_ok}"
echo "  Cadastre (no data for commune):   ${total_skip}"
echo "  Cadastre (errors):                ${total_fail}"
echo ""
echo "  BD Forêt: manual download required (see instructions above)"
echo ""
echo "  Next step: ./scripts/import-data.sh"
echo ""
