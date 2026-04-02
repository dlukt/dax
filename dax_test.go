package dax

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOpenRealFiles tests parsing actual Gold Box DAX files from the game data directory.
// Set DAX_TEST_DATA to the path containing original .DAX files.
func TestOpenRealFiles(t *testing.T) {
	testDir := os.Getenv("DAX_TEST_DATA")
	if testDir == "" {
		// Default: look relative to the pool-remake project
		testDir = filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")
		if _, err := os.Stat(testDir); os.IsNotExist(err) {
			t.Skip("no test data directory found, set DAX_TEST_DATA")
		}
	}

	files := []struct {
		name     string
		minRecs  int
		hasDecom bool
	}{
		{"geo1.dax", 1, true},
		{"pic1.dax", 1, true},
		{"title.dax", 1, true},
		{"sprit1.dax", 1, true},
		{"ecl1.dax", 1, true},
		{"walldef1.dax", 1, true},
		{"8x8d1.dax", 1, true},
		{"item1.dax", 1, true},
		{"head1.dax", 1, true},
		{"body1.dax", 1, true},
		{"mon1cha.dax", 1, true},
		{"mon1itm.dax", 1, true},
	}

	for _, tc := range files {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(testDir, tc.name)
			dax, err := Open(path)
			if err != nil {
				t.Fatalf("Open(%q): %v", tc.name, err)
			}

			if dax.RecordCount() < tc.minRecs {
				t.Errorf("expected >= %d records, got %d", tc.minRecs, dax.RecordCount())
			}

			if tc.hasDecom {
				// Try decompressing the first record
				entries := dax.Entries()
				if len(entries) == 0 {
					t.Fatal("no entries")
				}
				data := dax.Decode(entries[0].ID)
				if data == nil {
					t.Fatal("Decode returned nil")
				}

				expectedSize := int(entries[0].RawSize)
				if len(data) != expectedSize {
					t.Errorf("decompressed size: got %d, want %d", len(data), expectedSize)
				}
			}
		})
	}
}

// TestParseMinimal tests parsing a hand-crafted DAX container.
func TestParseMinimal(t *testing.T) {
	// Build a minimal DAX with one record containing compressed data.
	// One TOC entry = 9 bytes, so tocSize = 9.
	// Payload: compress the bytes [0x41, 0x42, 0x43] using the RLE scheme.
	// "2" (literal count=3) followed by ABC
	compressed := []byte{0x02, 0x41, 0x42, 0x43}

	data := []byte{
		0x09, 0x00, // tocSize = 9 (1 entry)
		// TOC entry: id=1, offset=0, rawSize=3, compSize=4
		0x01,
		0x00, 0x00, 0x00, 0x00, // offset
		0x03, 0x00, // rawSize
		0x04, 0x00, // compSize
	}
	data = append(data, compressed...)

	dax, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if dax.RecordCount() != 1 {
		t.Fatalf("expected 1 record, got %d", dax.RecordCount())
	}

	got := dax.Decode(0x01)
	want := []byte{0x41, 0x42, 0x43}
	if string(got) != string(want) {
		t.Errorf("Decode(1) = %v, want %v", got, want)
	}
}

// TestDecompressRLE tests the RLE decompression against known patterns.
func TestDecompressRLE(t *testing.T) {
	tests := []struct {
		name   string
		rawSz  int
		input  []byte
		expect []byte
	}{
		{
			"literal only",
			3,
			[]byte{0x02, 'A', 'B', 'C'},
			[]byte{'A', 'B', 'C'},
		},
		{
			"fill only",
			5,
			[]byte{0xFB, 'X'}, // -5 => fill 5 times
			[]byte{'X', 'X', 'X', 'X', 'X'},
		},
		{
			"mixed literal and fill",
			7,
			[]byte{
				0x02, 'A', 'B', 'C', // literal 3
				0xFD, 'Z', // fill 3
				0x00, 'Q', // literal 1
			},
			[]byte{'A', 'B', 'C', 'Z', 'Z', 'Z', 'Q'},
		},
		{
			"single byte literal",
			1,
			[]byte{0x00, 'Y'},
			[]byte{'Y'},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decompress(tc.rawSz, tc.input)
			if string(got) != string(tc.expect) {
				t.Errorf("got %v, want %v", got, tc.expect)
			}
		})
	}
}

// TestDecodeAll tests that all records in a file can be decompressed.
func TestDecodeAll(t *testing.T) {
	testDir := os.Getenv("DAX_TEST_DATA")
	if testDir == "" {
		testDir = filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")
		if _, err := os.Stat(testDir); os.IsNotExist(err) {
			t.Skip("no test data directory")
		}
	}

	// Test a medium-sized file
	path := filepath.Join(testDir, "geo1.dax")
	dax, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	all := dax.DecodeAll()
	if len(all) != dax.RecordCount() {
		t.Errorf("DecodeAll returned %d records, expected %d", len(all), dax.RecordCount())
	}

	for id, data := range all {
		if data == nil {
			t.Errorf("record 0x%02X: nil data", id)
		}
	}
}
