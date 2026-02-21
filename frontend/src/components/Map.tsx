'use client';

import { useRef, useEffect, useState, useCallback } from 'react';
import mapboxgl from 'mapbox-gl';
import MapboxDraw from '@mapbox/mapbox-gl-draw';
import 'mapbox-gl/dist/mapbox-gl.css';
import '@mapbox/mapbox-gl-draw/dist/mapbox-gl-draw.css';
import styles from './Map.module.css';
import { useAppDispatch, useAppSelector } from '@/store';
import { setMapState, saveMapStateThunk } from '@/store/mapSlice';
import { startDrawing, clearAnalysis, analyzePolygonThunk } from '@/store/analysisSlice';
import AnalysisPanel from './AnalysisPanel';

// ── BD Forêt V2 classification ────────────────────────────────────────────────

const TFV_COLORS: Record<string, string> = {
  FF1: '#1a7a3c',
  FF2: '#2d9e55',
  FF3: '#52b788',
  FF4: '#40916c',
  FO1: '#74c69d',
  FO2: '#95d5b2',
  FO3: '#b7e4c7',
  LA: '#d4e157',
  FP: '#aed9c9',
};

const TFV_LABELS: Record<string, string> = {
  FF1: 'Forêt fermée feuillus',
  FF2: 'Forêt fermée conifères',
  FF3: 'Forêt fermée mixte',
  FF4: 'Forêt fermée autre',
  FO1: 'Forêt ouverte feuillus',
  FO2: 'Forêt ouverte conifères',
  FO3: 'Forêt ouverte mixte',
  LA: 'Lande',
  FP: 'Peupleraie',
};

const DEFAULT_FOREST_COLOR = '#a8d5a2';

function buildColorExpression(): mapboxgl.Expression {
  const expr: mapboxgl.Expression = ['match', ['get', 'code_tfv']];
  for (const [code, color] of Object.entries(TFV_COLORS)) {
    expr.push(code, color);
  }
  expr.push(DEFAULT_FOREST_COLOR);
  return expr;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function geometryBounds(geometry: GeoJSON.Geometry): mapboxgl.LngLatBounds {
  const bounds = new mapboxgl.LngLatBounds();

  const extendRings = (rings: number[][][]) => {
    for (const ring of rings) {
      for (const coord of ring) {
        bounds.extend([coord[0], coord[1]] as [number, number]);
      }
    }
  };

  if (geometry.type === 'Polygon') {
    extendRings(geometry.coordinates);
  } else if (geometry.type === 'MultiPolygon') {
    for (const poly of geometry.coordinates) {
      extendRings(poly);
    }
  }

  return bounds;
}

// ── Zoom-tier legend logic ────────────────────────────────────────────────────

type ZoomTier = 'regions' | 'departements' | 'communes' | 'foret' | 'cadastre';

function zoomTier(zoom: number): ZoomTier {
  if (zoom < 8) return 'regions';
  if (zoom < 11) return 'departements';
  if (zoom < 14) return 'communes';
  if (zoom < 15) return 'foret';
  return 'cadastre';
}

const TIER_LABELS: Record<ZoomTier, string> = {
  regions: 'Régions (zoom 5–7)',
  departements: 'Départements (zoom 8–10)',
  communes: 'Communes (zoom 11–13)',
  foret: 'BD Forêt V2 (zoom 14)',
  cadastre: 'BD Forêt + Parcelles (zoom 15+)',
};

// ── Component ─────────────────────────────────────────────────────────────────

export default function Map() {
  const mapContainer = useRef<HTMLDivElement>(null);
  const map = useRef<mapboxgl.Map | null>(null);
  const popup = useRef<mapboxgl.Popup | null>(null);
  const drawRef = useRef<MapboxDraw | null>(null);
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [currentZoom, setCurrentZoom] = useState(6);

  const dispatch = useAppDispatch();
  const { center, zoom: savedZoom, hydrated } = useAppSelector((s) => s.map);
  const isLoggedIn = useAppSelector((s) => s.auth.user !== null);
  const analysisStatus = useAppSelector((s) => s.analysis.status);

  // ── Draw control handlers ─────────────────────────────────────────────────

  const handleDrawClick = () => {
    if (!map.current || !drawRef.current) return;
    if (analysisStatus === 'drawing') {
      // Cancel: exit draw mode and clear everything
      drawRef.current.changeMode('simple_select');
      drawRef.current.deleteAll();
      dispatch(clearAnalysis());
    } else {
      // Start: delete any previous polygon and enter draw_polygon mode
      drawRef.current.deleteAll();
      dispatch(startDrawing());
      drawRef.current.changeMode('draw_polygon');
    }
  };

  // Called from AnalysisPanel's close/clear button
  const handleAnalysisClear = useCallback(() => {
    if (drawRef.current) {
      drawRef.current.deleteAll();
      drawRef.current.changeMode('simple_select');
    }
  }, []);

  useEffect(() => {
    if (!hydrated) return;
    if (map.current || !mapContainer.current) return;

    const token = process.env.NEXT_PUBLIC_MAPBOX_TOKEN;
    if (!token || token === 'pk.your_mapbox_token_here') {
      console.warn('Mapbox token not configured');
      return;
    }

    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

    mapboxgl.accessToken = token;

    map.current = new mapboxgl.Map({
      container: mapContainer.current,
      style: 'mapbox://styles/mapbox/light-v11',
      center: center,
      zoom: savedZoom,
      attributionControl: true,
      transformRequest: (url, resourceType) => {
        if (resourceType === 'Tile' && url.startsWith(apiUrl)) {
          return { url, credentials: 'include' };
        }
        return { url };
      },
    });

    map.current.addControl(new mapboxgl.NavigationControl(), 'top-right');
    map.current.addControl(new mapboxgl.ScaleControl(), 'bottom-left');

    // Attach MapboxDraw (no built-in toolbar — we supply our own button)
    const draw = new MapboxDraw({
      displayControlsDefault: false,
    });
    map.current.addControl(draw);
    drawRef.current = draw;

    // Sync legend zoom
    map.current.on('zoom', () => {
      setCurrentZoom(Math.round(map.current!.getZoom()));
    });

    // Persist map position (debounced 1 s)
    map.current.on('moveend', () => {
      const c = map.current!.getCenter();
      const z = map.current!.getZoom();
      dispatch(setMapState({ center: [c.lng, c.lat], zoom: z }));
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
      saveTimerRef.current = setTimeout(() => {
        dispatch(saveMapStateThunk({ lng: c.lng, lat: c.lat, zoom: z }));
      }, 1000);
    });

    map.current.on('load', () => {
      const m = map.current!;

      // ── Sources ───────────────────────────────────────────────────────
      m.addSource('regions', {
        type: 'vector',
        tiles: [`${apiUrl}/tiles/admin/regions/{z}/{x}/{y}.mvt`],
        minzoom: 0,
        maxzoom: 8,
      });

      m.addSource('departements', {
        type: 'vector',
        tiles: [`${apiUrl}/tiles/admin/departements/{z}/{x}/{y}.mvt`],
        minzoom: 6,
        maxzoom: 12,
      });

      m.addSource('communes', {
        type: 'vector',
        tiles: [`${apiUrl}/tiles/admin/communes/{z}/{x}/{y}.mvt`],
        minzoom: 9,
        maxzoom: 15,
      });

      m.addSource('foret', {
        type: 'vector',
        tiles: [`${apiUrl}/tiles/foret/{z}/{x}/{y}.mvt`],
        minzoom: 8,
        maxzoom: 16,
      });

      m.addSource('cadastre', {
        type: 'vector',
        tiles: [`${apiUrl}/tiles/cadastre/{z}/{x}/{y}.mvt`],
        minzoom: 14,
        maxzoom: 18,
      });

      // ── Regions (zoom 5–7) ────────────────────────────────────────────
      m.addLayer({
        id: 'regions-fill',
        type: 'fill',
        source: 'regions',
        'source-layer': 'regions',
        minzoom: 5,
        maxzoom: 8,
        paint: { 'fill-color': '#4a90d9', 'fill-opacity': 0.12 },
      });
      m.addLayer({
        id: 'regions-outline',
        type: 'line',
        source: 'regions',
        'source-layer': 'regions',
        minzoom: 5,
        maxzoom: 8,
        paint: { 'line-color': '#1a5fa8', 'line-width': 1.5, 'line-opacity': 0.7 },
      });

      // ── Departments (zoom 8–10) ───────────────────────────────────────
      m.addLayer({
        id: 'departements-fill',
        type: 'fill',
        source: 'departements',
        'source-layer': 'departements',
        minzoom: 8,
        maxzoom: 11,
        paint: { 'fill-color': '#7b5ea7', 'fill-opacity': 0.12 },
      });
      m.addLayer({
        id: 'departements-outline',
        type: 'line',
        source: 'departements',
        'source-layer': 'departements',
        minzoom: 8,
        maxzoom: 11,
        paint: { 'line-color': '#5a3f8a', 'line-width': 1.5, 'line-opacity': 0.7 },
      });

      // ── Communes (zoom 11–13) ─────────────────────────────────────────
      m.addLayer({
        id: 'communes-fill',
        type: 'fill',
        source: 'communes',
        'source-layer': 'communes',
        minzoom: 11,
        maxzoom: 14,
        paint: { 'fill-color': '#f0a500', 'fill-opacity': 0.08 },
      });
      m.addLayer({
        id: 'communes-outline',
        type: 'line',
        source: 'communes',
        'source-layer': 'communes',
        minzoom: 11,
        maxzoom: 14,
        paint: { 'line-color': '#c87800', 'line-width': 0.8, 'line-opacity': 0.6 },
      });

      // ── BD Forêt (zoom 14+) ───────────────────────────────────────────
      m.addLayer({
        id: 'foret-fill',
        type: 'fill',
        source: 'foret',
        'source-layer': 'forest',
        minzoom: 14,
        paint: { 'fill-color': buildColorExpression(), 'fill-opacity': 0.7 },
      });
      m.addLayer({
        id: 'foret-outline',
        type: 'line',
        source: 'foret',
        'source-layer': 'forest',
        minzoom: 14,
        paint: { 'line-color': '#2d6a4f', 'line-width': 0.5, 'line-opacity': 0.4 },
      });

      // ── Cadastre parcelles (zoom 15+) ─────────────────────────────────
      m.addLayer({
        id: 'cadastre-fill',
        type: 'fill',
        source: 'cadastre',
        'source-layer': 'cadastre',
        minzoom: 15,
        paint: { 'fill-color': '#e8c97a', 'fill-opacity': 0.2 },
      });
      m.addLayer({
        id: 'cadastre-outline',
        type: 'line',
        source: 'cadastre',
        'source-layer': 'cadastre',
        minzoom: 15,
        paint: { 'line-color': '#b8860b', 'line-width': 0.7, 'line-opacity': 0.7 },
      });

      // ── Pointer cursors ───────────────────────────────────────────────
      const interactiveLayers = [
        'regions-fill', 'departements-fill', 'communes-fill',
        'foret-fill', 'cadastre-fill',
      ];
      for (const layerId of interactiveLayers) {
        m.on('mouseenter', layerId, () => { m.getCanvas().style.cursor = 'pointer'; });
        m.on('mouseleave', layerId, () => { m.getCanvas().style.cursor = ''; });
      }

      // ── Click: Region → zoom to region ───────────────────────────────
      m.on('click', 'regions-fill', (e) => {
        if (!e.features || e.features.length === 0) return;
        const feat = e.features[0];
        const nom: string = feat.properties?.nom ?? 'Région';
        if (feat.geometry) {
          m.fitBounds(geometryBounds(feat.geometry as GeoJSON.Geometry), { padding: 40 });
        }
        popup.current?.remove();
        popup.current = new mapboxgl.Popup({ closeButton: false })
          .setLngLat(e.lngLat)
          .setHTML(`<strong>${nom}</strong>`)
          .addTo(m);
        window.setTimeout(() => popup.current?.remove(), 1500);
      });

      // ── Click: Department → zoom to department ────────────────────────
      m.on('click', 'departements-fill', (e) => {
        if (!e.features || e.features.length === 0) return;
        const feat = e.features[0];
        const nom: string = feat.properties?.nom ?? 'Département';
        if (feat.geometry) {
          m.fitBounds(geometryBounds(feat.geometry as GeoJSON.Geometry), { padding: 40 });
        }
        popup.current?.remove();
        popup.current = new mapboxgl.Popup({ closeButton: false })
          .setLngLat(e.lngLat)
          .setHTML(`<strong>${nom}</strong>`)
          .addTo(m);
        window.setTimeout(() => popup.current?.remove(), 1500);
      });

      // ── Click: Commune → zoom to commune ─────────────────────────────
      m.on('click', 'communes-fill', (e) => {
        if (!e.features || e.features.length === 0) return;
        const feat = e.features[0];
        const nom: string = feat.properties?.nom ?? 'Commune';
        if (feat.geometry) {
          m.fitBounds(geometryBounds(feat.geometry as GeoJSON.Geometry), { padding: 40 });
        }
        popup.current?.remove();
        popup.current = new mapboxgl.Popup({ closeButton: false })
          .setLngLat(e.lngLat)
          .setHTML(`<strong>${nom}</strong>`)
          .addTo(m);
        window.setTimeout(() => popup.current?.remove(), 1500);
      });

      // ── Click: BD Forêt → feature popup ──────────────────────────────
      m.on('click', 'foret-fill', (e) => {
        if (!e.features || e.features.length === 0) return;
        const props = e.features[0].properties ?? {};
        const codeTfv: string = props.code_tfv ?? '—';
        const libTfv: string = props.lib_tfv ?? TFV_LABELS[codeTfv] ?? '—';
        const essence1: string = props.essence1 ?? '—';
        const dept: string = props.departement ?? '—';

        popup.current?.remove();
        popup.current = new mapboxgl.Popup({ closeButton: true, maxWidth: '280px' })
          .setLngLat(e.lngLat)
          .setHTML(
            `<div style="font-family:sans-serif;font-size:13px;line-height:1.6">
              <strong>${libTfv}</strong><br/>
              <span style="color:#555">Code TFV:</span> ${codeTfv}<br/>
              <span style="color:#555">Essence principale:</span> ${essence1}<br/>
              <span style="color:#555">Département:</span> ${dept}
            </div>`
          )
          .addTo(m);
      });

      // ── Click: Cadastre → parcel popup ────────────────────────────────
      m.on('click', 'cadastre-fill', (e) => {
        if (!e.features || e.features.length === 0) return;
        const props = e.features[0].properties ?? {};
        const section: string = props.section ?? '—';
        const numero: string = props.numero ?? '—';
        const commune: string = props.commune ?? '—';

        popup.current?.remove();
        popup.current = new mapboxgl.Popup({ closeButton: true, maxWidth: '240px' })
          .setLngLat(e.lngLat)
          .setHTML(
            `<div style="font-family:sans-serif;font-size:13px;line-height:1.6">
              <strong>Parcelle cadastrale</strong><br/>
              <span style="color:#555">Section:</span> ${section}<br/>
              <span style="color:#555">Numéro:</span> ${numero}<br/>
              <span style="color:#555">Commune:</span> ${commune}
            </div>`
          )
          .addTo(m);
      });

      // ── Draw events ───────────────────────────────────────────────────
      // Fired when the user completes a polygon by double-clicking.
      m.on('draw.create', () => {
        const fc = draw.getAll();
        if (fc.features.length === 0) return;
        const geom = fc.features[0].geometry;
        dispatch(analyzePolygonThunk(JSON.stringify(geom)));
      });

      setLoaded(true);
    });

    return () => {
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
      popup.current?.remove();
      map.current?.remove();
      map.current = null;
      drawRef.current = null;
    };
  }, [hydrated]); // eslint-disable-line react-hooks/exhaustive-deps

  const tier = zoomTier(currentZoom);

  return (
    <div className={styles.mapWrapper}>
      <div ref={mapContainer} className={styles.mapContainer} />

      {!loaded && (
        <div className={styles.loadingOverlay}>
          <div className={styles.spinner} />
          <p>Loading map...</p>
        </div>
      )}

      {/* Analysis panel — always rendered so it can show its "drawing" hint */}
      <AnalysisPanel onClear={handleAnalysisClear} />

      {loaded && (
        <>
          {/* Draw / analyse button — only visible to authenticated users */}
          {isLoggedIn && (
            <button
              className={`${styles.drawBtn}${analysisStatus === 'drawing' ? ` ${styles.drawBtnActive}` : ''}`}
              onClick={handleDrawClick}
              title={analysisStatus === 'drawing' ? 'Annuler le dessin' : 'Tracer un polygone et analyser la couverture forestière'}
            >
              {analysisStatus === 'drawing' ? '✕ Annuler' : '⬡ Analyser une zone'}
            </button>
          )}

          {/* Zoom-tier legend */}
          <div className={styles.legend}>
            <p className={styles.legendTitle}>{TIER_LABELS[tier]}</p>

            {tier === 'regions' && (
              <div className={styles.legendItem}>
                <span
                  className={styles.legendSwatch}
                  style={{ background: '#4a90d9', border: '1.5px solid #1a5fa8' }}
                />
                <span>Région — cliquer pour zoomer</span>
              </div>
            )}

            {tier === 'departements' && (
              <div className={styles.legendItem}>
                <span
                  className={styles.legendSwatch}
                  style={{ background: '#7b5ea7', border: '1.5px solid #5a3f8a' }}
                />
                <span>Département — cliquer pour zoomer</span>
              </div>
            )}

            {tier === 'communes' && (
              <div className={styles.legendItem}>
                <span
                  className={styles.legendSwatch}
                  style={{ background: '#f0a500', border: '1.5px solid #c87800' }}
                />
                <span>Commune — cliquer pour zoomer</span>
              </div>
            )}

            {(tier === 'foret' || tier === 'cadastre') && (
              <>
                {Object.entries(TFV_LABELS).map(([code, label]) => (
                  <div key={code} className={styles.legendItem}>
                    <span
                      className={styles.legendSwatch}
                      style={{ background: TFV_COLORS[code] ?? DEFAULT_FOREST_COLOR }}
                    />
                    <span>{label}</span>
                  </div>
                ))}
                <div className={styles.legendItem}>
                  <span
                    className={styles.legendSwatch}
                    style={{ background: DEFAULT_FOREST_COLOR }}
                  />
                  <span>Autre forêt</span>
                </div>
              </>
            )}

            {tier === 'cadastre' && (
              <div
                className={styles.legendItem}
                style={{ marginTop: 6, borderTop: '1px solid #eee', paddingTop: 6 }}
              >
                <span
                  className={styles.legendSwatch}
                  style={{ background: '#e8c97a', border: '1.5px solid #b8860b' }}
                />
                <span>Parcelle cadastrale</span>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
