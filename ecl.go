package dax

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Opcode definitions for the ECL virtual machine.
var opcodes = [0x41]opcodeDef{
	{0x00, "EXIT", 0, "Stop VM, clear call stack"},
	{0x01, "GOTO", 1, "Unconditional jump to address"},
	{0x02, "GOSUB", 1, "Call subroutine"},
	{0x03, "COMPARE", 2, "Compare A vs B, set flags"},
	{0x04, "ADD", 3, "A + B -> result"},
	{0x05, "SUB", 3, "B - A -> result"},
	{0x06, "DIV", 3, "A / B -> result"},
	{0x07, "MUL", 3, "A * B -> result"},
	{0x08, "RANDOM", 2, "Random 0..max -> result"},
	{0x09, "SAVE", 2, "Write value to memory"},
	{0x0A, "LOAD_CHAR", 1, "Select player by index"},
	{0x0B, "LOAD_MONST", 3, "Load monster(s) by ID, count, icon"},
	{0x0C, "SETUP_MONST", 3, "Set encounter sprite, distance, pic"},
	{0x0D, "APPROACH", 0, "Decrement encounter distance"},
	{0x0E, "PICTURE", 1, "Display picture/bigpic"},
	{0x0F, "INPUT_NUM", 2, "Prompt for number -> memory"},
	{0x10, "INPUT_STR", 2, "Prompt for string -> memory"},
	{0x11, "PRINT", 1, "Print text + wait for key"},
	{0x12, "PRINTCLR", 1, "Clear area then print text"},
	{0x13, "RETURN", 0, "Return from subroutine"},
	{0x14, "CMPAND", 4, "Double compare: (A==B && C==D)"},
	{0x15, "VERTMENU", -1, "Vertical menu (dynamic)"},
	{0x16, "IF_EQ", 0, "If == flag false, skip next"},
	{0x17, "IF_NE", 0, "If != flag false, skip next"},
	{0x18, "IF_LT", 0, "If < flag false, skip next"},
	{0x19, "IF_GT", 0, "If > flag false, skip next"},
	{0x1A, "IF_LE", 0, "If <= flag false, skip next"},
	{0x1B, "IF_GE", 0, "If >= flag false, skip next"},
	{0x1C, "CLRMONST", 0, "Clear all loaded monsters"},
	{0x1D, "PARTYSTR", 1, "Calculate party combat strength"},
	{0x1E, "CHECKPARTY", 6, "Check party affects/skills"},
	{0x1F, "UNKNOWN_1F", 2, "Unimplemented"},
	{0x20, "NEWECL", 1, "Load new ECL script block"},
	{0x21, "LOADFILES", 3, "Load area resources"},
	{0x22, "PARTYSURP", 2, "Check party ranger/surprise"},
	{0x23, "SURPRISE", 4, "Roll surprise for both sides"},
	{0x24, "COMBAT", 0, "Enter combat"},
	{0x25, "ONGOTO", -1, "Computed goto (dynamic)"},
	{0x26, "ONGOSUB", -1, "Computed gosub (dynamic)"},
	{0x27, "TREASURE", 8, "Generate treasure"},
	{0x28, "ROB", 3, "Rob money/items from player(s)"},
	{0x29, "ENCNTMENU", 14, "Full encounter menu"},
	{0x2A, "GETTABLE", 3, "Read from memory table"},
	{0x2B, "HORIZMENU", -1, "Horizontal menu (dynamic)"},
	{0x2C, "PARLAY", 6, "Parley menu"},
	{0x2D, "CALL", 1, "Call native engine subroutine"},
	{0x2E, "DAMAGE", 5, "Deal dice-based damage"},
	{0x2F, "AND", 3, "Bitwise AND"},
	{0x30, "OR", 3, "Bitwise OR"},
	{0x31, "SPRITEOFF", 0, "Turn off encounter sprite"},
	{0x32, "FINDITEM", 1, "Check if party has item type"},
	{0x33, "PRINTRET", 0, "Advance text cursor (newline)"},
	{0x34, "ECLCLOCK", 2, "Advance game clock"},
	{0x35, "SAVETABLE", 3, "Write to memory table"},
	{0x36, "ADDNPC", 2, "Load NPC with morale"},
	{0x37, "LOADPIECES", 3, "Load wall definitions"},
	{0x38, "PROGRAM", 1, "Meta: menu/win/die/encamp"},
	{0x39, "WHO", 1, "Prompt select player"},
	{0x3A, "DELAY", 0, "Brief pause"},
	{0x3B, "SPELL", 3, "Search party for spell"},
	{0x3C, "PROTECT", 1, "Copy protection check"},
	{0x3D, "CLEARBOX", 0, "Clear text box"},
	{0x3E, "DUMP", 0, "Refresh party summary"},
	{0x3F, "FINDSPEC", 1, "Check player affect"},
	{0x40, "DESTROYITEM", 1, "Remove items by type"},
}

type opcodeDef struct {
	Opcode   byte
	Name     string
	Operands int // -1 = dynamic
	Comment  string
}

// Operand represents one parsed operand from ECL bytecode.
type Operand struct {
	Code byte
	Low  byte
	High byte
	Text string
}

func (op Operand) String() string {
	switch op.Code {
	case 0x00:
		return fmt.Sprintf("#%02X", op.Low)
	case 0x01:
		return fmt.Sprintf("[mem:%04X]", uint16(op.Low)|uint16(op.High)<<8)
	case 0x02:
		return fmt.Sprintf("$%04X", uint16(op.Low)|uint16(op.High)<<8)
	case 0x03:
		return fmt.Sprintf("[var:%04X]", uint16(op.Low)|uint16(op.High)<<8)
	case 0x80:
		if op.Text != "" {
			return fmt.Sprintf("\"%s\"", truncate(op.Text, 40))
		}
		return fmt.Sprintf("str(len=%d)", op.Low)
	case 0x81:
		return fmt.Sprintf("str[@%04X]", uint16(op.Low)|uint16(op.High)<<8)
	default:
		return fmt.Sprintf("#%02X", op.Low)
	}
}

// Word returns the 16-bit value composed of Low and High bytes.
func (op Operand) Word() uint16 {
	return uint16(op.Low) | uint16(op.High)<<8
}

// resolveValue returns the immediate byte value if this is a literal operand.
func (op Operand) resolveValue() byte {
	return op.Low
}

// Instruction represents one disassembled ECL instruction.
type Instruction struct {
	Addr     uint16
	Opcode   byte
	Name     string
	Operands []Operand
	Comment  string
	Bytes    []byte
}

func (inst Instruction) String() string {
	ops := make([]string, len(inst.Operands))
	for i, op := range inst.Operands {
		ops[i] = op.String()
	}
	s := fmt.Sprintf("%04X: %-12s", inst.Addr&0x7FFF, inst.Name)
	if len(ops) > 0 {
		s += " " + strings.Join(ops, ", ")
	}
	if inst.Comment != "" {
		s += "\t; " + inst.Comment
	}
	return s
}

// ECLHeader holds the 5 entry point addresses from the start of an ECL block.
type ECLHeader struct {
	RunAddr         uint16
	SearchLocation  uint16
	PreCampCheck    uint16
	CampInterrupted uint16
	InitialEntry    uint16
}

// inflateChar maps a 6-bit ECL character code to a printable character.
// Values 0x00-0x1F map to uppercase A-Z and symbols (offset by 0x40).
// Values >= 0x20 map directly to ASCII.
func inflateChar(v uint16) byte {
	if v <= 0x1F {
		v += 0x40
	}
	return byte(v)
}

// decompressECLString decodes a 6-bit packed string from ECL bytecode.
// Every 3 bytes produce 4 characters via the same packing scheme used
// by the Gold Box engine's DecompressString function.
func decompressECLString(raw []byte) string {
	var sb strings.Builder
	state := 1
	var lastByte byte

	for _, b := range raw {
		var curr uint16
		switch state {
		case 1:
			curr = uint16(b>>2) & 0x3F
			if curr != 0 {
				sb.WriteByte(inflateChar(curr))
			}
			state = 2
		case 2:
			curr = uint16(lastByte<<4|b>>4) & 0x3F
			if curr != 0 {
				sb.WriteByte(inflateChar(curr))
			}
			state = 3
		case 3:
			curr = uint16(lastByte<<2|b>>6) & 0x3F
			if curr != 0 {
				sb.WriteByte(inflateChar(curr))
			}
			curr = uint16(b) & 0x3F
			if curr != 0 {
				sb.WriteByte(inflateChar(curr))
			}
			state = 1
		}
		lastByte = b
	}
	return sb.String()
}

// DisassembleECL disassembles all instructions from a decompressed ECL record.
func DisassembleECL(data []byte) (ECLHeader, []Instruction) {
	if len(data) < 10 {
		return ECLHeader{}, nil
	}

	hdr := ECLHeader{
		RunAddr:         binary.LittleEndian.Uint16(data[0:2]),
		SearchLocation:  binary.LittleEndian.Uint16(data[2:4]),
		PreCampCheck:    binary.LittleEndian.Uint16(data[4:6]),
		CampInterrupted: binary.LittleEndian.Uint16(data[6:8]),
		InitialEntry:    binary.LittleEndian.Uint16(data[8:10]),
	}

	var insts []Instruction
	off := 0

	for off < len(data)-1 {
		instStart := off
		opcode := data[off]
		off++

		if int(opcode) >= len(opcodes) || opcodes[opcode].Name == "" {
			insts = append(insts, Instruction{
				Addr:   uint16(0x8000 + instStart),
				Opcode: opcode,
				Name:   fmt.Sprintf("DB_%02X", opcode),
				Bytes:  data[instStart : instStart+1],
			})
			continue
		}

		def := opcodes[opcode]
		numOps := def.Operands

		var operands []Operand

		for i := 0; i < numOps && off < len(data)-1; i++ {
			op, consumed := parseOperand(data, off)
			operands = append(operands, op)
			off += consumed
		}

		inst := Instruction{
			Addr:     uint16(0x8000 + instStart),
			Opcode:   opcode,
			Name:     def.Name,
			Operands: operands,
			Comment:  def.Comment,
		}

		// Handle dynamic operand counts
		switch opcode {
		case 0x15: // VERTMENU: 3 fixed + menuCount dynamic
			if len(operands) >= 3 {
				menuCount := int(operands[2].resolveValue())
				for i := 0; i < menuCount && off < len(data)-1; i++ {
					op, consumed := parseOperand(data, off)
					operands = append(operands, op)
					off += consumed
				}
				inst.Operands = operands
			}
		case 0x25, 0x26: // ONGOTO/ONGOSUB: 2 fixed + N dynamic
			if len(operands) >= 2 {
				tableSize := int(operands[1].resolveValue())
				for i := 0; i < tableSize && off < len(data)-1; i++ {
					op, consumed := parseOperand(data, off)
					operands = append(operands, op)
					off += consumed
				}
				inst.Operands = operands
			}
		case 0x2B: // HORIZMENU: 2 fixed + N dynamic
			if len(operands) >= 2 {
				itemCount := int(operands[1].resolveValue())
				for i := 0; i < itemCount && off < len(data)-1; i++ {
					op, consumed := parseOperand(data, off)
					operands = append(operands, op)
					off += consumed
				}
				inst.Operands = operands
			}
		}

		if off > len(data) {
			off = len(data)
		}
		inst.Bytes = data[instStart:off]
		insts = append(insts, inst)
	}

	return hdr, insts
}

// parseOperand parses one operand set starting at off.
func parseOperand(data []byte, off int) (Operand, int) {
	if off >= len(data) {
		return Operand{}, 0
	}
	if off+1 >= len(data) {
		return Operand{Code: data[off]}, 1
	}

	code := data[off]
	low := data[off+1]
	op := Operand{Code: code, Low: low}

	switch {
	case code == 0x01 || code == 0x02 || code == 0x03:
		if off+2 < len(data) {
			op.High = data[off+2]
		}
		return op, 3

	case code == 0x80:
		// Compressed string: low = byte length, data is 6-bit packed chars
		rawLen := int(low)
		available := len(data) - (off + 2)
		if rawLen > available {
			rawLen = available
		}
		if rawLen > 0 {
			op.Text = decompressECLString(data[off+2 : off+2+rawLen])
		}
		return op, 2 + rawLen

	case code == 0x81:
		if off+2 < len(data) {
			op.High = data[off+2]
		}
		return op, 3

	default:
		return op, 2
	}
}

// DisassembleAllECL disassembles all records from an ECL DAX file.
func (f *File) DisassembleAllECL() map[byte]struct {
	Header ECLHeader
	Code   []Instruction
} {
	result := make(map[byte]struct {
		Header ECLHeader
		Code   []Instruction
	})

	all := f.DecodeAll()
	for id, raw := range all {
		hdr, insts := DisassembleECL(raw)
		result[id] = struct {
			Header ECLHeader
			Code   []Instruction
		}{hdr, insts}
	}
	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
