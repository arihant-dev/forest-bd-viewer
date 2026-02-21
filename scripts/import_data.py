#!/usr/bin/env python3
"""
scripts/import_data.py

Imports all downloaded spatial data into PostGIS:
  1. Admin boundaries  → regions, departements, communes
  2. Cadastre parcelles → cadastre_parcelles
  3. BD Forêt V2 (IGN SHP) → forest_parcels  (via ogr2ogr)
"""

import glob
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

import psycopg2
from psycopg2.extras import execute_batch

# ── Config ────────────────────────────────────────────────────────────────────

DATA_DIR = "/app/data/raw"
DEPTS = ["78", "91", "95"]
BATCH_SIZE = 500

DB_CFG = {
    "host":     os.environ.get("POSTGRES_HOST", "postgres"),
    "port":     int(os.environ.get("POSTGRES_PORT", "5432")),
    "dbname":   os.environ.get("POSTGRES_DB", "forest_bd"),
    "user":     os.environ.get("POSTGRES_USER", "forestviewer"),
    "password": os.environ.get("POSTGRES_PASSWORD", "forestviewer_secret"),
}

SEP = "=" * 67

# ── Helpers ───────────────────────────────────────────────────────────────────

def connect():
    return psycopg2.connect(**DB_CFG)


def load_geojson(path):
    with open(path) as f:
        return json.load(f)


def check_connection():
    print("Testing database connection...")
    try:
        conn = connect()
        conn.close()
        print("✓ Database connected\n")
    except Exception as e:
        print(f"ERROR: Cannot connect to database: {e}")
        sys.exit(1)


def check_tables(conn):
    print("Checking required tables...")
    required = ["regions", "departements", "communes", "cadastre_parcelles", "forest_parcels"]
    with conn.cursor() as cur:
        for table in required:
            cur.execute(
                "SELECT 1 FROM information_schema.tables WHERE table_name = %s",
                (table,),
            )
            if not cur.fetchone():
                print(f"ERROR: Table '{table}' does not exist. Run migrations first.")
                sys.exit(1)
    print("✓ All required tables exist\n")


# ── 1. Admin boundaries ───────────────────────────────────────────────────────

def import_regions(conn):
    path = os.path.join(DATA_DIR, "admin", "regions.geojson")
    if not os.path.exists(path):
        print(f"  ✗ SKIP regions: file not found at {path}")
        return

    data = load_geojson(path)
    if isinstance(data, list) or "features" not in data:
        print("  ✗ SKIP regions: file has no features (re-run download-data.sh)")
        return

    features = data["features"]
    print(f"  → regions: {len(features)} features...")

    rows = [
        (
            feat.get("properties", {}).get("code", ""),
            feat.get("properties", {}).get("nom", ""),
            json.dumps(feat["geometry"]),
        )
        for feat in features
    ]

    with conn.cursor() as cur:
        cur.execute("TRUNCATE TABLE regions RESTART IDENTITY CASCADE;")
        execute_batch(
            cur,
            "INSERT INTO regions (code, nom, geom) VALUES (%s, %s,"
            " ST_Multi(ST_GeomFromGeoJSON(%s))::geometry(MultiPolygon,4326))",
            rows,
        )
    conn.commit()
    print(f"    ✓ OK ({len(rows)} rows)")


def import_departements(conn):
    path = os.path.join(DATA_DIR, "admin", "departements.geojson")
    if not os.path.exists(path):
        print(f"  ✗ SKIP departements: file not found at {path}")
        return

    data = load_geojson(path)
    if isinstance(data, list) or "features" not in data:
        print("  ✗ SKIP departements: file has no features (re-run download-data.sh)")
        return

    features = data["features"]
    print(f"  → departements: {len(features)} features...")

    rows = []
    for feat in features:
        p = feat.get("properties", {})
        region_obj = p.get("region", {})
        region_code = region_obj.get("code") if isinstance(region_obj, dict) else None
        rows.append((p.get("code", ""), p.get("nom", ""), region_code, json.dumps(feat["geometry"])))

    with conn.cursor() as cur:
        cur.execute("TRUNCATE TABLE departements RESTART IDENTITY CASCADE;")
        execute_batch(
            cur,
            "INSERT INTO departements (code, nom, region_code, geom) VALUES (%s, %s, %s,"
            " ST_Multi(ST_GeomFromGeoJSON(%s))::geometry(MultiPolygon,4326))",
            rows,
        )
        # Fill in region_code where absent via spatial containment
        cur.execute(
            "UPDATE departements d SET region_code = r.code"
            " FROM regions r"
            " WHERE d.region_code IS NULL"
            "   AND ST_Within(ST_Centroid(d.geom), r.geom)"
        )
    conn.commit()
    print(f"    ✓ OK ({len(rows)} rows)")


def import_communes(conn):
    print("  → communes...")

    all_rows = []
    for dept in DEPTS:
        path = os.path.join(DATA_DIR, "admin", "communes", f"{dept}-communes.geojson")
        if not os.path.exists(path):
            print(f"    SKIP dept {dept}: file not found")
            continue
        features = load_geojson(path).get("features", [])
        for feat in features:
            p = feat.get("properties", {})
            all_rows.append((p.get("code", ""), p.get("nom", ""), dept, json.dumps(feat["geometry"])))
        print(f"    Dept {dept}: {len(features)} communes queued")

    if not all_rows:
        print("    No commune files found")
        return

    with conn.cursor() as cur:
        cur.execute("TRUNCATE TABLE communes RESTART IDENTITY CASCADE;")
        execute_batch(
            cur,
            "INSERT INTO communes (code, nom, departement_code, geom) VALUES (%s, %s, %s,"
            " ST_Multi(ST_GeomFromGeoJSON(%s))::geometry(MultiPolygon,4326))",
            all_rows,
        )
    conn.commit()
    print(f"    ✓ OK ({len(all_rows)} total rows)")


# ── 2. Cadastre parcelles ─────────────────────────────────────────────────────

def import_cadastre(conn):
    print("[2/3] Importing cadastre parcelles")
    print("      (iterating per-commune files — this may take a while)\n")

    with conn.cursor() as cur:
        cur.execute("TRUNCATE TABLE cadastre_parcelles RESTART IDENTITY;")
    conn.commit()

    total_ok = 0
    total_fail = 0

    for dept in DEPTS:
        cadastre_dir = os.path.join(DATA_DIR, "cadastre", dept)
        if not os.path.isdir(cadastre_dir):
            print(f"  SKIP dept {dept}: directory not found")
            continue

        commune_files = glob.glob(os.path.join(cadastre_dir, "*", "parcelles.geojson"))
        if not commune_files:
            print(f"  SKIP dept {dept}: no parcelles.geojson files found")
            continue

        print(f"  Dept {dept}: {len(commune_files)} commune files...")
        dept_ok = 0
        dept_fail = 0
        batch = []

        def flush(batch):
            with conn.cursor() as cur:
                execute_batch(
                    cur,
                    "INSERT INTO cadastre_parcelles (commune, departement, section, numero, geom)"
                    " VALUES (%s, %s, %s, %s,"
                    " ST_Multi(ST_GeomFromGeoJSON(%s))::geometry(MultiPolygon,4326))",
                    batch,
                )
            conn.commit()

        for cad_file in commune_files:
            try:
                data = load_geojson(cad_file)
            except Exception:
                dept_fail += 1
                continue

            for feat in data.get("features", []):
                p = feat.get("properties", {})
                geom = feat.get("geometry")
                if not p.get("commune") or not geom:
                    continue
                batch.append((
                    p.get("commune", ""),
                    dept,
                    p.get("section", ""),
                    p.get("numero", ""),
                    json.dumps(geom),
                ))

                if len(batch) >= BATCH_SIZE:
                    try:
                        flush(batch)
                        dept_ok += len(batch)
                    except Exception:
                        conn.rollback()
                        dept_fail += len(batch)
                    batch = []

        if batch:
            try:
                flush(batch)
                dept_ok += len(batch)
            except Exception:
                conn.rollback()
                dept_fail += len(batch)

        print(f"    ✓ {dept_ok} parcelles imported, {dept_fail} failed")
        total_ok += dept_ok
        total_fail += dept_fail

    print(f"  Cadastre total — OK: {total_ok}, Failed: {total_fail}\n")


# ── 3. BD Forêt V2 (ogr2ogr) ─────────────────────────────────────────────────

def _shp_fields(shp_path):
    """Return the set of field names present in the first layer of a SHP file."""
    result = subprocess.run(
        ["ogrinfo", "-al", "-so", shp_path],
        capture_output=True, text=True,
    )
    fields = set()
    for line in result.stdout.splitlines():
        m = re.match(r"^([A-Z_][A-Z0-9_]*):", line)
        if m:
            fields.add(m.group(1))
    return fields


def _bdforet_sql(fields, dept):
    """
    Build an OGR SQL SELECT that maps whatever fields exist in this SHP to the
    canonical aliases expected by the forest_parcels table.

    Known schemas encountered in the wild:
      Old (BD Forêt V1-style): DEP CYCLE ANREF TFIFN LIBELLE [LIBELLE2] TYPN NOM_TYPN
      New (BD Forêt V2):       CODE_TFV LIB_TFV ESSENCE1 ESSENCE2 CODE_COM …
      Mixed variant:           CODE_TFV TFV ESSENCE ID …
    """
    def pick(*candidates):
        for c in candidates:
            if c in fields:
                return c
        return None

    def col(src, alias):
        return f"{src} AS {alias}" if src else f"'' AS {alias}"

    parts = [
        col(pick("CODE_TFV", "TFIFN"),          "code_tfv"),
        col(pick("LIB_TFV",  "TFV", "LIBELLE"), "lib_tfv"),
        col(pick("ESSENCE1", "ESSENCE"),         "essence1"),
        col(pick("ESSENCE2"),                    "essence2"),
        f"'{dept}' AS departement",
        col(pick("CODE_COM"),                    "commune"),
    ]
    return "SELECT " + ", ".join(parts) + " FROM FORMATION_VEGETALE"


def import_bdforet(conn):
    print("[3/3] Importing BD Forêt V2 (IGN shapefiles)\n")

    with conn.cursor() as cur:
        cur.execute("TRUNCATE TABLE forest_parcels RESTART IDENTITY;")
    conn.commit()

    pg_dsn = (
        f"PG:host={DB_CFG['host']} port={DB_CFG['port']}"
        f" dbname={DB_CFG['dbname']} user={DB_CFG['user']}"
        f" password={DB_CFG['password']}"
    )

    imported = 0
    for dept in DEPTS:
        src_dir = os.path.join(DATA_DIR, "bdforet", dept)
        src_shp = os.path.join(src_dir, "FORMATION_VEGETALE.shp")
        if not os.path.exists(src_shp):
            print(f"  ✗ Dept {dept}: shapefile not found at {src_shp}")
            continue

        # Copy SHP bundle to a local /tmp directory before calling ogr2ogr.
        # Files on the bind-mounted volume may carry macOS extended attributes
        # (com.apple.provenance) that cause Docker's VirtioFS to return I/O
        # errors on reads issued by GDAL/ogr2ogr, even when Python's own I/O
        # works fine.  Copying strips the attributes and lands the data on the
        # container's native tmpfs, which GDAL can read without issue.
        tmp_dir = tempfile.mkdtemp(prefix=f"bdforet_{dept}_")
        try:
            for ext in (".shp", ".dbf", ".shx", ".prj", ".cpg"):
                src = os.path.join(src_dir, f"FORMATION_VEGETALE{ext}")
                if os.path.exists(src):
                    shutil.copy2(src, tmp_dir)

            tmp_shp = os.path.join(tmp_dir, "FORMATION_VEGETALE.shp")
            fields  = _shp_fields(tmp_shp)
            sql     = _bdforet_sql(fields, dept)

            print(f"  Importing BD Forêt dept {dept}...")
            result = subprocess.run(
                [
                    "ogr2ogr",
                    "-f", "PostgreSQL", pg_dsn,
                    tmp_shp,
                    "-nln",  "forest_parcels",
                    "-nlt",  "PROMOTE_TO_MULTI",
                    "-lco",  "GEOMETRY_NAME=geom",
                    "-append",
                    "-t_srs", "EPSG:4326",
                    "-sql", sql,
                    "-progress",
                ],
                capture_output=True,
            )
            if result.returncode == 0:
                imported += 1
                print("    ✓ OK")
            else:
                print(f"    ✗ Failed: {result.stderr.decode(errors='replace')[:500]}")
        finally:
            shutil.rmtree(tmp_dir, ignore_errors=True)

    if imported == 0:
        print("  No BD Forêt data imported. See download-data.sh for manual steps.")
    print()


# ── Summary ───────────────────────────────────────────────────────────────────

def print_summary(conn):
    print(SEP)
    print(" Import Summary")
    print(SEP)
    with conn.cursor() as cur:
        for table in ["regions", "departements", "communes", "cadastre_parcelles", "forest_parcels"]:
            cur.execute(f"SELECT COUNT(*) FROM {table}")  # noqa: S608 — table name is hardcoded
            count = cur.fetchone()[0]
            print(f"  {table:<25} {count} rows")
    print()
    print("  Next step: docker-compose up --build")
    print()


# ── Entry point ───────────────────────────────────────────────────────────────

def main():
    print(SEP)
    print(" Forest BD Viewer — PostGIS Data Importer")
    print(SEP)
    print(f"Data dir: {DATA_DIR}")
    print(f"DB: {DB_CFG['user']}@{DB_CFG['host']}:{DB_CFG['port']}/{DB_CFG['dbname']}\n")

    check_connection()

    conn = connect()
    try:
        check_tables(conn)

        print("[1/3] Importing admin boundaries\n")
        import_regions(conn)
        import_departements(conn)
        import_communes(conn)
        print()

        import_cadastre(conn)
        import_bdforet(conn)
        print_summary(conn)
    finally:
        conn.close()


if __name__ == "__main__":
    main()
