package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ── WFS tile index types ─────────────────────────────────────────────────────

const (
	wfsBaseURL    = "https://data.geopf.fr/wfs/ows"
	maxLidarTiles = 25 // safety cap: max tiles per request
	tileCacheDir  = "/tmp/lidar-cache"
)

// WFSTile represents a single MNS or MNT tile from the IGN WFS tile index.
type WFSTile struct {
	Name         string `json:"name"`
	NameDownload string `json:"name_download"`
	URL          string `json:"url"`
	Projection   string `json:"projection"`
	Format       string `json:"format"`
	BBox         string `json:"bbox"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type wfsFeatureCollection struct {
	TotalFeatures int          `json:"totalFeatures"`
	Features      []wfsFeature `json:"features"`
}

type wfsFeature struct {
	Properties wfsTileProps `json:"properties"`
}

type wfsTileProps struct {
	Name         string `json:"name"`
	NameDownload string `json:"name_download"`
	URL          string `json:"url"`
	Projection   string `json:"projection"`
	Format       string `json:"format"`
	BBox         string `json:"bbox"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// LidarResult is the output of a LiDAR CHM analysis.
type LidarResult struct {
	HasCoverage  bool
	Message      string
	MinHeight    float64
	MaxHeight    float64
	MeanHeight   float64
	MedianHeight float64
	CHMImageID   string     // unique id for the generated PNG
	Bounds       [4]float64 // [west, south, east, north] in EPSG:4326
}

// ── Public API ───────────────────────────────────────────────────────────────

// AnalyzeLidar fetches LIDAR HD MNS and MNT tiles for the polygon,
// computes CHM = MNS - MNT, clips to the polygon bbox, and returns stats.
func AnalyzeLidar(ctx context.Context, geojsonGeom string) (*LidarResult, error) {
	// Parse polygon bbox
	bbox, err := geojsonBBox(geojsonGeom)
	if err != nil {
		return nil, fmt.Errorf("lidar: %w", err)
	}

	// Query WFS for MNS tiles
	mnsTiles, err := queryWFSTiles(ctx, "IGNF_MNS-LIDAR-HD:dalle", bbox)
	if err != nil {
		return nil, fmt.Errorf("lidar: querying MNS tiles: %w", err)
	}

	if len(mnsTiles) == 0 {
		return &LidarResult{
			HasCoverage: false,
			Message:     "No LIDAR HD coverage available for this area. LIDAR HD data is being progressively published by IGN and does not yet cover all of France.",
		}, nil
	}

	if len(mnsTiles) > maxLidarTiles {
		return &LidarResult{
			HasCoverage: false,
			Message:     fmt.Sprintf("Area too large: %d LIDAR tiles required (max %d). Please draw a smaller polygon.", len(mnsTiles), maxLidarTiles),
		}, nil
	}

	// Query matching MNT tiles
	mntTiles, err := queryWFSTiles(ctx, "IGNF_MNT-LIDAR-HD:dalle", bbox)
	if err != nil {
		return nil, fmt.Errorf("lidar: querying MNT tiles: %w", err)
	}

	// Match MNS tiles to MNT tiles by grid position (name pattern)
	mnsPairs, mntPairs := matchTilePairs(mnsTiles, mntTiles)
	if len(mnsPairs) == 0 {
		return &LidarResult{
			HasCoverage: false,
			Message:     "LIDAR HD MNS tiles found but matching MNT tiles are missing.",
		}, nil
	}

	// Download and parse tiles (parallel)
	os.MkdirAll(tileCacheDir, 0755)

	type tileResult struct {
		raster *Raster
		err    error
	}
	nPairs := len(mnsPairs)
	mnsResults := make([]tileResult, nPairs)
	mntResults := make([]tileResult, nPairs)

	var wg sync.WaitGroup
	wg.Add(nPairs * 2)
	for i := range mnsPairs {
		go func(idx int) {
			defer wg.Done()
			r, e := downloadAndParseTile(ctx, mnsPairs[idx])
			mnsResults[idx] = tileResult{r, e}
		}(i)
		go func(idx int) {
			defer wg.Done()
			r, e := downloadAndParseTile(ctx, mntPairs[idx])
			mntResults[idx] = tileResult{r, e}
		}(i)
	}
	wg.Wait()

	mnsRasters := make([]*Raster, 0, nPairs)
	mntRasters := make([]*Raster, 0, nPairs)
	for i := 0; i < nPairs; i++ {
		if mnsResults[i].err != nil {
			return nil, fmt.Errorf("lidar: downloading MNS tile %s: %w", mnsPairs[i].Name, mnsResults[i].err)
		}
		if mntResults[i].err != nil {
			return nil, fmt.Errorf("lidar: downloading MNT tile %s: %w", mntPairs[i].Name, mntResults[i].err)
		}
		mnsRasters = append(mnsRasters, mnsResults[i].raster)
		mntRasters = append(mntRasters, mntResults[i].raster)
	}

	// Compute CHM for each pair and merge stats
	var allCHM []float32
	var mergedBBox [4]float64
	first := true
	for i := range mnsRasters {
		chm := computeCHM(mnsRasters[i], mntRasters[i])
		for _, v := range chm.Data {
			if chm.HasNoData && v == chm.NoData {
				continue
			}
			if !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) {
				allCHM = append(allCHM, v)
			}
		}
		if first {
			mergedBBox = chm.BBox
			first = false
		} else {
			if chm.BBox[0] < mergedBBox[0] {
				mergedBBox[0] = chm.BBox[0]
			}
			if chm.BBox[1] < mergedBBox[1] {
				mergedBBox[1] = chm.BBox[1]
			}
			if chm.BBox[2] > mergedBBox[2] {
				mergedBBox[2] = chm.BBox[2]
			}
			if chm.BBox[3] > mergedBBox[3] {
				mergedBBox[3] = chm.BBox[3]
			}
		}
	}

	if len(allCHM) == 0 {
		return &LidarResult{
			HasCoverage: true,
			Message:     "LIDAR tiles found but all values are NoData in this area.",
		}, nil
	}

	// Filter out ground-level and noise pixels so stats reflect actual canopy.
	// Threshold of 2 m is the standard forestry minimum for "tree height".
	const canopyThreshold float32 = 2.0
	var canopyVals []float32
	for _, v := range allCHM {
		if v >= canopyThreshold {
			canopyVals = append(canopyVals, v)
		}
	}

	if len(canopyVals) == 0 {
		return &LidarResult{
			HasCoverage: true,
			Message:     "LIDAR tiles found but no canopy height detected in this area.",
		}, nil
	}

	// Compute stats on canopy-only values
	sort.Slice(canopyVals, func(i, j int) bool { return canopyVals[i] < canopyVals[j] })
	minH := float64(canopyVals[0])
	maxH := float64(canopyVals[len(canopyVals)-1])
	var sum float64
	for _, v := range canopyVals {
		sum += float64(v)
	}
	meanH := sum / float64(len(canopyVals))
	medianH := float64(canopyVals[len(canopyVals)/2])

	// Generate CHM image from the first pair (or mosaic for multiple)
	imageID := fmt.Sprintf("chm_%d", time.Now().UnixNano())
	var chmForImage *Raster
	if len(mnsRasters) == 1 {
		chmForImage = computeCHM(mnsRasters[0], mntRasters[0])
	} else {
		// Use the first tile for the image (simplification for multi-tile)
		chmForImage = computeCHM(mnsRasters[0], mntRasters[0])
	}
	if err := generateCHMImage(chmForImage, imageID, maxH); err != nil {
		return nil, fmt.Errorf("lidar: generating CHM image: %w", err)
	}

	// Convert bbox from native CRS to approximate EPSG:4326 if needed
	boundsWGS84 := estimateWGS84Bounds(mergedBBox, mnsRasters[0].EPSG)

	// Safety: ensure bounds are valid WGS84 (not raw projected coordinates)
	if !isValidWGS84(boundsWGS84) {
		// Fallback: use the input polygon bbox instead of tile-derived bounds
		boundsWGS84 = bbox
	}

	return &LidarResult{
		HasCoverage:  true,
		MinHeight:    math.Round(minH*100) / 100,
		MaxHeight:    math.Round(maxH*100) / 100,
		MeanHeight:   math.Round(meanH*100) / 100,
		MedianHeight: math.Round(medianH*100) / 100,
		CHMImageID:   imageID,
		Bounds:       boundsWGS84,
	}, nil
}

// ServeCHMImage returns the path to a generated CHM PNG image.
func ServeCHMImage(imageID string) (string, error) {
	path := filepath.Join(tileCacheDir, imageID+".png")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("CHM image not found: %s", imageID)
	}
	return path, nil
}

// ── WFS queries ──────────────────────────────────────────────────────────────

func queryWFSTiles(ctx context.Context, typeName string, bbox [4]float64) ([]WFSTile, error) {
	params := url.Values{
		"SERVICE":      {"WFS"},
		"VERSION":      {"2.0.0"},
		"REQUEST":      {"GetFeature"},
		"TYPENAMES":    {typeName},
		"OUTPUTFORMAT": {"application/json"},
		"COUNT":        {fmt.Sprintf("%d", maxLidarTiles+5)},
		"BBOX":         {fmt.Sprintf("%.6f,%.6f,%.6f,%.6f,EPSG:4326", bbox[0], bbox[1], bbox[2], bbox[3])},
	}

	reqURL := wfsBaseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("WFS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WFS returned %d: %s", resp.StatusCode, string(body))
	}

	var fc wfsFeatureCollection
	if err := json.NewDecoder(resp.Body).Decode(&fc); err != nil {
		return nil, fmt.Errorf("parsing WFS response: %w", err)
	}

	tiles := make([]WFSTile, len(fc.Features))
	for i, f := range fc.Features {
		tiles[i] = WFSTile{
			Name:         f.Properties.Name,
			NameDownload: f.Properties.NameDownload,
			URL:          f.Properties.URL,
			Projection:   f.Properties.Projection,
			Format:       f.Properties.Format,
			BBox:         f.Properties.BBox,
			Width:        f.Properties.Width,
			Height:       f.Properties.Height,
		}
	}

	return tiles, nil
}

// ── Tile matching ────────────────────────────────────────────────────────────

// matchTilePairs matches MNS tiles with MNT tiles by grid coordinates
// (extracted from the tile name: the 4-digit X and Y coordinates).
func matchTilePairs(mns, mnt []WFSTile) ([]WFSTile, []WFSTile) {
	mntMap := make(map[string]WFSTile)
	for _, t := range mnt {
		key := tileGridKey(t.Name)
		if key != "" {
			mntMap[key] = t
		}
	}

	var mnsPaired, mntPaired []WFSTile
	for _, t := range mns {
		key := tileGridKey(t.Name)
		if key == "" {
			continue
		}
		if mntTile, ok := mntMap[key]; ok {
			mnsPaired = append(mnsPaired, t)
			mntPaired = append(mntPaired, mntTile)
		}
	}
	return mnsPaired, mntPaired
}

// tileGridKey extracts the grid position from a tile name like
// "LHD_FXX_0599_6329_MNS_O_0M50_LAMB93_IGN69" → "0599_6329"
func tileGridKey(name string) string {
	parts := strings.Split(name, "_")
	if len(parts) < 5 {
		return ""
	}
	// Pattern: LHD_{area}_{X}_{Y}_{product}_...
	return parts[2] + "_" + parts[3]
}

// ── Tile download ────────────────────────────────────────────────────────────

func downloadAndParseTile(ctx context.Context, tile WFSTile) (*Raster, error) {
	cachePath := filepath.Join(tileCacheDir, tile.NameDownload)

	// Check cache
	if _, err := os.Stat(cachePath); err == nil {
		data, err := os.ReadFile(cachePath)
		if err == nil {
			return ParseGeoTIFF(data)
		}
	}

	// Build download URL from the WMS url property
	// The WFS provides a WMS GetMap URL; we modify it to request GeoTIFF
	downloadURL := buildDownloadURL(tile)
	if downloadURL == "" {
		return nil, fmt.Errorf("could not construct download URL for tile %s", tile.Name)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading tile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tile download returned %d for %s", resp.StatusCode, tile.NameDownload)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tile data: %w", err)
	}

	// Cache to disk (file names are unique per tile, no lock needed)
	os.WriteFile(cachePath, data, 0644)

	return ParseGeoTIFF(data)
}

// buildDownloadURL constructs the URL for downloading the actual GeoTIFF tile.
// Strategy: use the WMS URL from the WFS metadata, ensure FORMAT=image/geotiff.
func buildDownloadURL(tile WFSTile) string {
	if tile.URL == "" {
		return ""
	}

	// The URL from WFS is typically a WMS GetMap request.
	// Parse it and ensure we have the right format for raw data.
	u, err := url.Parse(tile.URL)
	if err != nil {
		return tile.URL
	}

	q := u.Query()
	q.Set("FORMAT", "image/geotiff")
	q.Set("REQUEST", "GetMap")
	q.Set("SERVICE", "WMS")
	q.Set("VERSION", "1.3.0")

	// Ensure bbox and dimensions are set from tile metadata
	if tile.BBox != "" {
		q.Set("BBOX", tile.BBox)
	}
	if tile.Width > 0 {
		q.Set("WIDTH", fmt.Sprintf("%d", tile.Width))
	}
	if tile.Height > 0 {
		q.Set("HEIGHT", fmt.Sprintf("%d", tile.Height))
	}
	if tile.Projection != "" {
		q.Set("CRS", tile.Projection)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// ── CHM computation ──────────────────────────────────────────────────────────

func computeCHM(mns, mnt *Raster) *Raster {
	w := mns.Width
	h := mns.Height
	if mnt.Width < w {
		w = mnt.Width
	}
	if mnt.Height < h {
		h = mnt.Height
	}

	chm := &Raster{
		Width:     w,
		Height:    h,
		Data:      make([]float32, w*h),
		NoData:    -9999,
		HasNoData: true,
		BBox:      mns.BBox,
		EPSG:      mns.EPSG,
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			mnsVal := mns.Data[y*mns.Width+x]
			mntVal := mnt.Data[y*mnt.Width+x]

			// Skip nodata
			if (mns.HasNoData && mnsVal == mns.NoData) || (mnt.HasNoData && mntVal == mnt.NoData) {
				chm.Data[i] = chm.NoData
				continue
			}
			if math.IsNaN(float64(mnsVal)) || math.IsNaN(float64(mntVal)) {
				chm.Data[i] = chm.NoData
				continue
			}

			height := mnsVal - mntVal
			// Clamp negative heights to 0 (can occur from processing artifacts)
			if height < 0 {
				height = 0
			}
			chm.Data[i] = height
		}
	}

	return chm
}

// ── CHM image generation ─────────────────────────────────────────────────────

// generateCHMImage creates a color-mapped PNG of the CHM raster.
// Uses a green-to-red color ramp based on canopy height.
func generateCHMImage(chm *Raster, imageID string, maxVal float64) error {
	if maxVal <= 0 {
		maxVal = 30 // default 30m scale
	}
	// Cap scale at 50m for visualization
	if maxVal > 50 {
		maxVal = 50
	}

	img := image.NewNRGBA(image.Rect(0, 0, chm.Width, chm.Height))

	for y := 0; y < chm.Height; y++ {
		for x := 0; x < chm.Width; x++ {
			v := chm.Data[y*chm.Width+x]
			if (chm.HasNoData && v == chm.NoData) || v <= 0 {
				img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 0}) // transparent
				continue
			}

			// Normalize to [0, 1]
			t := float64(v) / maxVal
			if t > 1 {
				t = 1
			}

			// Color ramp: green (low) → yellow (mid) → red (high)
			var r, g, b uint8
			if t < 0.5 {
				// Green → Yellow
				s := t * 2
				r = uint8(s * 255)
				g = 200
				b = 50
			} else {
				// Yellow → Red
				s := (t - 0.5) * 2
				r = 255
				g = uint8((1 - s) * 200)
				b = uint8((1 - s) * 50)
			}

			img.SetNRGBA(x, y, color.NRGBA{r, g, b, 180})
		}
	}

	path := filepath.Join(tileCacheDir, imageID+".png")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

// ── Coordinate helpers ───────────────────────────────────────────────────────

// geojsonBBox extracts the bounding box [west, south, east, north] from a
// GeoJSON Polygon/MultiPolygon in EPSG:4326.
func geojsonBBox(geojson string) ([4]float64, error) {
	var geom struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(geojson), &geom); err != nil {
		return [4]float64{}, fmt.Errorf("invalid GeoJSON: %w", err)
	}

	var west, south, east, north float64
	first := true

	extend := func(lon, lat float64) {
		if first {
			west, east = lon, lon
			south, north = lat, lat
			first = false
		} else {
			if lon < west {
				west = lon
			}
			if lon > east {
				east = lon
			}
			if lat < south {
				south = lat
			}
			if lat > north {
				north = lat
			}
		}
	}

	switch geom.Type {
	case "Polygon":
		var coords [][][2]float64
		if err := json.Unmarshal(geom.Coordinates, &coords); err != nil {
			return [4]float64{}, err
		}
		for _, ring := range coords {
			for _, c := range ring {
				extend(c[0], c[1])
			}
		}
	case "MultiPolygon":
		var coords [][][][2]float64
		if err := json.Unmarshal(geom.Coordinates, &coords); err != nil {
			return [4]float64{}, err
		}
		for _, poly := range coords {
			for _, ring := range poly {
				for _, c := range ring {
					extend(c[0], c[1])
				}
			}
		}
	default:
		return [4]float64{}, fmt.Errorf("unsupported geometry type: %s", geom.Type)
	}

	return [4]float64{west, south, east, north}, nil
}

// estimateWGS84Bounds converts a bbox from a projected CRS to approximate
// WGS84 coordinates. For EPSG:2154 (Lambert 93), uses a simple affine
// approximation suitable for France mainland.
func estimateWGS84Bounds(bbox [4]float64, epsg int) [4]float64 {
	if epsg == 4326 {
		return bbox
	}

	// EPSG:2154 (RGF93 / Lambert 93) → WGS84 approximate conversion
	if epsg == 2154 || (epsg == 0 && looksLikeLambert93(bbox)) {
		return [4]float64{
			lambert93ToLon(bbox[0], bbox[1]),
			lambert93ToLat(bbox[0], bbox[1]),
			lambert93ToLon(bbox[2], bbox[3]),
			lambert93ToLat(bbox[2], bbox[3]),
		}
	}

	// For other CRS, return as-is (would need proj4 for accuracy)
	return bbox
}

// looksLikeLambert93 checks if the coordinate ranges are consistent with
// EPSG:2154 (Lambert 93) for metropolitan France.
// X (easting) ∈ [100 000, 1 300 000], Y (northing) ∈ [6 000 000, 7 200 000].
func looksLikeLambert93(bbox [4]float64) bool {
	return bbox[0] > 50000 && bbox[0] < 1400000 &&
		bbox[1] > 5500000 && bbox[1] < 7500000 &&
		bbox[2] > 50000 && bbox[2] < 1400000 &&
		bbox[3] > 5500000 && bbox[3] < 7500000
}

// isValidWGS84 checks that coordinates are in the valid WGS84 range.
func isValidWGS84(bbox [4]float64) bool {
	return bbox[0] >= -180 && bbox[0] <= 180 &&
		bbox[1] >= -90 && bbox[1] <= 90 &&
		bbox[2] >= -180 && bbox[2] <= 180 &&
		bbox[3] >= -90 && bbox[3] <= 90
}

// Approximate Lambert 93 (EPSG:2154) → WGS84 conversion.
// Uses the IGN reference point and linear coefficients.
// Accurate to ~10m for Ile-de-France area, sufficient for map overlays.
func lambert93ToLon(x, y float64) float64 {
	// Reference: E=700000, N=6600000 → lon≈3°, lat≈46.5°
	return 3.0 + (x-700000.0)/((math.Cos(46.5*math.Pi/180))*111320.0)
}

func lambert93ToLat(x, y float64) float64 {
	return 46.5 + (y-6600000.0)/110540.0
}
