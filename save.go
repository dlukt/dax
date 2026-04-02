package dax

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PoolRadPlayerSize is the size of a Pool of Radiance character file.
const PoolRadPlayerSize = 0x11D // 285 bytes

// ItemSize is the size of a single item record (0x3F = 63 bytes).
const ItemSize = 0x3F

// AffectSize is the size of a single affect record (9 bytes).
const AffectSize = 9

// LoadParty loads all character files matching chrdata{1..6}.sav from dir,
// plus their .itm and .spc files, and returns a populated GameState.
func LoadParty(dir string) (*GameState, error) {
	gs := new(GameState)

	for i := 1; i <= 6; i++ {
		savPath := filepath.Join(dir, fmt.Sprintf("chrdata%d.sav", i))
		if _, err := os.Stat(savPath); err != nil {
			continue
		}
		p, err := LoadCharacter(savPath)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", savPath, err)
		}

		// Load items
		itmPath := filepath.Join(dir, fmt.Sprintf("chrdata%d.itm", i))
		if items, err := LoadItems(itmPath); err == nil {
			p.items = items
		}

		// Load affects (.spc for Pool characters)
		spcPath := filepath.Join(dir, fmt.Sprintf("chrdata%d.spc", i))
		if affects, err := LoadAffects(spcPath); err == nil {
			p.affects = affects
		}

		gs.Players = append(gs.Players, p)
	}

	return gs, nil
}

// LoadCharacter reads a Pool of Radiance .sav file and converts it
// into the Curse-era Player format (422 bytes in Raw).
func LoadCharacter(path string) (*Player, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) != PoolRadPlayerSize {
		return nil, fmt.Errorf("expected %d bytes, got %d", PoolRadPlayerSize, len(data))
	}
	return ConvertPoolRadPlayer(data), nil
}

// ConvertPoolRadPlayer maps a 285-byte PoolRadPlayer into a 422-byte Player.Raw.
func ConvertPoolRadPlayer(src []byte) *Player {
	p := new(Player)

	// Name: Pascal string at src 0x00, 15 bytes → Player.Raw 0x00, 15 bytes
	// Copy name bytes directly (Pascal string: len byte + chars)
	copy(p.Raw[0x00:0x0F], src[0x00:0x0F])

	// Stats: src 0x10-0x16 → Player.Raw 0x10-0x1B (cur+full pairs)
	// In Curse format, stats are stored as [cur, full] pairs at:
	//   STR 0x10, INT 0x12, WIS 0x14, DEX 0x16, CON 0x18, CHA 0x1A, STR00 0x1C
	setStatPair(&p.Raw, 0x10, src[0x10]) // STR
	setStatPair(&p.Raw, 0x12, src[0x11]) // INT
	setStatPair(&p.Raw, 0x14, src[0x12]) // WIS
	setStatPair(&p.Raw, 0x16, src[0x13]) // DEX
	setStatPair(&p.Raw, 0x18, src[0x14]) // CON
	setStatPair(&p.Raw, 0x1A, src[0x15]) // CHA
	setStatPair(&p.Raw, 0x1C, src[0x16]) // STR00

	// THAC0: src 0x2D → Player.Raw 0x73 (sbyte)
	p.Raw[0x73] = src[0x2D]

	// Race: src 0x2E → Player.Raw 0x74
	p.Raw[0x74] = src[0x2E]

	// Class: src 0x2F → Player.Raw 0x75
	p.Raw[0x75] = src[0x2F]

	// Age: src 0x30 (word) → Player.Raw 0x76 (word)
	binary.LittleEndian.PutUint16(p.Raw[0x76:], binary.LittleEndian.Uint16(src[0x30:]))

	// HP max: src 0x32 → Player.Raw 0x78
	p.Raw[0x78] = src[0x32]

	// Spell book: src 0x33-0x6A (56 bytes) → Player.Raw 0x79-0xB0
	copy(p.Raw[0x79:0x79+0x38], src[0x33:0x33+0x38])

	// Attack level: src 0x6B → Player.Raw 0xDD
	p.Raw[0xDD] = src[0x6B]

	// Field 0x6C → Player.Raw 0xDE
	p.Raw[0xDE] = src[0x6C]

	// Save verses: src 0x6D-0x71 (5 bytes) → Player.Raw 0xDF-0xE3
	copy(p.Raw[0xDF:0xDF+5], src[0x6D:0x6D+5])

	// Base movement: src 0x72 → Player.Raw 0xE4
	p.Raw[0xE4] = src[0x72]

	// Hit dice: src 0x73 → Player.Raw 0xE5
	p.Raw[0xE5] = src[0x73]

	// Multiclass level = hit dice → Player.Raw 0xE6
	p.Raw[0xE6] = src[0x73]

	// Lost levels: src 0x74 → Player.Raw 0xE7
	p.Raw[0xE7] = src[0x74]

	// Lost HP: src 0x75 → Player.Raw 0xE8
	p.Raw[0xE8] = src[0x75]

	// Field 0x76 → Player.Raw 0xE9
	p.Raw[0xE9] = src[0x76]

	// Thief skills: src 0x77-0x7E (8 bytes) → Player.Raw 0xEA-0xF1
	copy(p.Raw[0xEA:0xEA+8], src[0x77:0x77+8])

	// Field 0x83 → Player.Raw 0xF6
	p.Raw[0xF6] = src[0x83]

	// Control/morale: src 0x84 → Player.Raw 0xF7
	p.Raw[0xF7] = src[0x84]

	// NPC treasure share: src 0x85 → Player.Raw 0xF8
	p.Raw[0xF8] = src[0x85]

	// Field 0x86 → Player.Raw 0xF9
	p.Raw[0xF9] = src[0x86]

	// Field 0x87 → Player.Raw 0xFA
	p.Raw[0xFA] = src[0x87]

	// Money: Pool imports get 300 platinum at Player.Raw 0xFB (7 shorts = 14 bytes)
	// Platinum is index 4: offset 0xFB + 4*2 = 0x103
	binary.LittleEndian.PutUint16(p.Raw[0x103:], 300)

	// Class levels: src 0x96-0x9D (8 bytes) → Player.Raw 0x109-0x110
	copy(p.Raw[0x109:0x109+8], src[0x96:0x96+8])

	// Sex: src 0x9E → Player.Raw 0x119
	p.Raw[0x119] = src[0x9E]

	// Monster type: src 0x9F → Player.Raw 0x11A
	p.Raw[0x11A] = src[0x9F]

	// Alignment: src 0xA0 → Player.Raw 0x11B
	p.Raw[0x11B] = src[0xA0]

	// Attacks count: src 0xA1 → Player.Raw 0x11C
	p.Raw[0x11C] = src[0xA1]

	// Base half moves: src 0xA2 → Player.Raw 0x11D
	p.Raw[0x11D] = src[0xA2]

	// Attack dice: src 0xA3-0xA8 → Player.Raw 0x11E-0x123
	copy(p.Raw[0x11E:0x11E+6], src[0xA3:0xA3+6])

	// Base AC: src 0xA9 → Player.Raw 0x124
	p.Raw[0x124] = src[0xA9]

	// Field 0xAA → Player.Raw 0x125
	p.Raw[0x125] = src[0xAA]

	// Mod ID: src 0xAB → Player.Raw 0x126
	p.Raw[0x126] = src[0xAB]

	// Experience: src 0xAC (int32) → Player.Raw 0x127 (int32)
	copy(p.Raw[0x127:0x127+4], src[0xAC:0xAC+4])

	// Class flags: src 0xB0 → Player.Raw 0x12B
	p.Raw[0x12B] = src[0xB0]

	// HP rolled: src 0xB1 → Player.Raw 0x12C
	p.Raw[0x12C] = src[0xB1]

	// Spell cast counts (cleric 1-3): src 0xB2-0xB4 → Player.Raw 0x12D-0x12F
	copy(p.Raw[0x12D:0x12D+3], src[0xB2:0xB2+3])

	// Spell cast counts (MU 1-3): src 0xB5-0xB7 → Player.Raw 0x132-0x134
	copy(p.Raw[0x132:0x132+3], src[0xB5:0xB5+3])

	// Field B8 (short): src 0xB8 → Player.Raw 0x13C
	binary.LittleEndian.PutUint16(p.Raw[0x13C:], binary.LittleEndian.Uint16(src[0xB8:]))

	// Fields BA-BC → Player.Raw 0x13E-0x140
	p.Raw[0x13E] = src[0xBA]
	p.Raw[0x13F] = src[0xBB]
	p.Raw[0x140] = src[0xBC]

	// Head icon: src 0xBD → Player.Raw 0x141
	p.Raw[0x141] = src[0xBD]

	// Weapon icon: src 0xBE → Player.Raw 0x142
	p.Raw[0x142] = src[0xBE]

	// Icon size: src 0xC0 → Player.Raw 0x144
	p.Raw[0x144] = src[0xC0]

	// Icon colours: src 0xC1-0xC6 (6 bytes) → Player.Raw 0x145-0x14A
	copy(p.Raw[0x145:0x145+6], src[0xC1:0xC1+6])

	// Weapons hands used: src 0x100 → Player.Raw 0x185
	p.Raw[0x185] = src[0x100]

	// Field 0x101 → Player.Raw 0x186
	p.Raw[0x186] = src[0x101]

	// Weight: src 0x102 (short) → Player.Raw 0x187 (short)
	binary.LittleEndian.PutUint16(p.Raw[0x187:], binary.LittleEndian.Uint16(src[0x102:]))

	// Health status: src 0x10C → Player.Raw 0x195
	p.Raw[0x195] = src[0x10C]

	// In combat: src 0x10D → Player.Raw 0x196
	p.Raw[0x196] = src[0x10D]

	// Combat team: src 0x10E → Player.Raw 0x197
	p.Raw[0x197] = src[0x10E]

	// Hit bonus: src 0x110 → Player.Raw 0x199
	p.Raw[0x199] = src[0x110]

	// AC: src 0x111 → Player.Raw 0x19A
	p.Raw[0x19A] = src[0x111]

	// AC behind: src 0x112 → Player.Raw 0x19B
	p.Raw[0x19B] = src[0x112]

	// Attacks left: src 0x113-0x114 → Player.Raw 0x19C-0x19D
	p.Raw[0x19C] = src[0x113]
	p.Raw[0x19D] = src[0x114]

	// Attack dice count: src 0x115-0x116 → Player.Raw 0x19E-0x19F
	p.Raw[0x19E] = src[0x115]
	p.Raw[0x19F] = src[0x116]

	// Attack dice size: src 0x117-0x118 → Player.Raw 0x1A0-0x1A1
	p.Raw[0x1A0] = src[0x117]
	p.Raw[0x1A1] = src[0x118]

	// Attack damage bonus: src 0x119-0x11A → Player.Raw 0x1A2-0x1A3
	p.Raw[0x1A2] = src[0x119]
	p.Raw[0x1A3] = src[0x11A]

	// Current HP: src 0x11B → Player.Raw 0x1A4
	p.Raw[0x1A4] = src[0x11B]
	p.HPCurrent = src[0x11B]

	// Movement/initiative: src 0x11C → Player.Raw 0x1A5
	p.Raw[0x1A5] = src[0x11C]

	return p
}

// setStatPair writes a stat value as [cur, full] pair in the Player.Raw array.
func setStatPair(raw *[0x1A6]byte, offset int, val byte) {
	raw[offset] = val
	raw[offset+1] = val
}

// PlayerName returns the character name from a Player.
func PlayerName(p *Player) string {
	if p == nil {
		return ""
	}
	nameLen := int(p.Raw[0])
	if nameLen > 14 {
		nameLen = 14
	}
	if nameLen == 0 {
		return ""
	}
	return strings.TrimRight(string(p.Raw[1 : 1+nameLen]), "\x00")
}

// LoadItems reads an .itm file containing 0x3F-byte item records.
func LoadItems(path string) ([]Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var items []Item
	for offset := 0; offset+ItemSize <= len(data); offset += ItemSize {
		items = append(items, parseItem(data[offset:offset+ItemSize]))
	}
	return items, nil
}

// Item represents an inventory item (0x3F bytes).
type Item struct {
	Name           string
	Type           byte
	NameNum1       byte
	NameNum2       byte
	NameNum3       byte
	Plus           byte
	PlusSave       byte
	Readied        bool
	HiddenNames    byte
	Cursed         bool
	Weight         uint16
	Count          byte
	Value          uint16
	Affect1        byte
	Affect2        byte
	Affect3        byte
}

func parseItem(data []byte) Item {
	// Item name: 0x2A bytes, Pascal-style (len byte at 0, chars follow)
	nameLen := int(data[0])
	if nameLen > 0x29 {
		nameLen = 0x29
	}
	name := strings.TrimRight(string(data[1:1+nameLen]), "\x00")

	return Item{
		Name:        name,
		Type:        data[0x2E],
		NameNum1:    data[0x2F],
		NameNum2:    data[0x30],
		NameNum3:    data[0x31],
		Plus:        data[0x32],
		PlusSave:    data[0x33],
		Readied:     data[0x34] != 0,
		HiddenNames: data[0x35],
		Cursed:      data[0x36] != 0,
		Weight:      binary.LittleEndian.Uint16(data[0x37:]),
		Count:       data[0x39],
		Value:       binary.LittleEndian.Uint16(data[0x3A:]),
		Affect1:     data[0x3C],
		Affect2:     data[0x3D],
		Affect3:     data[0x3E],
	}
}

// Affect represents a spell effect / status (9 bytes).
type Affect struct {
	Type          byte
	Minutes       uint16
	AffectData    byte
	CallAffectTab bool
}

// LoadAffects reads a .spc file containing 9-byte affect records.
func LoadAffects(path string) ([]Affect, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var affects []Affect
	for offset := 0; offset+AffectSize <= len(data); offset += AffectSize {
		affects = append(affects, Affect{
			Type:          data[offset+0x0],
			Minutes:       binary.LittleEndian.Uint16(data[offset+0x1:]),
			AffectData:    data[offset+0x3],
			CallAffectTab: data[offset+0x4] != 0,
		})
	}
	return affects, nil
}
