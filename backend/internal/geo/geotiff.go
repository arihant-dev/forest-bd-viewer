package geo

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Raster holds a 2D grid of float32 elevation values parsed from a GeoTIFF.
type Raster struct {
	Width, Height int
	Data          []float32
	NoData        float32
	HasNoData     bool
	// BBox in the native CRS: [xmin, ymin, xmax, ymax]
	BBox [4]float64
	EPSG int
}

// TIFF tag IDs we care about
const (
	tagImageWidth      = 256
	tagImageLength     = 257
	tagBitsPerSample   = 258
	tagCompression     = 259
	tagStripOffsets    = 273
	tagSamplesPerPixel = 277
	tagRowsPerStrip    = 278
	tagStripByteCounts = 279
	tagTileWidth       = 322
	tagTileLength      = 323
	tagTileOffsets     = 324
	tagTileByteCounts  = 325
	tagSampleFormat    = 339
	tagModelPixelScale = 33550
	tagModelTiepoint   = 33922
	tagGeoKeyDirectory = 34735
	tagGDALNoData      = 42113
)

// TIFF data types
const (
	tiffByte   = 1
	tiffASCII  = 2
	tiffShort  = 3
	tiffLong   = 4
	tiffFloat  = 11
	tiffDouble = 12
)

// ParseGeoTIFF reads a float32 GeoTIFF from raw bytes and returns a Raster.
// Supports uncompressed and DEFLATE-compressed strip/tile-organized TIFFs.
func ParseGeoTIFF(data []byte) (*Raster, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("geotiff: data too short")
	}

	// Byte order
	var bo binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return nil, fmt.Errorf("geotiff: invalid byte order marker")
	}

	magic := bo.Uint16(data[2:4])
	if magic != 42 {
		return nil, fmt.Errorf("geotiff: not a TIFF file (magic=%d)", magic)
	}

	ifdOffset := bo.Uint32(data[4:8])
	return parseIFD(data, bo, ifdOffset)
}

type ifdEntry struct {
	tag    uint16
	dtype  uint16
	count  uint32
	valOff uint32
}

func parseIFD(data []byte, bo binary.ByteOrder, offset uint32) (*Raster, error) {
	if int(offset)+2 > len(data) {
		return nil, fmt.Errorf("geotiff: IFD offset out of range")
	}

	numEntries := int(bo.Uint16(data[offset:]))
	entries := make([]ifdEntry, numEntries)

	pos := int(offset) + 2
	for i := 0; i < numEntries; i++ {
		if pos+12 > len(data) {
			return nil, fmt.Errorf("geotiff: truncated IFD entry")
		}
		e := ifdEntry{
			tag:    bo.Uint16(data[pos:]),
			dtype:  bo.Uint16(data[pos+2:]),
			count:  bo.Uint32(data[pos+4:]),
			valOff: bo.Uint32(data[pos+8:]),
		}
		entries[i] = e
		pos += 12
	}

	getEntry := func(tag uint16) *ifdEntry {
		for i := range entries {
			if entries[i].tag == tag {
				return &entries[i]
			}
		}
		return nil
	}

	getUint32 := func(e *ifdEntry) uint32 {
		if e == nil {
			return 0
		}
		if e.dtype == tiffShort {
			return uint32(bo.Uint16(data[int(e.valOff):]))
		}
		// For types that fit in 4 bytes, value is stored in valOff directly
		if typeSize(e.dtype)*int(e.count) <= 4 {
			if e.dtype == tiffShort {
				// value in the valOff field bytes
				buf := make([]byte, 4)
				bo.PutUint32(buf, e.valOff)
				return uint32(bo.Uint16(buf))
			}
			return e.valOff
		}
		return e.valOff
	}

	getUint32Value := func(tag uint16) uint32 {
		e := getEntry(tag)
		if e == nil {
			return 0
		}
		// If data fits in 4 bytes, it's stored inline in valOff
		sz := typeSize(e.dtype) * int(e.count)
		if sz <= 4 {
			if e.dtype == tiffShort && e.count == 1 {
				buf := make([]byte, 4)
				bo.PutUint32(buf, e.valOff)
				return uint32(bo.Uint16(buf))
			}
			return e.valOff
		}
		// Otherwise valOff is an offset into the file
		off := e.valOff
		if e.dtype == tiffLong {
			return bo.Uint32(data[off:])
		}
		if e.dtype == tiffShort {
			return uint32(bo.Uint16(data[off:]))
		}
		return e.valOff
	}
	_ = getUint32

	readUint32Array := func(e *ifdEntry) []uint32 {
		if e == nil {
			return nil
		}
		n := int(e.count)
		arr := make([]uint32, n)
		sz := typeSize(e.dtype) * n
		var src []byte
		if sz <= 4 {
			buf := make([]byte, 4)
			bo.PutUint32(buf, e.valOff)
			src = buf
		} else {
			off := int(e.valOff)
			if off+sz > len(data) {
				return nil
			}
			src = data[off:]
		}
		for i := 0; i < n; i++ {
			if e.dtype == tiffShort {
				arr[i] = uint32(bo.Uint16(src[i*2:]))
			} else {
				arr[i] = bo.Uint32(src[i*4:])
			}
		}
		return arr
	}

	readFloat64Array := func(e *ifdEntry) []float64 {
		if e == nil {
			return nil
		}
		n := int(e.count)
		off := int(e.valOff)
		if off+n*8 > len(data) {
			return nil
		}
		arr := make([]float64, n)
		for i := 0; i < n; i++ {
			arr[i] = math.Float64frombits(bo.Uint64(data[off+i*8:]))
		}
		return arr
	}

	width := int(getUint32Value(tagImageWidth))
	height := int(getUint32Value(tagImageLength))
	compression := getUint32Value(tagCompression)
	bitsPerSample := getUint32Value(tagBitsPerSample)
	sampleFormat := getUint32Value(tagSampleFormat)
	if sampleFormat == 0 {
		sampleFormat = 1 // default unsigned int
	}

	if width == 0 || height == 0 {
		return nil, fmt.Errorf("geotiff: zero image dimensions")
	}
	if bitsPerSample != 32 {
		return nil, fmt.Errorf("geotiff: expected 32 bits/sample, got %d", bitsPerSample)
	}
	if sampleFormat != 3 {
		return nil, fmt.Errorf("geotiff: expected float sample format (3), got %d", sampleFormat)
	}

	// Read NoData
	var noData float32
	var hasNoData bool
	if e := getEntry(tagGDALNoData); e != nil {
		off := int(e.valOff)
		end := off
		for end < len(data) && data[end] != 0 {
			end++
		}
		if end > off {
			s := string(data[off:end])
			var f float64
			if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
				noData = float32(f)
				hasNoData = true
			}
		}
	}

	// Determine if tiled or stripped
	tileWidthEntry := getEntry(tagTileWidth)
	isTiled := tileWidthEntry != nil

	pixels := make([]float32, width*height)
	// Initialize with nodata
	if hasNoData {
		for i := range pixels {
			pixels[i] = noData
		}
	}

	if isTiled {
		tw := int(getUint32Value(tagTileWidth))
		th := int(getUint32Value(tagTileLength))
		offsets := readUint32Array(getEntry(tagTileOffsets))
		byteCounts := readUint32Array(getEntry(tagTileByteCounts))

		if len(offsets) == 0 {
			return nil, fmt.Errorf("geotiff: no tile offsets")
		}

		tilesX := (width + tw - 1) / tw
		tilesY := (height + th - 1) / th

		for ty := 0; ty < tilesY; ty++ {
			for tx := 0; tx < tilesX; tx++ {
				idx := ty*tilesX + tx
				if idx >= len(offsets) {
					break
				}
				raw, err := decompressChunk(data, offsets[idx], byteCounts[idx], compression)
				if err != nil {
					return nil, fmt.Errorf("geotiff: tile (%d,%d): %w", tx, ty, err)
				}
				writeTileToPixels(raw, pixels, bo, tx*tw, ty*th, tw, th, width, height)
			}
		}
	} else {
		// Strip-based
		rowsPerStrip := int(getUint32Value(tagRowsPerStrip))
		if rowsPerStrip == 0 {
			rowsPerStrip = height
		}
		offsets := readUint32Array(getEntry(tagStripOffsets))
		byteCounts := readUint32Array(getEntry(tagStripByteCounts))

		if len(offsets) == 0 {
			return nil, fmt.Errorf("geotiff: no strip offsets")
		}

		y := 0
		for i, off := range offsets {
			bc := uint32(0)
			if i < len(byteCounts) {
				bc = byteCounts[i]
			}
			raw, err := decompressChunk(data, off, bc, compression)
			if err != nil {
				return nil, fmt.Errorf("geotiff: strip %d: %w", i, err)
			}
			rows := rowsPerStrip
			if y+rows > height {
				rows = height - y
			}
			n := rows * width
			if len(raw) < n*4 {
				n = len(raw) / 4
			}
			for j := 0; j < n; j++ {
				pixels[y*width+j] = math.Float32frombits(bo.Uint32(raw[j*4:]))
			}
			y += rows
		}
	}

	// Parse geo-referencing
	r := &Raster{
		Width:     width,
		Height:    height,
		Data:      pixels,
		NoData:    noData,
		HasNoData: hasNoData,
	}

	// ModelPixelScaleTag + ModelTiepointTag → BBox
	scales := readFloat64Array(getEntry(tagModelPixelScale))
	tiepoints := readFloat64Array(getEntry(tagModelTiepoint))
	if len(scales) >= 2 && len(tiepoints) >= 6 {
		scaleX := scales[0]
		scaleY := scales[1]
		tieI := tiepoints[0]
		tieJ := tiepoints[1]
		tieX := tiepoints[3]
		tieY := tiepoints[4]

		xMin := tieX - tieI*scaleX
		yMax := tieY + tieJ*scaleY
		xMax := xMin + float64(width)*scaleX
		yMin := yMax - float64(height)*scaleY

		r.BBox = [4]float64{xMin, yMin, xMax, yMax}
	}

	// Try to extract EPSG from GeoKeyDirectory
	if e := getEntry(tagGeoKeyDirectory); e != nil {
		keys := readUint32Array(e)
		// GeoKeyDirectory: [keyDirVersion, keyRevision, minorRevision, numberOfKeys, ...]
		// Then groups of 4: [keyID, TIFFTagLocation, count, valueOffset]
		if len(keys) > 4 {
			nKeys := int(keys[3])
			for k := 0; k < nKeys && 4+k*4+3 < len(keys); k++ {
				keyID := keys[4+k*4]
				loc := keys[4+k*4+1]
				val := keys[4+k*4+3]
				// ProjectedCSTypeGeoKey = 3072
				if keyID == 3072 && loc == 0 {
					r.EPSG = int(val)
				}
				// GeographicTypeGeoKey = 2048 (fallback)
				if keyID == 2048 && loc == 0 && r.EPSG == 0 {
					r.EPSG = int(val)
				}
			}
		}
	}

	return r, nil
}

func typeSize(dtype uint16) int {
	switch dtype {
	case tiffByte, tiffASCII:
		return 1
	case tiffShort:
		return 2
	case tiffLong, tiffFloat:
		return 4
	case tiffDouble:
		return 8
	default:
		return 1
	}
}

func decompressChunk(data []byte, offset, byteCount, compression uint32) ([]byte, error) {
	off := int(offset)
	bc := int(byteCount)
	if off+bc > len(data) {
		return nil, fmt.Errorf("chunk out of bounds (off=%d bc=%d len=%d)", off, bc, len(data))
	}
	chunk := data[off : off+bc]

	switch compression {
	case 1: // None
		return chunk, nil
	case 8, 32946: // DEFLATE / new-style DEFLATE
		r, err := zlib.NewReader(bytes.NewReader(chunk))
		if err != nil {
			return nil, fmt.Errorf("zlib init: %w", err)
		}
		defer r.Close()
		return io.ReadAll(r)
	default:
		return nil, fmt.Errorf("unsupported compression type %d", compression)
	}
}

func writeTileToPixels(raw []byte, pixels []float32, bo binary.ByteOrder, startX, startY, tw, th, imgW, imgH int) {
	for row := 0; row < th; row++ {
		y := startY + row
		if y >= imgH {
			break
		}
		for col := 0; col < tw; col++ {
			x := startX + col
			if x >= imgW {
				continue
			}
			idx := row*tw + col
			if idx*4+4 > len(raw) {
				continue
			}
			pixels[y*imgW+x] = math.Float32frombits(bo.Uint32(raw[idx*4:]))
		}
	}
}
