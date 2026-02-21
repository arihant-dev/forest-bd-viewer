import { createContext, useContext, useState, useCallback, ReactNode } from 'react';

// ── Supported locales ─────────────────────────────────────────────────────────

export type Locale = 'en' | 'fr';

// ── Dictionary ────────────────────────────────────────────────────────────────

const dict = {
  // AnalysisPanel
  'analysis.title':          { en: 'Polygon Analysis',                          fr: 'Analyse de polygone' },
  'analysis.hint':           { en: 'Draw a polygon on the map to analyse forest cover.\nClick to place points, then press Finish.', fr: 'Dessinez un polygone sur la carte pour analyser la couverture forestière.\nCliquez pour placer des points, puis appuyez sur Terminer.' },
  'analysis.loading':        { en: 'Analysing...',                              fr: 'Analyse en cours...' },
  'analysis.error':          { en: 'An error occurred.',                        fr: 'Une erreur est survenue.' },
  'analysis.totalArea':      { en: 'Total area',                                fr: 'Surface totale' },
  'analysis.forestCover':    { en: 'Forest cover',                              fr: 'Couverture forêt' },
  'analysis.forestShare':    { en: 'Forest share',                              fr: 'Part forestière' },
  'analysis.parcels':        { en: 'Parcels',                                   fr: 'Parcelles' },
  'analysis.tfvSection':     { en: 'Forest type (TFV)',                         fr: 'Type de forêt (TFV)' },
  'analysis.speciesSection': { en: 'Main species',                              fr: 'Essence principale' },
  'analysis.noParcels':      { en: 'No forest parcels in this area.',           fr: 'Aucune parcelle forestière dans cette zone.' },
  'analysis.close':          { en: 'Close',                                     fr: 'Fermer' },

  // Draw button
  'draw.analyse':            { en: 'Analyse area',                              fr: 'Analyser une zone' },
  'draw.cancel':             { en: 'Cancel',                                    fr: 'Annuler' },
  'draw.finish':             { en: 'Finish',                                    fr: 'Terminer' },
  'draw.titleStart':         { en: 'Draw a polygon to analyse forest cover',    fr: 'Tracer un polygone et analyser la couverture forestière' },
  'draw.titleCancel':        { en: 'Cancel drawing',                            fr: 'Annuler le dessin' },
  'draw.titleFinish':        { en: 'Complete the polygon',                      fr: 'Terminer le polygone' },

  // Legend — tier labels
  'legend.regions':          { en: 'Regions (zoom 5\u20137)',                   fr: 'Régions (zoom 5\u20137)' },
  'legend.departements':     { en: 'Departments (zoom 8\u201310)',              fr: 'Départements (zoom 8\u201310)' },
  'legend.communes':         { en: 'Municipalities (zoom 11\u201313)',          fr: 'Communes (zoom 11\u201313)' },
  'legend.foret':            { en: 'BD Forêt V2 (zoom 14)',                     fr: 'BD Forêt V2 (zoom 14)' },
  'legend.cadastre':         { en: 'BD Forêt + Parcels (zoom 15+)',             fr: 'BD Forêt + Parcelles (zoom 15+)' },

  // Legend — interactive hints
  'legend.clickToZoom':      { en: 'click to zoom',                             fr: 'cliquer pour zoomer' },
  'legend.otherForest':      { en: 'Other forest',                              fr: 'Autre forêt' },
  'legend.cadastreParcel':   { en: 'Cadastral parcel',                          fr: 'Parcelle cadastrale' },

  // TFV labels (BD Forêt classification names)
  'tfv.FF1':                 { en: 'Closed broadleaf forest',                   fr: 'Forêt fermée feuillus' },
  'tfv.FF2':                 { en: 'Closed conifer forest',                     fr: 'Forêt fermée conifères' },
  'tfv.FF3':                 { en: 'Closed mixed forest',                       fr: 'Forêt fermée mixte' },
  'tfv.FF4':                 { en: 'Closed forest (other)',                     fr: 'Forêt fermée autre' },
  'tfv.FO1':                 { en: 'Open broadleaf forest',                     fr: 'Forêt ouverte feuillus' },
  'tfv.FO2':                 { en: 'Open conifer forest',                       fr: 'Forêt ouverte conifères' },
  'tfv.FO3':                 { en: 'Open mixed forest',                         fr: 'Forêt ouverte mixte' },
  'tfv.LA':                  { en: 'Moorland',                                  fr: 'Lande' },
  'tfv.FP':                  { en: 'Poplar plantation',                         fr: 'Peupleraie' },

  // Popup labels
  'popup.region':            { en: 'Region',                                    fr: 'Région' },
  'popup.department':        { en: 'Department',                                fr: 'Département' },
  'popup.municipality':      { en: 'Municipality',                              fr: 'Commune' },
  'popup.tfvCode':           { en: 'TFV code',                                  fr: 'Code TFV' },
  'popup.mainSpecies':       { en: 'Main species',                              fr: 'Essence principale' },
  'popup.cadastreTitle':     { en: 'Cadastral parcel',                          fr: 'Parcelle cadastrale' },
  'popup.section':           { en: 'Section',                                   fr: 'Section' },
  'popup.number':            { en: 'Number',                                    fr: 'Numéro' },

  // Language toggle
  'lang.toggle':             { en: 'FR',                                        fr: 'EN' },
  'lang.toggleTitle':        { en: 'Switch to French',                          fr: 'Switch to English' },

  // LiDAR CHM analysis
  'lidar.section':           { en: 'Canopy Height Model (LiDAR)',              fr: 'Modèle de Hauteur de Canopée (LiDAR)' },
  'lidar.loading':           { en: 'Analysing LiDAR data...',                  fr: 'Analyse des données LiDAR...' },
  'lidar.noCoverage':        { en: 'No LiDAR HD coverage',                    fr: 'Pas de couverture LiDAR HD' },
  'lidar.minHeight':         { en: 'Min height',                               fr: 'Hauteur min' },
  'lidar.maxHeight':         { en: 'Max height',                               fr: 'Hauteur max' },
  'lidar.meanHeight':        { en: 'Mean height',                              fr: 'Hauteur moyenne' },
  'lidar.medianHeight':      { en: 'Median height',                            fr: 'Hauteur médiane' },
  'lidar.error':             { en: 'LiDAR analysis failed.',                   fr: 'Analyse LiDAR échouée.' },
  'lidar.overlay':           { en: 'CHM overlay shown on map',                   fr: 'Couche MHC affichée sur la carte' },
} as const;

export type DictKey = keyof typeof dict;

// ── Context ───────────────────────────────────────────────────────────────────

interface I18nContextType {
  locale: Locale;
  t: (key: DictKey) => string;
  toggle: () => void;
}

const I18nContext = createContext<I18nContextType>({
  locale: 'en',
  t: (key) => dict[key]?.en ?? key,
  toggle: () => {},
});

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>('en');

  const toggle = useCallback(() => {
    setLocale((prev) => (prev === 'en' ? 'fr' : 'en'));
  }, []);

  const t = useCallback(
    (key: DictKey): string => dict[key]?.[locale] ?? key,
    [locale],
  );

  return (
    <I18nContext.Provider value={{ locale, t, toggle }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n() {
  return useContext(I18nContext);
}
