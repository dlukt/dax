package dax

// GeoMap represents a 16x16 map tile grid from a GEO*.DAX file.
//
// Each decompressed record is 1026 bytes: 2 header bytes + 1024 data bytes.
// The 1024 data bytes are organized as 4 pages of 256 bytes (16x16):
//
//	Page 0 (0x000-0x0FF): wall types for directions 0 (N) and 2 (E), packed as nibbles
//	Page 1 (0x100-0x1FF): wall types for directions 4 (S) and 6 (W), packed as nibbles
//	Page 2 (0x200-0x2FF): tile property byte x2
//	Page 3 (0x300-0x3FF): additional direction flags (2 bits each, 4 dirs packed in 1 byte)
type GeoMap struct {
	Header  byte
	Tiles   [16][16]Tile
}

// Direction constants for wall references.
const (
	DirNorth = 0 // direction 0
	DirEast  = 2 // direction 2
	DirSouth = 4 // direction 4
	DirWest  = 6 // direction 6
)

// Tile represents one cell in a 16x16 GeoMap.
type Tile struct {
	WallType [4]byte // wall type indices for directions 0,2,4,6
	Prop     byte    // tile property (x2 field)
	Flags    [4]byte // 2-bit direction flags for 0,2,4,6
}

// DecodeGeo decodes a GEO map record. Each record is 1026 bytes:
// 2 header bytes + 1024 data bytes representing a 16x16 grid.
func (f *File) DecodeGeo(id byte) *GeoMap {
	raw := f.Decode(id)
	if raw == nil || len(raw) < 1026 {
		return nil
	}

	geo := &GeoMap{
		Header: raw[0], // first byte of the 2-byte header
	}

	data := raw[2:] // skip 2-byte header, remaining is 1024 bytes

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			idx := y*16 + x
			tile := Tile{}

			// Page 0: high nibble = dir 0, low nibble = dir 2
			tile.WallType[0] = (data[idx] >> 4) & 0x0F
			tile.WallType[1] = data[idx] & 0x0F

			// Page 1 (0x100 offset): high nibble = dir 4, low nibble = dir 6
			tile.WallType[2] = (data[0x100+idx] >> 4) & 0x0F
			tile.WallType[3] = data[0x100+idx] & 0x0F

			// Page 2 (0x200 offset): property byte
			tile.Prop = data[0x200+idx]

			// Page 3 (0x300 offset): 2-bit flags packed in one byte
			// bits 0-1 = dir 0, bits 2-3 = dir 2, bits 4-5 = dir 4, bits 6-7 = dir 6
			b := data[0x300+idx]
			tile.Flags[0] = b & 0x03
			tile.Flags[1] = (b >> 2) & 0x03
			tile.Flags[2] = (b >> 4) & 0x03
			tile.Flags[3] = (b >> 6) & 0x03

			geo.Tiles[y][x] = tile
		}
	}

	return geo
}

// WallDefinition represents wall rendering lookup data from WALLDEF*.DAX.
// Each record contains 780 bytes organized as 5 rows of 156 entries.
// Multiple blocks can exist within one record (780 bytes each).
type WallDefinition struct {
	Data [5][156]byte
}

// DecodeWallDefs decodes wall definition records from a WALLDEF*.DAX file.
// Returns one WallDefinition per 780-byte block in the record.
func (f *File) DecodeWallDefs(id byte) []WallDefinition {
	raw := f.Decode(id)
	if raw == nil {
		return nil
	}

	blockSize := 780
	if len(raw)%blockSize != 0 {
		return nil
	}

	count := len(raw) / blockSize
	defs := make([]WallDefinition, count)

	for i := 0; i < count; i++ {
		off := i * blockSize
		for y := 0; y < 5; y++ {
			for x := 0; x < 156; x++ {
				defs[i].Data[y][x] = raw[off]
				off++
			}
		}
	}

	return defs
}
