package dax

import (
	"path/filepath"
	"sync"
	"testing"
)

// testHost is a minimal Host implementation that records calls for assertions.
type testHost struct {
	mu       sync.Mutex
	vars     map[uint16]uint16
	prints   []string
	pictures []int
	menus    [][]string
	menuRes  int
	randVal  int
	calls    []string
}

func newTestHost() *testHost {
	return &testHost{
		vars:    make(map[uint16]uint16),
		menuRes: 0,
		randVal: 0,
	}
}

func (h *testHost) GetVar(addr uint16) uint16      { return h.vars[addr] }
func (h *testHost) SetVar(addr uint16, val uint16)  { h.vars[addr] = val }
func (h *testHost) Print(t string, _ bool)          { h.prints = append(h.prints, t) }
func (h *testHost) Picture(id int)                   { h.pictures = append(h.pictures, id) }
func (h *testHost) InputNumber(_ string) int         { return 0 }
func (h *testHost) InputString(_ string) string      { return "" }
func (h *testHost) ShowMenu(items []string, v bool) int {
	h.menus = append(h.menus, items)
	return h.menuRes
}
func (h *testHost) WaitKey()                          {}
func (h *testHost) LoadCharacter(idx int)             { h.calls = append(h.calls, "LoadChar") }
func (h *testHost) Combat() bool                      { return false }
func (h *testHost) Delay()                            {}
func (h *testHost) SelectPlayer() int                 { return 0 }
func (h *testHost) GetRandom(max int) int             { return h.randVal }
func (h *testHost) LoadMonster(_, _, _ int)           {}
func (h *testHost) SetupMonster(_, _, _ int)          {}
func (h *testHost) ClearMonsters()                    {}
func (h *testHost) Approach()                         {}
func (h *testHost) SpriteOff()                        {}
func (h *testHost) NewECL(_ byte)                     {}
func (h *testHost) Program(_ int)                     {}
func (h *testHost) CallSub(id int)                    { h.calls = append(h.calls, "Call") }
func (h *testHost) Treasure(_ [8]int)                 {}
func (h *testHost) Rob(_, _, _ int)                   {}
func (h *testHost) FindItem(_ int) bool               { return false }
func (h *testHost) DestroyItems(_ int)                {}
func (h *testHost) FindSpecial(_ int) bool            { return false }
func (h *testHost) PartyStrength() int                { return 42 }
func (h *testHost) CheckParty(_ [6]int)               {}
func (h *testHost) PartySurprise(_ [2]int) (_, _ bool) { return false, false }
func (h *testHost) Surprise(_ [4]int)                 {}
func (h *testHost) Spell(_ [3]int) bool               { return false }
func (h *testHost) Damage(_, _, _, _, _ int)          {}
func (h *testHost) GetTable(_, _, _ int) uint16       { return 0 }
func (h *testHost) SaveTable(_, _, _ int, _ uint16)   {}
func (h *testHost) Clock(_, _ int)                    {}
func (h *testHost) AddNPC(_, _ int)                   {}
func (h *testHost) LoadPieces(_, _, _ int)            {}
func (h *testHost) LoadFiles(_, _, _ int)             {}
func (h *testHost) Dump()                             {}
func (h *testHost) ClearBox()                         {}
func (h *testHost) Protection(_ int)                  {}
func (h *testHost) EncounterMenu(_ [14]int)           {}
func (h *testHost) Parlay(_ [6]int)                   {}

// buildECL constructs an ECL bytecode array from raw bytes.
// Prefixes with a 10-byte header (5 x 16-bit entry points).
func buildECL(headerEntries []uint16, code []byte) []byte {
	buf := make([]byte, 10)
	for i, v := range headerEntries {
		if i >= 5 {
			break
		}
		buf[i*2] = byte(v)
		buf[i*2+1] = byte(v >> 8)
	}
	return append(buf, code...)
}

// operand literal byte: code=0x00, low=val
func opLit(val byte) []byte { return []byte{0x00, val} }

// operand word: code=0x02, low, high
func opWord(val uint16) []byte { return []byte{0x02, byte(val), byte(val >> 8)} }

// operand memory: code=0x01, low, high
func opMem(addr uint16) []byte { return []byte{0x01, byte(addr), byte(addr >> 8)} }

func TestVMExit(t *testing.T) {
	h := newTestHost()
	code := []byte{0x00} // EXIT
	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10) // start at code section (past header)
	if !vm.stop {
		t.Error("VM should have stopped")
	}
}

func TestVMGoto(t *testing.T) {
	h := newTestHost()
	// Code section starts at offset 10.
	const base = 10
	// GOTO base+5, then EXIT at code offset 5
	code := []byte{
		0x01, // GOTO
	}
	code = append(code, opWord(base+5)...)
	code = append(code, make([]byte, 2)...) // padding
	code = append(code, 0x00)               // EXIT at code offset 5

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)
	if !vm.stop {
		t.Error("VM should have stopped after GOTO -> EXIT")
	}
}

func TestVMGosubReturn(t *testing.T) {
	h := newTestHost()
	// Code section starts at offset 10 (after header).
	// All addresses are absolute offsets into data[].
	const base = 10
	// offset 0: GOSUB base+5 (jump to code byte 5)
	// offset 3: EXIT (safety)
	// offset 5: SAVE #05 [mem:0010]
	// offset 8: RETURN
	// offset 9: SAVE #0A [mem:0012]
	// offset 12: EXIT
	// code[0]:  GOSUB -> sub at code[11]
	// code[4]:  SAVE #0A [mem:0012] (after return)
	// code[10]: EXIT
	// code[11]: SAVE #05 [mem:0010] (subroutine)
	// code[17]: RETURN
	code := []byte{0x02}
	code = append(code, opWord(uint16(base+11))...)
	code = append(code, 0x09)
	code = append(code, opLit(10)...)
	code = append(code, opMem(0x12)...)
	code = append(code, 0x00) // EXIT
	// subroutine at code[11]:
	code = append(code, 0x09)
	code = append(code, opLit(5)...)
	code = append(code, opMem(0x10)...)
	code = append(code, 0x13) // RETURN

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	if h.vars[0x10] != 5 {
		t.Errorf("var[0x10] = %d, want 5", h.vars[0x10])
	}
	if h.vars[0x12] != 10 {
		t.Errorf("var[0x12] = %d, want 10", h.vars[0x12])
	}
}

func TestVMCompareAndIf(t *testing.T) {
	h := newTestHost()
	// COMPARE #03, #05  (compare 3 vs 5)
	// IF_GT -> goto "yes" (3 < 5 is false for GT, so this should NOT skip)
	// Wait — compare order: resolve(op[0])=a, resolve(op[1])=b, compareVars(b, a)
	// So COMPARE #03, #05: a=3, b=5, compareVars(5, 3) => EQ=F, NE=T, LT=F, GT=T, LE=F, GE=T
	// IF_GT: flag[3]=true => don't skip => execute next
	// IF_LT: flag[2]=false => skip next

	code := []byte{
		0x03, // COMPARE
	}
	code = append(code, opLit(3)...)  // op[0] = 3
	code = append(code, opLit(5)...)  // op[1] = 5
	code = append(code, 0x19)         // IF_GT (flag index 3, which is true) => don't skip
	code = append(code, 0x09)         // SAVE
	code = append(code, opLit(1)...)  // value 1
	code = append(code, opMem(0x20)...) // dest
	code = append(code, 0x18)         // IF_LT (flag index 2, which is false) => skip next
	code = append(code, 0x09)         // SAVE (should be skipped)
	code = append(code, opLit(2)...)  // value 2
	code = append(code, opMem(0x22)...) // dest
	code = append(code, 0x00)         // EXIT

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	if h.vars[0x20] != 1 {
		t.Errorf("var[0x20] = %d, want 1 (IF_GT should not have skipped)", h.vars[0x20])
	}
	if h.vars[0x22] != 0 {
		t.Errorf("var[0x22] = %d, want 0 (IF_LT should have skipped)", h.vars[0x22])
	}
}

func TestVMArithmetic(t *testing.T) {
	h := newTestHost()

	// ADD #03, #05 -> result (3+5=8)
	code := []byte{
		0x04, // ADD
	}
	code = append(code, opLit(3)...)
	code = append(code, opLit(5)...)
	code = append(code, opMem(0x30)...)
	// SUB #03, #05 -> result (5-3=2)
	code = append(code, 0x05) // SUB
	code = append(code, opLit(3)...)
	code = append(code, opLit(5)...)
	code = append(code, opMem(0x32)...)
	// MUL #03, #05 -> result (3*5=15)
	code = append(code, 0x07) // MUL
	code = append(code, opLit(3)...)
	code = append(code, opLit(5)...)
	code = append(code, opMem(0x34)...)
	// DIV #02, #0A -> result (10/2=5)
	code = append(code, 0x06) // DIV
	code = append(code, opLit(2)...)
	code = append(code, opLit(10)...)
	code = append(code, opMem(0x36)...)
	code = append(code, 0x00) // EXIT

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	if h.vars[0x30] != 8 {
		t.Errorf("ADD: got %d, want 8", h.vars[0x30])
	}
	if h.vars[0x32] != 2 {
		t.Errorf("SUB: got %d, want 2", h.vars[0x32])
	}
	if h.vars[0x34] != 15 {
		t.Errorf("MUL: got %d, want 15", h.vars[0x34])
	}
	if h.vars[0x36] != 5 {
		t.Errorf("DIV: got %d, want 5", h.vars[0x36])
	}
}

func TestVMBitwiseOps(t *testing.T) {
	h := newTestHost()

	// AND #0F, #3F -> 0x0F
	code := []byte{
		0x2F, // AND
	}
	code = append(code, opLit(0x0F)...)
	code = append(code, opLit(0x3F)...)
	code = append(code, opMem(0x40)...)
	// OR #0F, #30 -> 0x3F
	code = append(code, 0x30) // OR
	code = append(code, opLit(0x0F)...)
	code = append(code, opLit(0x30)...)
	code = append(code, opMem(0x42)...)
	code = append(code, 0x00) // EXIT

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	if h.vars[0x40] != 0x0F {
		t.Errorf("AND: got %02X, want 0F", h.vars[0x40])
	}
	if h.vars[0x42] != 0x3F {
		t.Errorf("OR: got %02X, want 3F", h.vars[0x42])
	}
}

func TestVMRandom(t *testing.T) {
	h := newTestHost()
	h.randVal = 3

	// RANDOM #0A -> [mem:0050]
	code := []byte{
		0x08, // RANDOM
	}
	code = append(code, opLit(10)...)
	code = append(code, opMem(0x50)...)
	code = append(code, 0x00) // EXIT

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	if h.vars[0x50] != 3 {
		t.Errorf("RANDOM: got %d, want 3", h.vars[0x50])
	}
}

func TestVMNestedGosub(t *testing.T) {
	h := newTestHost()
	const base = 10

	// code[0-3]:   GOSUB -> sub1
	// code[4-9]:   SAVE #03 [0x54]
	// code[10-15]: SAVE #04 [0x56]
	// code[16]:    EXIT
	// sub1 at code[17]:
	// code[17-22]: SAVE #01 [0x50]
	// code[23-26]: GOSUB -> sub2
	// code[27]:    RETURN
	// sub2 at code[28]:
	// code[28-33]: SAVE #02 [0x52]
	// code[34]:    RETURN

	sub1Off := base + 17
	sub2Off := base + 28

	code := []byte{0x02}
	code = append(code, opWord(uint16(sub1Off))...)
	// after return from sub1:
	code = append(code, 0x09)
	code = append(code, opLit(3)...)
	code = append(code, opMem(0x54)...)
	code = append(code, 0x09)
	code = append(code, opLit(4)...)
	code = append(code, opMem(0x56)...)
	code = append(code, 0x00) // EXIT at code[16]
	// sub1 at code[17]:
	code = append(code, 0x09)
	code = append(code, opLit(1)...)
	code = append(code, opMem(0x50)...)
	code = append(code, 0x02) // GOSUB -> sub2
	code = append(code, opWord(uint16(sub2Off))...)
	code = append(code, 0x13) // RETURN at code[27]
	// sub2 at code[28]:
	code = append(code, 0x09)
	code = append(code, opLit(2)...)
	code = append(code, opMem(0x52)...)
	code = append(code, 0x13) // RETURN at code[34]

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	if h.vars[0x50] != 1 {
		t.Errorf("var[0x50] = %d, want 1", h.vars[0x50])
	}
	if h.vars[0x52] != 2 {
		t.Errorf("var[0x52] = %d, want 2", h.vars[0x52])
	}
	if h.vars[0x54] != 3 {
		t.Errorf("var[0x54] = %d, want 3", h.vars[0x54])
	}
	if h.vars[0x56] != 4 {
		t.Errorf("var[0x56] = %d, want 4", h.vars[0x56])
	}
}

func TestVMPrint(t *testing.T) {
	// Build bytecode with a PRINT of a compressed string
	// This uses operand code 0x80 with a known string
	// For simplicity, test with a literal string that we encode manually
	h := newTestHost()

	// PRINT with string operand code 0x80
	// 0x80, length, raw_bytes..., EXIT
	// Use "HELLO" encoded in the 6-bit format
	// H=0x08, E=0x05, L=0x0C, L=0x0C, O=0x0F (6-bit values + 0x40 = ASCII)
	// Actually, inflateChar adds 0x40 for values <= 0x1F
	// So the 6-bit values are: H=8, E=5, L=12, L=12, O=15
	// Packing 4 chars per 3 bytes:
	// chars: 8, 5, 12, 12
	// byte0 = (8<<2) | (5>>4) = 32 | 0 = 32 = 0x20
	// byte1 = (5<<4) | (12>>2) = 80 | 3 = 83 = 0x53 (wait: 5&0xF)<<4 = 0x50, (12>>2)&0xF = 3 => 0x53
	// byte2 = (12<<6) | 12 = (12&3)<<6 | 12 = 0xC0 | 12 = 0xCC (wait: lastByte<<2|b>>6)
	// Let me just use the decompress function to verify

	// Encode "HE" (2 chars from 2 bytes)
	// H = inflateChar(8) -> 'H' (0x48), so 6-bit value = 8
	// E = inflateChar(5) -> 'E' (0x45), so 6-bit value = 5
	// State 1: byte b, curr = (b>>2)&0x3F = 8 => b>>2 = 8 => b = 32+something
	//          If b = 0x22 (34), 34>>2 = 8, curr=8 ✓
	// State 2: lastByte=0x22, byte b
	//          curr = (lastByte<<4 | b>>4) & 0x3F = 5
	//          lastByte<<4 = 0x20, so we need b>>4 = 0x05-0x20... no that's wrong
	//          lastByte<<4 & 0x3F: 0x22<<4 = 0x220, &0x3F = 0x20. So we need b>>4 to contribute 5-0x20... no
	//          (0x220 | (b>>4)) & 0x3F = (0x20 | (b>>4)) & 0x3F = 5
	//          0x20 | (b>>4) = 5 => impossible since 0x20 is set
	// Hmm, the state machine is: curr = (lastByte<<4 | b>>4) & 0x3F
	// 0x22 = 0b00100010
	// lastByte<<4 = 0b001000100000 = 0x220
	// 0x220 & 0x3F = 0x20
	// So (0x20 | (b>>4)) & 0x3F needs to = 5
	// That means 0x20 | (b>>4) = 0x25 (since 0x25 & 0x3F = 0x25, not 5)
	// Wait, 0x20 | (b>>4) & 0x3F: the & applies to the whole expression
	// (0x220 | (b>>4)) & 0x3F = (0x20 | (b>>4)) (since 0x220 & 0x3F = 0x20)
	// We want this = 5, but 0x20 | x >= 0x20. Can't be 5.

	// I think the issue is I need to work backwards from the decompression.
	// Let me just test with the actual decompression function and verify.
	// For now, test PRINT with a simpler approach — just check it runs without error.

	code := []byte{
		0x11, // PRINT
		0x80, // string operand
		0x00, // length 0 (empty string)
		0x00, // EXIT
	}

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	// Empty string printed — just verify no crash
	if len(h.prints) != 1 {
		t.Errorf("expected 1 print call, got %d", len(h.prints))
	}
}

func TestVMNewECL(t *testing.T) {
	h := newTestHost()

	// NEWECL #05
	code := []byte{
		0x20, // NEWECL
	}
	code = append(code, opLit(5)...)
	code = append(code, 0x00) // EXIT (shouldn't be reached)

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	if !vm.stop {
		t.Error("VM should stop after NEWECL")
	}
	if !vm.reloadFlag {
		t.Error("reloadFlag should be set after NEWECL")
	}
	if vm.BlockID() != 5 {
		t.Errorf("blockID = %d, want 5", vm.BlockID())
	}
}

func TestVMCallStackEmptyReturn(t *testing.T) {
	h := newTestHost()

	// RETURN with empty stack should trigger EXIT
	code := []byte{
		0x13, // RETURN
	}

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	if !vm.stop {
		t.Error("RETURN on empty stack should exit VM")
	}
}

func TestVMPartyStr(t *testing.T) {
	h := newTestHost()

	// PARTYSTR -> [mem:0060]
	code := []byte{
		0x1D, // PARTYSTR
	}
	code = append(code, opMem(0x60)...)
	code = append(code, 0x00) // EXIT

	data := buildECL(nil, code)
	vm := NewVM(h, 0x01, data)
	vm.Run(10)

	if h.vars[0x60] != 42 {
		t.Errorf("var[0x60] = %d, want 42 (PartyStrength)", h.vars[0x60])
	}
}

func TestVMRunRealECL(t *testing.T) {
	// Load real ECL data and run a few instructions from InitialEntry
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	f, err := Open(filepath.Join(dir, "ecl1.dax"))
	if err != nil {
		t.Fatal(err)
	}

	entries := f.Entries()
	if len(entries) == 0 {
		t.Fatal("no entries in ecl1.dax")
	}

	// Decode first record and parse header
	raw := f.Decode(entries[0].ID)
	if raw == nil {
		t.Fatal("nil data")
	}

	hdr, _ := DisassembleECL(raw)
	if hdr.InitialEntry == 0 {
		t.Skip("no initial entry point")
	}

	h := newTestHost()
	vm := NewVM(h, entries[0].ID, raw)

	vm.Run(hdr.InitialEntry)
}
