package dax

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestGameStateMemoryArea1(t *testing.T) {
	gs := new(GameState)
	// Write to Area1 region (0x4B00+)
	gs.SetVar(0x4B00, 42)
	if got := gs.GetVar(0x4B00); got != 42 {
		t.Errorf("Area1 GetVar(0x4B00) = %d, want 42", got)
	}
}

func TestGameStateMemoryStruct(t *testing.T) {
	gs := new(GameState)
	// Write to Struct region (0x7A00+)
	gs.SetVar(0x7A10, 100)
	if got := gs.GetVar(0x7A10); got != 100 {
		t.Errorf("Struct GetVar(0x7A10) = %d, want 100", got)
	}
}

func TestGameStateGlobals(t *testing.T) {
	gs := new(GameState)
	gs.SetVar(0xFB, 10) // MapX
	gs.SetVar(0xFC, 20) // MapY
	gs.SetVar(0x3DE, 2) // MapDir

	if gs.MapX != 10 {
		t.Errorf("MapX = %d, want 10", gs.MapX)
	}
	if gs.MapY != 20 {
		t.Errorf("MapY = %d, want 20", gs.MapY)
	}
	if gs.MapDir != 2 {
		t.Errorf("MapDir = %d, want 2", gs.MapDir)
	}
}

func TestGameStatePlayerFieldAccess(t *testing.T) {
	gs := new(GameState)
	p := new(Player)
	p.Raw[0x127] = 0x34 // exp low byte
	p.Raw[0x128] = 0x12 // exp high byte
	p.Raw[0xD6] = 1     // sex = male
	p.Raw[0x143] = 5    // icon ID
	gs.Players = append(gs.Players, p)
	gs.SelectedIdx = 0

	// Read player fields through Area2 address space
	if v := gs.GetVar(0x7C00 + 0x127); v != 0x1234 {
		t.Errorf("player exp_low = %04X, want 1234", v)
	}
	if v := gs.GetVar(0x7C00 + 0xD6); v != 1 {
		t.Errorf("player sex = %d, want 1", v)
	}
	if v := gs.GetVar(0x7C00 + 0x143); v != 5 {
		t.Errorf("player icon = %d, want 5", v)
	}

	// Write to player field
	gs.SetVar(0x7C00+0xD6, 2) // set sex = female
	if p.Raw[0xD6] != 2 {
		t.Errorf("player sex after write = %d, want 2", p.Raw[0xD6])
	}
}

func TestGameStateGameArea(t *testing.T) {
	gs := new(GameState)
	gs.SetVar(0x7C00+0x312, 5) // game area
	if gs.GameArea != 5 {
		t.Errorf("GameArea = %d, want 5", gs.GameArea)
	}
}

func TestGameStateNoPlayer(t *testing.T) {
	gs := new(GameState)
	// With no players, Area2 access should read raw bytes
	gs.Area2[0x10] = 0xAB
	gs.Area2[0x11] = 0xCD
	v := gs.GetVar(0x7C00 + 0x10)
	if v != 0xCDAB {
		t.Errorf("raw Area2 read = %04X, want CDAB", v)
	}
}

func TestDriverLoadArea(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	f, err := Open(filepath.Join(dir, "ecl1.dax"))
	if err != nil {
		t.Fatal(err)
	}

	entries := f.Entries()
	if len(entries) == 0 {
		t.Fatal("no entries")
	}

	gs := new(GameState)
	d := NewDriver(gs, f)

	// Should not panic when loading an ECL block
	d.LoadArea(entries[0].ID)

	if d.BlockID != entries[0].ID {
		t.Errorf("BlockID = %d, want %d", d.BlockID, entries[0].ID)
	}
}

func TestDriverAllECLFiles(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	for i := 1; i <= 8; i++ {
		name := filepath.Join(dir, fmt.Sprintf("ecl%d.dax", i))
		t.Run(fmt.Sprintf("ecl%d", i), func(t *testing.T) {
			f, err := Open(name)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}

			for _, e := range f.Entries() {
				gs := new(GameState)
				d := NewDriver(gs, f)
				d.LoadArea(e.ID) // should not panic
			}
		})
	}
}
