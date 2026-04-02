package dax

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestGameStateMemoryArea1(t *testing.T) {
	gs := new(GameState)
	gs.SetVar(0x4B00, 42)
	if got := gs.GetVar(0x4B00); got != 42 {
		t.Errorf("Area1 GetVar(0x4B00) = %d, want 42", got)
	}
}

func TestGameStateMemoryStruct(t *testing.T) {
	gs := new(GameState)
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

	if v := gs.GetVar(0x7C00 + 0x127); v != 0x1234 {
		t.Errorf("player exp = %04X, want 1234", v)
	}
	if v := gs.GetVar(0x7C00 + 0xD6); v != 1 {
		t.Errorf("player sex = %d, want 1", v)
	}
	if v := gs.GetVar(0x7C00 + 0x143); v != 5 {
		t.Errorf("player icon = %d, want 5", v)
	}

	gs.SetVar(0x7C00+0xD6, 2)
	if p.Raw[0xD6] != 2 {
		t.Errorf("player sex after write = %d, want 2", p.Raw[0xD6])
	}
}

func TestGameStateGameArea(t *testing.T) {
	gs := new(GameState)
	gs.SetVar(0x7C00+0x312, 5)
	if gs.GameArea != 5 {
		t.Errorf("GameArea = %d, want 5", gs.GameArea)
	}
}

func TestGameStateNoPlayer(t *testing.T) {
	gs := new(GameState)
	gs.Area2[0x10] = 0xAB
	gs.Area2[0x11] = 0xCD
	v := gs.GetVar(0x7C00 + 0x10)
	if v != 0xCDAB {
		t.Errorf("raw Area2 read = %04X, want CDAB", v)
	}
}

func TestGameStateMapPosition(t *testing.T) {
	gs := new(GameState)
	gs.SetVar(0xC04B, 5)  // posX
	gs.SetVar(0xC04C, 10) // posY
	gs.SetVar(0xC04D, 2)  // direction=2 -> engine dir=4 (south)

	if gs.MapX != 5 {
		t.Errorf("MapX = %d, want 5", gs.MapX)
	}
	if gs.MapY != 10 {
		t.Errorf("MapY = %d, want 10", gs.MapY)
	}
	if gs.MapDir != 4 {
		t.Errorf("MapDir = %d, want 4", gs.MapDir)
	}
	if !gs.PosChanged {
		t.Error("PosChanged should be true")
	}

	if v := gs.GetVar(0xC04B); v != 5 {
		t.Errorf("read MapX = %d, want 5", v)
	}
	if v := gs.GetVar(0xC04C); v != 10 {
		t.Errorf("read MapY = %d, want 10", v)
	}
	if v := gs.GetVar(0xC04D); v != 2 {
		t.Errorf("read MapDir/2 = %d, want 2", v)
	}
	if v := gs.GetVar(0xFB); v != 5 {
		t.Errorf("read 0xFB = %d, want 5", v)
	}
	if v := gs.GetVar(0xFC); v != 10 {
		t.Errorf("read 0xFC = %d, want 10", v)
	}
	if v := gs.GetVar(0x3DE); v != 4 {
		t.Errorf("read 0x3DE = %d, want 4", v)
	}
}

func TestGameStateDirectionMapping(t *testing.T) {
	gs := new(GameState)
	for vmDir, engineDir := range map[int]int{0: 0, 1: 2, 2: 4, 3: 6} {
		gs.SetVar(0xC04D, uint16(vmDir))
		if gs.MapDir != engineDir {
			t.Errorf("vmDir=%d -> MapDir=%d, want %d", vmDir, gs.MapDir, engineDir)
		}
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
	d.LoadArea(entries[0].ID)

	if d.BlockID != entries[0].ID {
		t.Errorf("BlockID = %d, want %d", d.BlockID, entries[0].ID)
	}
}

func TestECLHeaderParsing(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	for i := 1; i <= 8; i++ {
		name := filepath.Join(dir, fmt.Sprintf("ecl%d.dax", i))
		t.Run(fmt.Sprintf("ecl%d", i), func(t *testing.T) {
			f, err := Open(name)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}

			for _, e := range f.Entries() {
				data := f.Decode(e.ID)
				if data == nil {
					continue
				}

				hdr, _ := DisassembleECL(data)
				codeSize := uint16(len(data) - 2)

				checkAddr := func(name string, addr uint16) {
					if addr < 0x8000 || addr >= 0x8000+codeSize {
						t.Errorf("block 0x%02X: %s %04X out of range [0x8000, 0x8000+%d)",
							e.ID, name, addr, codeSize)
					}
				}
				checkAddr("RunAddr", hdr.RunAddr)
				checkAddr("SearchLocation", hdr.SearchLocation)
				checkAddr("PreCampCheck", hdr.PreCampCheck)
				checkAddr("CampInterrupted", hdr.CampInterrupted)
				checkAddr("InitialEntry", hdr.InitialEntry)
			}
		})
	}
}
