package dax

import (
	"path/filepath"
	"testing"
)

func TestLoadCharacter(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	p, err := LoadCharacter(filepath.Join(dir, "chrdata1.sav"))
	if err != nil {
		t.Fatal(err)
	}

	name := PlayerName(p)
	if name == "" {
		t.Error("expected non-empty player name")
	}
	t.Logf("Name: %s", name)

	// Check stats were mapped (STR at Player.Raw 0x10-0x11)
	if p.Raw[0x10] == 0 {
		t.Error("expected non-zero STR stat")
	}
	t.Logf("STR: %d (cur=%d full=%d)", p.Raw[0x10], p.Raw[0x10], p.Raw[0x11])

	// Check race was mapped (Player.Raw 0x74)
	if p.Raw[0x74] == 0 {
		t.Error("expected non-zero race")
	}
	t.Logf("Race: %d, Class: %d", p.Raw[0x74], p.Raw[0x75])

	// Check HP (current at 0x1A4, max at 0x78)
	t.Logf("HP: %d/%d", p.Raw[0x1A4], p.Raw[0x78])

	// Check experience (int32 at 0x127)
	t.Logf("Exp: %d", int(p.Raw[0x127])|int(p.Raw[0x128])<<8|int(p.Raw[0x129])<<16|int(p.Raw[0x12A])<<24)

	// Check platinum (300 at offset 0x103)
	if v := p.wordAt(0x103); v != 300 {
		t.Errorf("platinum = %d, want 300", v)
	}
}

func TestLoadParty(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	gs, err := LoadParty(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(gs.Players) == 0 {
		t.Fatal("expected at least 1 player")
	}
	t.Logf("Loaded %d characters", len(gs.Players))

	for i, p := range gs.Players {
		name := PlayerName(p)
		t.Logf("  [%d] %s HP=%d/%d race=%d class=%d exp=%d items=%d affects=%d",
			i, name, p.Raw[0x1A4], p.Raw[0x78],
			p.Raw[0x74], p.Raw[0x75],
			int(p.Raw[0x127])|int(p.Raw[0x128])<<8|int(p.Raw[0x129])<<16|int(p.Raw[0x12A])<<24,
			len(p.items), len(p.affects))
	}
}

func TestLoadItems(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	items, err := LoadItems(filepath.Join(dir, "chrdata1.itm"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least 1 item")
	}
	t.Logf("Loaded %d items", len(items))
	for i, it := range items {
		t.Logf("  [%d] %q type=%d count=%d", i, it.Name, it.Type, it.Count)
	}
}

func TestLoadAffects(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	affects, err := LoadAffects(filepath.Join(dir, "chrdata1.spc"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Loaded %d affects", len(affects))
	for i, a := range affects {
		t.Logf("  [%d] type=%d minutes=%d data=%d", i, a.Type, a.Minutes, a.AffectData)
	}
}

func TestConvertPoolRadPlayerFieldMapping(t *testing.T) {
	// Build a test PoolRadPlayer with known values
	src := make([]byte, PoolRadPlayerSize)

	// Name: "TEST" (len=4, then chars)
	src[0] = 4
	copy(src[1:5], "TEST")

	// Stats
	src[0x10] = 18 // STR
	src[0x11] = 16 // INT
	src[0x12] = 14 // WIS
	src[0x13] = 12 // DEX
	src[0x14] = 15 // CON
	src[0x15] = 10 // CHA
	src[0x16] = 50 // STR00 (percentile)

	// THAC0
	src[0x2D] = 19

	// Race=1 (elf), Class=5 (magic-user)
	src[0x2E] = 1
	src[0x2F] = 5

	// Age = 120
	src[0x30] = 0x78
	src[0x31] = 0x00

	// HP max = 24
	src[0x32] = 24

	// Current HP = 22
	src[0x11B] = 22

	// Experience = 50000
	src[0xAC] = 0x50
	src[0xAD] = 0xC3
	src[0xAE] = 0x00
	src[0xAF] = 0x00

	// Class flags
	src[0xB0] = 0x20

	p := ConvertPoolRadPlayer(src)

	// Verify name
	if name := PlayerName(p); name != "TEST" {
		t.Errorf("name = %q, want TEST", name)
	}

	// Verify stats
	if p.Raw[0x10] != 18 || p.Raw[0x11] != 18 {
		t.Errorf("STR cur=%d full=%d, want 18/18", p.Raw[0x10], p.Raw[0x11])
	}
	if p.Raw[0x12] != 16 || p.Raw[0x13] != 16 {
		t.Errorf("INT cur=%d full=%d, want 16/16", p.Raw[0x12], p.Raw[0x13])
	}

	// Verify THAC0
	if p.Raw[0x73] != 19 {
		t.Errorf("THAC0 = %d, want 19", p.Raw[0x73])
	}

	// Verify race/class
	if p.Raw[0x74] != 1 {
		t.Errorf("race = %d, want 1", p.Raw[0x74])
	}
	if p.Raw[0x75] != 5 {
		t.Errorf("class = %d, want 5", p.Raw[0x75])
	}

	// Verify HP
	if p.Raw[0x78] != 24 {
		t.Errorf("HP max = %d, want 24", p.Raw[0x78])
	}
	if p.Raw[0x1A4] != 22 {
		t.Errorf("HP current = %d, want 22", p.Raw[0x1A4])
	}
	if p.HPCurrent != 22 {
		t.Errorf("HPCurrent = %d, want 22", p.HPCurrent)
	}

	// Verify experience
	if exp := p.wordAt(0x127); exp != 0xC350 {
		t.Errorf("exp low word = %04X, want C350", exp)
	}

	// Verify platinum
	if v := p.wordAt(0x103); v != 300 {
		t.Errorf("platinum = %d, want 300", v)
	}

	// Verify class flags
	if p.Raw[0x12B] != 0x20 {
		t.Errorf("classFlags = %02X, want 20", p.Raw[0x12B])
	}
}
