package dax

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Player represents a character or monster in the Gold Box engine.
type Player struct {
	Raw [0x1A6]byte // Full 422-byte player structure

	// Convenience accessors pull from Raw
	HPCurrent byte
	InCombat  bool

	// Items and affects loaded from separate files
	items   []Item
	affects []Affect
}

func (p *Player) byteAt(off int) byte {
	if off >= 0 && off < len(p.Raw) {
		return p.Raw[off]
	}
	return 0
}

func (p *Player) wordAt(off int) uint16 {
	if off >= 0 && off+2 <= len(p.Raw) {
		return binary.LittleEndian.Uint16(p.Raw[off:])
	}
	return 0
}

func (p *Player) setByte(off int, v byte) {
	if off >= 0 && off < len(p.Raw) {
		p.Raw[off] = v
	}
}

func (p *Player) setWord(off int, v uint16) {
	if off >= 0 && off+2 <= len(p.Raw) {
		binary.LittleEndian.PutUint16(p.Raw[off:], v)
	}
}

// Items returns the player's inventory items (loaded from .itm files).
func (p *Player) Items() []Item { return p.items }

// Affects returns the player's active spell effects (loaded from .spc files).
func (p *Player) Affects() []Affect { return p.affects }

// HasAffect returns true if the player has an active affect with the given type.
func (p *Player) HasAffect(affectType byte) bool {
	for _, a := range p.affects {
		if a.Type == affectType {
			return true
		}
	}
	return false
}

// SpellIDs parses the spell list from Raw[0x1E:0x72] (84 bytes).
// Each non-zero byte is a spell ID (bit 7 = learning status).
// Returns all spell IDs in order.
func (p *Player) SpellIDs() []int {
	var ids []int
	for i := 0x1E; i < 0x72; i++ {
		if p.Raw[i] != 0 {
			ids = append(ids, int(p.Raw[i]&0x7F))
		}
	}
	return ids
}

// RemoveItemsByType removes all items with the given type from the player's
// inventory and returns the number of items removed.
func (p *Player) RemoveItemsByType(itemType byte) int {
	n := 0
	filtered := p.items[:0]
	for _, item := range p.items {
		if item.Type == itemType {
			n++
		} else {
			filtered = append(filtered, item)
		}
	}
	p.items = filtered
	return n
}

// GameState holds all game state and implements the Host interface.
type GameState struct {
	Area1  [0x800]byte // 0x4B00 region: game state, time, flags
	Area2  [0x800]byte // 0x7C00 region: player/encounter data
	Struct [0x400]byte // 0x7A00 region: ECL variables
	ECLData []byte     // 0x8000 region: current ECL bytecode

	Players     []*Player
	SelectedIdx int
	MapX, MapY  int
	MapDir      int       // 0=N, 2=E, 4=S, 6=W (coab convention)
	GameArea    byte
	InDungeon   bool
	PosChanged  bool      // set when VM writes to position addresses
	WallType    byte      // wall type in facing direction (read via 0xC04E)
	WallRoof    byte      // tile property x2 from GEO map (read via 0xC04F)
}

// word reads a uint16 from a byte slice.
func word(b []byte) uint16 {
	if len(b) < 2 {
		return 0
	}
	return binary.LittleEndian.Uint16(b)
}

// putWord writes a uint16 to a byte slice.
func putWord(b []byte, v uint16) {
	if len(b) < 2 {
		return
	}
	binary.LittleEndian.PutUint16(b, v)
}

// GetVar reads a 16-bit value from the VM address space.
// Memory regions (ordered by priority for correct dispatch):
//   0x8000+:       ECL bytecode (size depends on concatenated block data)
//   0x7C00-0x7FFF: Area2 (player/encounter) — but NOT addresses >= 0xC04B
//   0x7A00-0x7BFF: Struct (ECL variables)
//   0x4B00-0x4EFF: Area1 (game state, flags)
//   everything else: globals (includes 0xC04B+ for map position)
func (gs *GameState) GetVar(addr uint16) uint16 {
	switch {
	case addr >= 0x8000:
		loc := int(addr) - 0x8000
		if loc >= 0 && loc < len(gs.ECLData) {
			return uint16(gs.ECLData[loc])
		}
		// Outside ECL data range, fall through to globals
		return gs.getGlobal(addr)

	case addr >= 0x7C00 && addr <= 0x7FFF:
		loc := addr - 0x7C00
		return gs.getArea2(loc)

	case addr >= 0x7A00 && addr <= 0x7BFF:
		loc := int(addr-0x7A00) * 2
		if loc+2 <= len(gs.Struct) {
			return word(gs.Struct[loc:])
		}
		return 0

	case addr >= 0x4B00 && addr <= 0x4EFF:
		loc := int(addr-0x4B00) * 2
		if loc+2 <= len(gs.Area1) {
			return word(gs.Area1[loc:])
		}
		return 0

	default:
		return gs.getGlobal(addr)
	}
}

// SetVar writes a 16-bit value to the VM address space.
func (gs *GameState) SetVar(addr uint16, val uint16) {
	switch {
	case addr >= 0x8000:
		loc := int(addr) - 0x8000
		if loc >= 0 && loc < len(gs.ECLData) {
			gs.ECLData[loc] = byte(val)
			return
		}
		// Outside ECL data range, fall through to globals
		gs.setGlobal(addr, val)

	case addr >= 0x7C00 && addr <= 0x7FFF:
		loc := addr - 0x7C00
		gs.setArea2(loc, val)

	case addr >= 0x7A00 && addr <= 0x7BFF:
		loc := int(addr-0x7A00) * 2
		if loc+2 <= len(gs.Struct) {
			putWord(gs.Struct[loc:], val)
		}

	case addr >= 0x4B00 && addr <= 0x4EFF:
		loc := int(addr-0x4B00) * 2
		if loc+2 <= len(gs.Area1) {
			putWord(gs.Area1[loc:], val)
		}

	default:
		gs.setGlobal(addr, val)
	}
}

// --- Area2 dispatch ---

func (gs *GameState) selectedPlayer() *Player {
	if gs.SelectedIdx >= 0 && gs.SelectedIdx < len(gs.Players) {
		return gs.Players[gs.SelectedIdx]
	}
	return nil
}

// Player field offsets in the Area2 (0x7C00) address space.
// These map to the Player.Raw byte array.
// For offsets not listed here, we fall through to raw Area2 bytes.
var playerFieldMap = map[uint16]int{
	0x15:  0x15,  // stats
	0x18:  0x18,
	0x72:  0x72,  // spell_learn_count
	0x73:  0x73,  // THAC0
	0x74:  0x74,  // race
	0x75:  0x75,  // class
	0x76:  0x76,  // age (word)
	0x78:  0x78,  // HP max
	0xA0:  0xA0,  // hit dice
	0xB8:  0xB8,  // control/morale
	0xBB:  0xBB,  // copper (word)
	0xBD:  0xBD,  // electrum (word)
	0xBF:  0xBF,  // silver (word)
	0xC1:  0xC1,  // gold (word)
	0xC3:  0xC3,  // platinum (word)
	0xC5:  0xC5,  // gems (word)
	0xC7:  0xC7,  // jewelry (word)
	0xD6:  0xD6,  // sex
	0xD8:  0xD8,  // alignment
	0x124: 0x124, // base AC
	0x127: 0x127, // exp low (word)
	0x129: 0x129, // exp high (word)
	0x12B: 0x12B, // class flags
	0x141: 0x141, // head icon
	0x142: 0x142, // weapon icon
	0x143: 0x143, // icon ID
	0x1A4: 0x1A4, // HP current
}

func (gs *GameState) getArea2(loc uint16) uint16 {
	// Special non-player offsets
	switch loc {
	case 0x2B1, 0x2B4:
		return uint16(gs.SelectedIdx)
	case 0x312:
		return uint16(gs.GameArea)
	case 0x33E:
		return uint16(len(gs.Players))
	}

	// Player field dispatch
	if pOff, ok := playerFieldMap[loc]; ok {
		if p := gs.selectedPlayer(); p != nil {
			return p.wordAt(pOff)
		}
	}

	// Spell list range
	if loc >= 0x20 && loc <= 0x73 {
		if p := gs.selectedPlayer(); p != nil {
			return uint16(p.byteAt(int(loc)))
		}
	}

	// Fall through to raw Area2
	if int(loc)+2 <= len(gs.Area2) {
		return word(gs.Area2[loc:])
	}
	return 0
}

func (gs *GameState) setArea2(loc uint16, val uint16) {
	// Special non-player offsets
	switch loc {
	case 0x312:
		gs.GameArea = byte(val)
		return
	}

	// Player field dispatch
	if pOff, ok := playerFieldMap[loc]; ok {
		if p := gs.selectedPlayer(); p != nil {
			p.setWord(pOff, val)
			return
		}
	}

	// Spell list range
	if loc >= 0x20 && loc <= 0x73 {
		if p := gs.selectedPlayer(); p != nil {
			p.setByte(int(loc), byte(val))
			return
		}
	}

	// Fall through to raw Area2
	if int(loc)+2 <= len(gs.Area2) {
		putWord(gs.Area2[loc:], val)
	}
}

// --- Global access ---
// Region mem_type 4: addresses outside the 4 named regions.
// Two sub-regions:
//   - low globals (< 0xC04B): scattered game state vars
//   - high globals (>= 0xC04B): map position (0xC04B=posX, 0xC04C=posY, 0xC04D=dir)

func (gs *GameState) getGlobal(addr uint16) uint16 {
	switch {
	case addr == 0xFB:
		return uint16(gs.MapX)
	case addr == 0xFC:
		return uint16(gs.MapY)
	case addr == 0x3DE:
		return uint16(gs.MapDir)
	case addr >= 0xC04B:
		return gs.getHighGlobal(addr - 0xC04B)
	default:
		return 0
	}
}

func (gs *GameState) setGlobal(addr uint16, val uint16) {
	switch {
	case addr == 0xFB:
		gs.MapX = int(val)
	case addr == 0xFC:
		gs.MapY = int(val)
	case addr == 0x3DE:
		gs.MapDir = int(val)
	case addr >= 0xC04B:
		gs.setHighGlobal(addr-0xC04B, val)
	}
}

// getHighGlobal reads from the 0xC04B+ address space.
func (gs *GameState) getHighGlobal(off uint16) uint16 {
	switch off {
	case 0: // 0xC04B: mapPosX
		return uint16(gs.MapX)
	case 1: // 0xC04C: mapPosY
		return uint16(gs.MapY)
	case 2: // 0xC04D: mapDirection / 2
		return uint16(gs.MapDir / 2)
	case 3: // 0xC04E: mapWallType
		return uint16(gs.WallType)
	case 4: // 0xC04F: mapWallRoof (tile property x2 from GEO)
		return uint16(gs.WallRoof)
	default:
		return 0
	}
}

// setHighGlobal writes to the 0xC04B+ address space.
// Offsets are relative to 0xC04B: 0=posX, 1=posY, 2=direction/2.
func (gs *GameState) setHighGlobal(off uint16, val uint16) {
	switch off {
	case 0: // 0xC04B: mapPosX
		gs.MapX = int(int8(val))
		gs.PosChanged = true
	case 1: // 0xC04C: mapPosY
		gs.MapY = int(int8(val))
		gs.PosChanged = true
	case 2: // 0xC04D: mapDirection (VM uses 0-3, engine uses 0,2,4,6)
		dir := val
		for {
			switch dir {
			case 0:
				gs.MapDir = 0
				gs.PosChanged = true
				return
			case 1:
				gs.MapDir = 2
				gs.PosChanged = true
				return
			case 2:
				gs.MapDir = 4
				gs.PosChanged = true
				return
			case 3:
				gs.MapDir = 6
				gs.PosChanged = true
				return
			default:
				dir -= 4
			}
		}
	case 3: // 0xC04E: mapWallType
		gs.WallType = byte(val)
	case 4: // 0xC04F: mapWallRoof
		gs.WallRoof = byte(val)
	}
}

// --- Host interface stubs ---

func (gs *GameState) Print(string, bool)                             {}
func (gs *GameState) Picture(int)                                    {}
func (gs *GameState) InputNumber(string) int                         { return 0 }
func (gs *GameState) InputString(string) string                      { return "" }
func (gs *GameState) ShowMenu(items []string, vertical bool) int     { return 0 }
func (gs *GameState) WaitKey()                                       {}
func (gs *GameState) LoadCharacter(idx int) {
	if idx >= 0 && idx < len(gs.Players) {
		gs.SelectedIdx = idx
	}
}
func (gs *GameState) Combat() bool                                  { return false }
func (gs *GameState) Delay()                                        {}
func (gs *GameState) SelectPlayer() int                             { return 0 }
func (gs *GameState) GetRandom(max int) int                         { return 0 }
func (gs *GameState) LoadMonster(int, int, int)                     {}
func (gs *GameState) SetupMonster(int, int, int)                    {}
func (gs *GameState) ClearMonsters()                                {}
func (gs *GameState) Approach()                                     {}
func (gs *GameState) SpriteOff()                                    {}
func (gs *GameState) NewECL(byte)                                   {}
func (gs *GameState) Program(int)                                   {}
func (gs *GameState) CallSub(int)                                   {}
func (gs *GameState) Treasure([8]int)                               {}
func (gs *GameState) Rob(int, int, int)                             {}
// FindItem searches all party members' inventories for an item with the
// given type. Returns true if any player has a matching item.
// Matches coab's CMD_FindItem which iterates gbl.TeamList.
func (gs *GameState) FindItem(itemType int) bool {
	for _, p := range gs.Players {
		for _, item := range p.items {
			if int(item.Type) == itemType {
				return true
			}
		}
	}
	return false
}

// DestroyItems removes all items with the given type from the selected
// player's inventory.
func (gs *GameState) DestroyItems(itemType int) {
	if p := gs.selectedPlayer(); p != nil {
		p.RemoveItemsByType(byte(itemType))
	}
}

// FindSpecial checks if the currently selected player has an active
// affect with the given type. Matches coab's CMD_FindSpecial which
// checks gbl.SelectedPlayer only (not the full party).
func (gs *GameState) FindSpecial(affect int) bool {
	if p := gs.selectedPlayer(); p != nil {
		return p.HasAffect(byte(affect))
	}
	return false
}
func (gs *GameState) PartyStrength() int                            { return 0 }
// CheckParty performs party-wide checks based on the selector in args[0].
// After subtracting 0x7FFF from args[0], the result determines the mode:
//   - 8001 (0x1F41): affect check — does any party member have affect args[1]?
//   - 0xA5..0xAC: thief skill check — min/max/avg of thief_skills[index-1]
//   - 0x9F: movement check — min/max/avg of player movement
// Results are written via SetVar to loc_a, loc_b, loc_c, loc_d.
func (gs *GameState) CheckParty(args [6]int) {
	selector := args[0] - 0x7FFF
	locA := uint16(args[2])
	locB := uint16(args[3])
	locC := uint16(args[4])
	locD := uint16(args[5])

	switch {
	case selector == 8001:
		// Affect check: does any party member have the specified affect?
		affectID := byte(args[1])
		found := false
		for _, p := range gs.Players {
			if p.HasAffect(affectID) {
				found = true
				break
			}
		}
		gs.SetVar(locA, 0)
		gs.SetVar(locB, 0)
		gs.SetVar(locC, 0)
		if found {
			gs.SetVar(locD, 1)
		} else {
			gs.SetVar(locD, 0)
		}

	case selector >= 0xA5 && selector <= 0xAC:
		// Thief skill check: compute min, max, avg across party
		index := selector - 0xA4 // 1-based into thief_skills array
		count := 0
		var minV, maxV, sum uint16
		first := true
		for _, p := range gs.Players {
			v := uint16(p.byteAt(0xEA + index - 1))
			count++
			if first {
				minV = v
				maxV = v
				first = false
			} else {
				if v < minV {
					minV = v
				}
				if v > maxV {
					maxV = v
				}
			}
			sum += v
		}
		var avgV uint16
		if count > 0 {
			avgV = sum / uint16(count)
		}
		gs.SetVar(locA, minV)
		gs.SetVar(locB, maxV)
		gs.SetVar(locC, avgV)
		gs.SetVar(locD, 0)

	case selector == 0x9F:
		// Movement check: compute min, max, avg across party
		count := 0
		var minV, maxV, sum uint16
		first := true
		for _, p := range gs.Players {
			v := uint16(p.byteAt(0xE4))
			count++
			if first {
				minV = v
				maxV = v
				first = false
			} else {
				if v < minV {
					minV = v
				}
				if v > maxV {
					maxV = v
				}
			}
			sum += v
		}
		var avgV uint16
		if count > 0 {
			avgV = sum / uint16(count)
		}
		gs.SetVar(locA, minV)
		gs.SetVar(locB, maxV)
		gs.SetVar(locC, avgV)
		gs.SetVar(locD, 0)
	}
}
func (gs *GameState) PartySurprise([2]int) (bool, bool)             { return false, false }
func (gs *GameState) Surprise([4]int)                               {}
// Spell searches all party members' spell lists for the given spell ID.
// On match, writes the 1-based spell index to locA and 0-based player
// index to locB via SetVar. If not found, writes 0xFF to locA.
// Matches coab's CMD_Spell (ovr003.cs).
func (gs *GameState) Spell(args [3]int) bool {
	spellID := args[0]
	locA := uint16(args[1])
	locB := uint16(args[2])

	for pi, p := range gs.Players {
		spellIndex := 0
		for _, id := range p.SpellIDs() {
			spellIndex++
			if id == spellID {
				gs.SetVar(locA, uint16(spellIndex))
				gs.SetVar(locB, uint16(pi))
				return true
			}
		}
	}

	gs.SetVar(locA, 0xFF)
	return false
}
func (gs *GameState) Damage(int, int, int, int, int)                {}
// GetTable reads a value from the VM address space at (table + index).
// Matches coab's CMD_GetTable: addr = base + index, result = vm_GetMemoryValue(addr).
func (gs *GameState) GetTable(table, index, count int) uint16 {
	addr := uint16(table + index)
	return gs.GetVar(addr)
}

// SaveTable writes a value to the VM address space at (table + index).
// Matches coab's CMD_SaveTable: addr = base + index, vm_SetMemoryValue(value, addr).
func (gs *GameState) SaveTable(table, index, count int, val uint16) {
	addr := uint16(table + index)
	gs.SetVar(addr, val)
}
func (gs *GameState) Clock(int, int)                                {}
func (gs *GameState) AddNPC(int, int)                               {}
func (gs *GameState) LoadPieces(int, int, int)                      {}
func (gs *GameState) LoadFiles(int, int, int)                       {}
func (gs *GameState) Dump()                                         {}
func (gs *GameState) ClearBox()                                     {}
func (gs *GameState) Protection(int)                                {}
func (gs *GameState) EncounterMenu([14]int)                         {}
func (gs *GameState) Parlay([6]int)                                 {}

// SaveData holds the parsed contents of a save game file.
type SaveData struct {
	GameArea byte
	Area1   [0x800]byte
	Area2   [0x800]byte
	Struct  [0x400]byte
	ECLData []byte
	MapX, MapY int
	MapDir   int
	WallType byte
	WallRoof byte
	LastEclBlockID byte // from Area1 word offset 0x1E4 (byte offset 0x3C8)
}

// LoadSaveGame reads a Pool of Radiance save game file (savgamX.dat).
// File layout (from coab ovr017.cs):
//
//	offset 0x0000: game_area (1 byte)
//	offset 0x0001: Area1 (0x800 bytes)
//	offset 0x0801: Area2 (0x800 bytes)
//	offset 0x1001: Struct (0x400 bytes)
//	offset 0x1401: ECL block (0x1E00 bytes)
//	offset 0x3201: map position (5 bytes: posX, posY, dir, wallType, wallRoof)
func LoadSaveGame(path string) (*SaveData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	const minSaveSize = 0x1401 // Struct region end
	if len(data) < minSaveSize {
		return nil, fmt.Errorf("save file too small: %d bytes, need at least %d", len(data), minSaveSize)
	}

	sd := new(SaveData)
	sd.GameArea = data[0]

	copy(sd.Area1[:], data[1:0x801])
	copy(sd.Area2[:], data[0x801:0x1001])
	copy(sd.Struct[:], data[0x1001:0x1401])

	eclSize := 0x1E00
	if len(data) >= 0x1401+eclSize {
		sd.ECLData = make([]byte, eclSize)
		copy(sd.ECLData, data[0x1401:0x1401+eclSize])
	}

	if len(data) >= 0x3206 {
		sd.MapX = int(int8(data[0x3201]))
		sd.MapY = int(int8(data[0x3202]))
		sd.MapDir = int(data[0x3203])
		sd.WallType = data[0x3204]
		sd.WallRoof = data[0x3205]
	}

	// LastEclBlockId is stored at Area1 byte offset 0x1E4.
	// (coab uses word-indexed DataOffset 0x1E4, but the raw file stores
	// the struct at byte offsets directly — no doubling.)
	if len(sd.Area1) > 0x1E4 {
		sd.LastEclBlockID = sd.Area1[0x1E4]
	}

	return sd, nil
}

// ApplySaveGame loads parsed save data into the GameState.
func (gs *GameState) ApplySaveGame(sd *SaveData) {
	gs.Area1 = sd.Area1
	gs.Area2 = sd.Area2
	gs.Struct = sd.Struct
	gs.GameArea = sd.GameArea
	if len(sd.ECLData) > 0 {
		gs.ECLData = sd.ECLData
	}
	gs.MapX = sd.MapX
	gs.MapY = sd.MapY
	if sd.MapDir != 0 {
		gs.MapDir = sd.MapDir
	}
	gs.WallType = sd.WallType
	gs.WallRoof = sd.WallRoof
}

// UpdateWallInfo computes WallRoof and WallType from a GeoMap for the current position.
// WallRoof is the tile's x2 property (page 2 byte). WallType is the wall type in the
// facing direction from the current tile.
func (gs *GameState) UpdateWallInfo(geo *GeoMap) {
	if geo == nil {
		return
	}
	x, y := gs.MapX, gs.MapY
	// Wrap to 0-15 range
	x &= 0x0F
	y &= 0x0F

	tile := geo.Tiles[y][x]
	gs.WallRoof = tile.Prop

	// WallType depends on facing direction
	dirIdx := gs.MapDir / 2 // 0,1,2,3 for N,E,S,W
	if dirIdx >= 0 && dirIdx < 4 {
		gs.WallType = tile.WallType[dirIdx]
	}
}
