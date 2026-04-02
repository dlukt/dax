package dax

import "encoding/binary"

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

// GameState holds all game state and implements the Host interface.
type GameState struct {
	Area1  [0x800]byte // 0x4B00 region: game state, time, flags
	Area2  [0x800]byte // 0x7C00 region: player/encounter data
	Struct [0x400]byte // 0x7A00 region: ECL variables
	ECLData []byte     // 0x8000 region: current ECL bytecode

	Players     []*Player
	SelectedIdx int
	MapX, MapY  int
	MapDir      int
	GameArea    byte
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
func (gs *GameState) GetVar(addr uint16) uint16 {
	switch {
	case addr >= 0x8000:
		loc := int(addr) - 0x8000
		if loc < len(gs.ECLData) {
			return uint16(gs.ECLData[loc])
		}
		return 0

	case addr >= 0x7C00:
		loc := addr - 0x7C00
		return gs.getArea2(loc)

	case addr >= 0x7A00:
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
		if loc < len(gs.ECLData) {
			gs.ECLData[loc] = byte(val)
		}

	case addr >= 0x7C00:
		loc := addr - 0x7C00
		gs.setArea2(loc, val)

	case addr >= 0x7A00:
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

func (gs *GameState) getGlobal(addr uint16) uint16 {
	switch addr {
	case 0xFB:
		return uint16(gs.MapX)
	case 0xFC:
		return uint16(gs.MapY)
	case 0x3DE:
		return uint16(gs.MapDir)
	default:
		return 0
	}
}

func (gs *GameState) setGlobal(addr uint16, val uint16) {
	switch addr {
	case 0xFB:
		gs.MapX = int(val)
	case 0xFC:
		gs.MapY = int(val)
	case 0x3DE:
		gs.MapDir = int(val)
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
func (gs *GameState) FindItem(int) bool                             { return false }
func (gs *GameState) DestroyItems(int)                              {}
func (gs *GameState) FindSpecial(int) bool                          { return false }
func (gs *GameState) PartyStrength() int                            { return 0 }
func (gs *GameState) CheckParty([6]int)                             {}
func (gs *GameState) PartySurprise([2]int) (bool, bool)             { return false, false }
func (gs *GameState) Surprise([4]int)                               {}
func (gs *GameState) Spell([3]int) bool                             { return false }
func (gs *GameState) Damage(int, int, int, int, int)                {}
func (gs *GameState) GetTable(int, int, int) uint16                 { return 0 }
func (gs *GameState) SaveTable(int, int, int, uint16)               {}
func (gs *GameState) Clock(int, int)                                {}
func (gs *GameState) AddNPC(int, int)                               {}
func (gs *GameState) LoadPieces(int, int, int)                      {}
func (gs *GameState) LoadFiles(int, int, int)                       {}
func (gs *GameState) Dump()                                         {}
func (gs *GameState) ClearBox()                                     {}
func (gs *GameState) Protection(int)                                {}
func (gs *GameState) EncounterMenu([14]int)                         {}
func (gs *GameState) Parlay([6]int)                                 {}
