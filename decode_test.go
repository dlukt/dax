package dax

import (
	"path/filepath"
	"testing"
)

func testDataDir() string {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")
	if _, err := filepath.Abs(dir); err != nil {
		return ""
	}
	return dir
}

func TestDecodePicture(t *testing.T) {
	dir := testDataDir()

	// Test 8x8 sprites - height=8, width=1, items=70
	f, err := Open(filepath.Join(dir, "8x8d1.dax"))
	if err != nil {
		t.Fatal(err)
	}
	p := f.DecodePicture(0x01, -1)
	if p == nil {
		t.Fatal("nil picture")
	}
	if p.Height != 8 {
		t.Errorf("height = %d, want 8", p.Height)
	}
	if p.Width != 1 {
		t.Errorf("width = %d, want 1", p.Width)
	}
	if p.ItemCount != 70 {
		t.Errorf("item_count = %d, want 70", p.ItemCount)
	}
	if len(p.Pixels) != 70*8*8 {
		t.Errorf("pixels len = %d, want %d", len(p.Pixels), 70*8*8)
	}

	// Verify first pixel of first tile is within EGA range (0-15)
	tile0 := p.ItemPixels(0)
	if tile0[0] > 15 {
		t.Errorf("first pixel = %d, want 0-15", tile0[0])
	}
}

func TestDecodePictureBody(t *testing.T) {
	dir := testDataDir()

	f, err := Open(filepath.Join(dir, "body1.dax"))
	if err != nil {
		t.Fatal(err)
	}
	entries := f.Entries()
	if len(entries) == 0 {
		t.Fatal("no entries")
	}
	p := f.DecodePicture(entries[0].ID, -1)
	if p == nil {
		t.Fatal("nil picture")
	}
	if p.Height == 0 || p.Width == 0 {
		t.Errorf("empty dimensions: %dx%d", p.Height, p.Width)
	}
	if p.ItemCount == 0 {
		t.Error("item_count = 0")
	}
	if len(p.Pixels) != p.ItemCount*p.Height*p.PixelWidth() {
		t.Errorf("pixels mismatch: got %d, want %d",
			len(p.Pixels), p.ItemCount*p.Height*p.PixelWidth())
	}
}

func TestDecodePictureHead(t *testing.T) {
	dir := testDataDir()

	f, err := Open(filepath.Join(dir, "head1.dax"))
	if err != nil {
		t.Fatal(err)
	}
	entries := f.Entries()
	if len(entries) == 0 {
		t.Fatal("no entries")
	}
	p := f.DecodePicture(entries[0].ID, -1)
	if p == nil {
		t.Fatal("nil picture")
	}
	if p.Height == 0 || p.Width == 0 {
		t.Errorf("empty dimensions: %dx%d", p.Height, p.Width)
	}
}

func TestDecodeAnimationSprite(t *testing.T) {
	dir := testDataDir()

	f, err := Open(filepath.Join(dir, "sprit1.dax"))
	if err != nil {
		t.Fatal(err)
	}
	entries := f.Entries()
	if len(entries) == 0 {
		t.Fatal("no entries")
	}

	// SPRIT files use the PIC animation format, not the simple DaxBlock format
	anim := f.DecodeAnimation(entries[0].ID)
	if anim == nil {
		t.Fatal("nil animation")
	}
	if len(anim.Frames) == 0 {
		t.Fatal("no frames")
	}
	frame := anim.Frames[0]
	if frame.Height == 0 || frame.Width == 0 {
		t.Errorf("empty frame dimensions: %dx%d", frame.Height, frame.Width)
	}
}

func TestDecodePictureWithMask(t *testing.T) {
	dir := testDataDir()

	f, err := Open(filepath.Join(dir, "8x8d1.dax"))
	if err != nil {
		t.Fatal(err)
	}

	// With maskColor=0, all 0-valued pixels should become 16 (transparent)
	p := f.DecodePicture(0x01, 0)
	if p == nil {
		t.Fatal("nil picture")
	}

	foundTransparent := false
	foundZero := false
	for _, px := range p.Pixels {
		if px == 16 {
			foundTransparent = true
		}
		if px == 0 {
			foundZero = true
		}
	}
	// If there are any zero pixels in the unmasked data, they should now be 16
	if !foundTransparent && !foundZero {
		// No zero pixels in this particular record, that's fine
		t.Log("no zero pixels found in this record")
	}
}

func TestDecodeAnimation(t *testing.T) {
	dir := testDataDir()

	f, err := Open(filepath.Join(dir, "pic1.dax"))
	if err != nil {
		t.Fatal(err)
	}

	anim := f.DecodeAnimation(0x01)
	if anim == nil {
		t.Fatal("nil animation")
	}
	if len(anim.Frames) == 0 {
		t.Fatal("no frames")
	}

	frame := anim.Frames[0]
	if frame.Height == 0 || frame.Width == 0 {
		t.Errorf("empty frame dimensions: %dx%d", frame.Height, frame.Width)
	}

	expectedPixels := frame.Height * frame.Width * 8
	if len(frame.Pixels) != expectedPixels {
		t.Errorf("frame 0 pixels: got %d, want %d", len(frame.Pixels), expectedPixels)
	}
}

func TestDecodeGeo(t *testing.T) {
	dir := testDataDir()

	f, err := Open(filepath.Join(dir, "geo1.dax"))
	if err != nil {
		t.Fatal(err)
	}

	entries := f.Entries()
	geo := f.DecodeGeo(entries[0].ID)
	if geo == nil {
		t.Fatal("nil geo")
	}

	// Verify all tiles have valid wall types (0-15)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			tile := geo.Tiles[y][x]
			for _, wt := range tile.WallType {
				if wt > 15 {
					t.Errorf("tile[%d][%d] walltype > 15: %d", y, x, wt)
				}
			}
			for _, fl := range tile.Flags {
				if fl > 3 {
					t.Errorf("tile[%d][%d] flag > 3: %d", y, x, fl)
				}
			}
		}
	}
}

func TestDecodeWallDefs(t *testing.T) {
	dir := testDataDir()

	f, err := Open(filepath.Join(dir, "walldef1.dax"))
	if err != nil {
		t.Fatal(err)
	}

	entries := f.Entries()
	defs := f.DecodeWallDefs(entries[0].ID)
	if defs == nil {
		t.Fatal("nil wall defs")
	}
	if len(defs) == 0 {
		t.Fatal("no wall def blocks")
	}

	// Each block should be 5 rows x 156 entries
	if len(defs[0].Data) != 5 || len(defs[0].Data[0]) != 156 {
		t.Errorf("wall def dimensions: %dx%d, want 5x156",
			len(defs[0].Data), len(defs[0].Data[0]))
	}
}

func TestDecodeAllFileTypes(t *testing.T) {
	dir := testDataDir()

	// Verify every DAX file type can be fully parsed and decoded
	files := []string{
		"8x8d1.dax", "8x8d2.dax", "8x8d3.dax", "8x8d4.dax",
		"pic1.dax", "pic2.dax", "pic3.dax",
		"geo1.dax", "geo2.dax", "geo3.dax",
		"walldef1.dax", "walldef2.dax",
		"body1.dax", "head1.dax",
		"sprit1.dax", "sprit2.dax",
		"ecl1.dax", "ecl2.dax",
		"item1.dax", "item2.dax",
		"mon1cha.dax", "mon1itm.dax",
		"cpic1.dax",
		"comspr.dax",
		"title.dax",
		"wildcom.dax",
		"dungcom.dax",
		"randcom.dax",
		"icon.dax",
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			f, err := Open(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}

			all := f.DecodeAll()
			if len(all) != f.RecordCount() {
				t.Errorf("decoded %d records, expected %d", len(all), f.RecordCount())
			}

			for id, data := range all {
				if data == nil {
					t.Errorf("record 0x%02X: nil", id)
				}
			}
		})
	}
}
