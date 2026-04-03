package dax

import (
	"testing"
)

func TestGameStatePosition(t *testing.T) {
	gs := new(GameState)
	gs.SetVar(0xFB, 10) // MapX (low global)
	gs.SetVar(0xFC, 20) // MapY (low global)
	gs.SetVar(0x3DE, 4) // MapDir directly (low global stores engine value)

	if v := gs.GetVar(0xFB); v != 10 {
		t.Errorf("GetVar(0xFB) = %d, want 10", v)
	}
	if v := gs.GetVar(0xFC); v != 20 {
		t.Errorf("GetVar(0xFC) = %d, want 20", v)
	}
	if v := gs.GetVar(0x3DE); v != 4 {
		t.Errorf("GetVar(0x3DE) = %d, want 4", v)
	}

	// High globals (0xC04B+) do set PosChanged
	gs.SetVar(0xC04B, 5)
	gs.SetVar(0xC04C, 6)
	gs.SetVar(0xC04D, 2) // VM dir 2 -> engine dir 4
	if !gs.PosChanged {
		t.Error("PosChanged should be true after high global write")
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
	gs.SetVar(0xC04D, 2)  // direction=2 -> engine dir=4

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

func TestGameStateWallRoofType(t *testing.T) {
	gs := new(GameState)
	if gs.WallRoof != 0 {
		t.Error("WallRoof should start 0")
	}
	gs.SetVar(0xC04F, 0x42)
	if gs.WallRoof != 0x42 {
		t.Errorf("WallRoof = %d, want 0x42", gs.WallRoof)
	}
	if v := gs.GetVar(0xC04F); v != 0x42 {
		t.Errorf("GetVar(0xC04F) = %d, want 0x42", v)
	}

	gs.SetVar(0xC04E, 7)
	if gs.WallType != 7 {
		t.Errorf("WallType = %d, want 7", gs.WallType)
	}
	if v := gs.GetVar(0xC04E); v != 7 {
		t.Errorf("GetVar(0xC04E) = %d, want 7", v)
	}
}

func TestGameStateUpdateWallInfo(t *testing.T) {
	gs := new(GameState)
	gs.MapX = 5
	gs.MapY = 10
	gs.MapDir = 4 // engine dir 4 = south (index 2)

	geo := new(GeoMap)
	geo.Tiles[10][5].Prop = 0x42
	geo.Tiles[10][5].WallType[0] = 1 // north
	geo.Tiles[10][5].WallType[1] = 3 // east
	geo.Tiles[10][5].WallType[2] = 7 // south
	geo.Tiles[10][5].WallType[3] = 9 // west

	gs.UpdateWallInfo(geo)

	if gs.WallRoof != 0x42 {
		t.Errorf("WallRoof = %d, want 0x42", gs.WallRoof)
	}
	if gs.WallType != 7 { // facing south = WallType[2]
		t.Errorf("WallType = %d, want 7", gs.WallType)
	}
}

func TestGameStateUpdateWallInfoNorth(t *testing.T) {
	gs := new(GameState)
	gs.MapX = 3
	gs.MapY = 7
	gs.MapDir = 0 // north (index 0)

	geo := new(GeoMap)
	geo.Tiles[7][3].Prop = 0x10
	geo.Tiles[7][3].WallType[0] = 5

	gs.UpdateWallInfo(geo)

	if gs.WallType != 5 {
		t.Errorf("WallType = %d, want 5", gs.WallType)
	}
}

func TestGameStateUpdateWallInfoNil(t *testing.T) {
	gs := new(GameState)
	gs.WallType = 3
	gs.WallRoof = 7
	gs.UpdateWallInfo(nil) // should not change anything
	if gs.WallType != 3 {
		t.Errorf("WallType = %d, want 3", gs.WallType)
	}
	if gs.WallRoof != 7 {
		t.Errorf("WallRoof = %d, want 7", gs.WallRoof)
	}
}

func TestGameStateArea1(t *testing.T) {
	gs := new(GameState)
	gs.SetVar(0x4B00, 0x1234)
	if v := gs.GetVar(0x4B00); v != 0x1234 {
		t.Errorf("Area1 readback = %04X, want 1234", v)
	}
}

func TestGameStateStruct(t *testing.T) {
	gs := new(GameState)
	gs.SetVar(0x7A00, 0xABCD)
	if v := gs.GetVar(0x7A00); v != 0xABCD {
		t.Errorf("Struct readback = %04X, want ABCD", v)
	}
}

func TestGameStatePlayerCount(t *testing.T) {
	gs := new(GameState)
	gs.Players = append(gs.Players, new(Player), new(Player), new(Player))
	if v := gs.GetVar(0x7C00 + 0x33E); v != 3 {
		t.Errorf("player count = %d, want 3", v)
	}
}

func TestPlayerItems(t *testing.T) {
	p := new(Player)
	p.items = []Item{
		{Type: 10, Name: "Sword"},
		{Type: 20, Name: "Shield"},
		{Type: 10, Name: "Dagger"},
	}

	items := p.Items()
	if len(items) != 3 {
		t.Fatalf("Items() returned %d items, want 3", len(items))
	}
	if items[0].Type != 10 {
		t.Errorf("items[0].Type = %d, want 10", items[0].Type)
	}
}

func TestPlayerSpellIDs(t *testing.T) {
	p := new(Player)
	// Spell list at Raw[0x1E:0x72], non-zero bytes are spell IDs
	p.Raw[0x1E] = 0x05 // spell ID 5
	p.Raw[0x1F] = 0x00 // empty
	p.Raw[0x20] = 0x12 // spell ID 18
	p.Raw[0x21] = 0x8F // spell ID 0x0F with learning flag (bit 7)

	ids := p.SpellIDs()
	if len(ids) != 3 {
		t.Fatalf("SpellIDs() returned %d IDs, want 3", len(ids))
	}
	if ids[0] != 5 {
		t.Errorf("ids[0] = %d, want 5", ids[0])
	}
	if ids[1] != 0x12 {
		t.Errorf("ids[1] = %d, want 0x12", ids[1])
	}
	if ids[2] != 0x0F {
		t.Errorf("ids[2] = %d, want 0x0F (bit 7 masked)", ids[2])
	}
}

func TestPlayerHasAffect(t *testing.T) {
	p := new(Player)
	p.affects = []Affect{
		{Type: 0x01, Minutes: 60},
		{Type: 0x27, Minutes: 30},
	}

	if !p.HasAffect(0x01) {
		t.Error("HasAffect(0x01) = false, want true")
	}
	if !p.HasAffect(0x27) {
		t.Error("HasAffect(0x27) = false, want true")
	}
	if p.HasAffect(0x99) {
		t.Error("HasAffect(0x99) = true, want false")
	}
}

func TestPlayerRemoveItemsByType(t *testing.T) {
	p := new(Player)
	p.items = []Item{
		{Type: 10, Name: "Sword"},
		{Type: 20, Name: "Shield"},
		{Type: 10, Name: "Dagger"},
	}

	n := p.RemoveItemsByType(10)
	if n != 2 {
		t.Errorf("removed %d, want 2", n)
	}
	if len(p.items) != 1 {
		t.Fatalf("remaining items = %d, want 1", len(p.items))
	}
	if p.items[0].Type != 20 {
		t.Errorf("remaining item type = %d, want 20", p.items[0].Type)
	}
}

func TestGameStateFindItem(t *testing.T) {
	gs := new(GameState)
	p1 := new(Player)
	p1.items = []Item{{Type: 10}, {Type: 20}}
	p2 := new(Player)
	p2.items = []Item{{Type: 30}}
	gs.Players = []*Player{p1, p2}

	if !gs.FindItem(10) {
		t.Error("FindItem(10) = false, want true (in p1)")
	}
	if !gs.FindItem(30) {
		t.Error("FindItem(30) = false, want true (in p2)")
	}
	if gs.FindItem(99) {
		t.Error("FindItem(99) = true, want false")
	}
}

func TestGameStateFindSpecial(t *testing.T) {
	gs := new(GameState)
	p := new(Player)
	p.affects = []Affect{{Type: 0x05}, {Type: 0x10}}
	gs.Players = []*Player{p}
	gs.SelectedIdx = 0

	if !gs.FindSpecial(0x05) {
		t.Error("FindSpecial(0x05) = false, want true")
	}
	if gs.FindSpecial(0x99) {
		t.Error("FindSpecial(0x99) = true, want false")
	}

	// No players
	gs2 := new(GameState)
	if gs2.FindSpecial(0x05) {
		t.Error("FindSpecial with no players should return false")
	}
}

func TestGameStateSpell(t *testing.T) {
	gs := new(GameState)
	p1 := new(Player)
	p1.Raw[0x1E] = 0x05 // spell ID 5
	p1.Raw[0x1F] = 0x12 // spell ID 18
	p2 := new(Player)
	p2.Raw[0x1E] = 0x2F // spell ID 47 (fireball)
	gs.Players = []*Player{p1, p2}

	// Find spell 0x12 in p1 (index 2, 1-based)
	found := gs.Spell([3]int{0x12, 0x7A00, 0x7A01})
	if !found {
		t.Error("Spell(0x12) = false, want true")
	}
	if v := gs.GetVar(0x7A00); v != 2 {
		t.Errorf("spell index = %d, want 2 (1-based)", v)
	}
	if v := gs.GetVar(0x7A01); v != 0 {
		t.Errorf("player index = %d, want 0", v)
	}

	// Find spell 0x2F in p2
	found = gs.Spell([3]int{0x2F, 0x7A00, 0x7A01})
	if !found {
		t.Error("Spell(0x2F) = false, want true")
	}
	if v := gs.GetVar(0x7A01); v != 1 {
		t.Errorf("player index = %d, want 1", v)
	}

	// Spell not found
	found = gs.Spell([3]int{0x99, 0x7A00, 0x7A01})
	if found {
		t.Error("Spell(0x99) should not be found")
	}
	if v := gs.GetVar(0x7A00); v != 0xFF {
		t.Errorf("not-found spell index = %d, want 0xFF", v)
	}
}

func TestGameStateDestroyItems(t *testing.T) {
	gs := new(GameState)
	p := new(Player)
	p.items = []Item{{Type: 10}, {Type: 20}, {Type: 10}}
	gs.Players = []*Player{p}
	gs.SelectedIdx = 0

	gs.DestroyItems(10)
	if len(p.items) != 1 {
		t.Errorf("after destroy, items = %d, want 1", len(p.items))
	}
	if p.items[0].Type != 20 {
		t.Errorf("remaining item type = %d, want 20", p.items[0].Type)
	}
}

func TestGameStateGetSaveTable(t *testing.T) {
	gs := new(GameState)
	// Write a value via Struct region (0x7A00+)
	gs.SetVar(0x7A00, 42)

	// GetTable reads from memory[base + index]
	val := gs.GetTable(0x7A00, 0, 0)
	if val != 42 {
		t.Errorf("GetTable(0x7A00, 0) = %d, want 42", val)
	}

	// GetTable with offset
	val = gs.GetTable(0x7A00-1, 1, 0)
	if val != 42 {
		t.Errorf("GetTable(0x7A00-1, 1) = %d, want 42", val)
	}

	// SaveTable writes to memory[base + index]
	gs.SaveTable(0x7A02, 0, 0, 99)
	if v := gs.GetVar(0x7A02); v != 99 {
		t.Errorf("after SaveTable, GetVar(0x7A02) = %d, want 99", v)
	}
}

func TestGameStateCheckPartyAffect(t *testing.T) {
	gs := new(GameState)
	p1 := new(Player)
	p1.affects = []Affect{{Type: 0x05, Minutes: 60}}
	p2 := new(Player)
	p2.affects = []Affect{{Type: 0x10, Minutes: 3}}
	gs.Players = []*Player{p1, p2}
	loc_d := uint16(0x7A03)

	// Mode 1: Affect check (var_2 == 8001 after subtracting 0x7FFF)
	gs.CheckParty([6]int{0x7FFF + 8001, 0x05, 0x7A00, 0x7A01, 0x7A02, 0x7A03})
	if v := gs.GetVar(loc_d); v != 1 {
		t.Errorf("CheckParty affect found = %d, want 1", v)
	}

	// Affect not found
	gs.CheckParty([6]int{0x7FFF + 8001, 0x99, 0x7A00, 0x7A01, 0x7A02, 0x7A03})
	if v := gs.GetVar(loc_d); v != 0 {
		t.Errorf("CheckParty affect not found = %d, want 0", v)
	}
}

func TestGameStateCheckPartyMovement(t *testing.T) {
	gs := new(GameState)
	p1 := new(Player)
	p1.Raw[0xE4] = 12 // movement 12
	p2 := new(Player)
	p2.Raw[0xE4] = 8  // movement 8
	gs.Players = []*Player{p1, p2}

	// Movement check: var_2 = 0x7FFF + 0x9F
	gs.CheckParty([6]int{0x7FFF + 0x9F, 0, 0x7A00, 0x7A01, 0x7A02, 0x7A03})
	minVal := gs.GetVar(0x7A00)
	maxVal := gs.GetVar(0x7A01)
	avgVal := gs.GetVar(0x7A02)
	if minVal != 8 {
		t.Errorf("min movement = %d, want 8", minVal)
	}
	if maxVal != 12 {
		t.Errorf("max movement = %d, want 12", maxVal)
	}
	if avgVal != 10 { // (8+12)/2 = 10
		t.Errorf("avg movement = %d, want 10", avgVal)
	}
}
