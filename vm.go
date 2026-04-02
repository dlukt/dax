package dax

// Host provides game-state and I/O callbacks for the ECL VM.
// The VM itself is stateless with respect to the game world — all
// memory, rendering, and input go through this interface.
type Host interface {
	// Memory access
	GetVar(addr uint16) uint16
	SetVar(addr uint16, val uint16)

	// I/O
	Print(text string, clearFirst bool)
	Picture(id int)
	InputNumber(prompt string) int
	InputString(prompt string) string
	ShowMenu(items []string, vertical bool) int
	WaitKey()

	// Game state
	LoadCharacter(index int)
	Combat() bool
	Delay()
	SelectPlayer() int
	GetRandom(max int) int

	// Encounters
	LoadMonster(id, count, icon int)
	SetupMonster(sprite, distance, pic int)
	ClearMonsters()
	Approach()
	SpriteOff()

	// Meta
	NewECL(blockID byte)
	Program(op int)
	CallSub(id int)

	// Treasure/items
	Treasure(args [8]int)
	Rob(who, moneyPct, itemPct int)
	FindItem(itemType int) bool
	DestroyItems(itemType int)
	FindSpecial(affect int) bool

	// Queries
	PartyStrength() int
	CheckParty(args [6]int)
	PartySurprise(args [2]int) (bool, bool)
	Surprise(args [4]int)
	Spell(args [3]int) bool
	Damage(dice, count, target, a, b int)

	// Tables
	GetTable(table, index, count int) uint16
	SaveTable(table, index, count int, val uint16)

	// Misc
	Clock(hours, minutes int)
	AddNPC(id, morale int)
	LoadPieces(a, b, c int)
	LoadFiles(a, b, c int)
	Dump()
	ClearBox()
	Protection(check int)
	EncounterMenu(args [14]int)
	Parlay(args [6]int)
}

// VM executes ECL bytecode for a single script block.
type VM struct {
	host Host

	data []byte   // current ECL bytecode (the decoded record)
	off  uint16   // program counter (index into data)

	compareFlags [6]bool // EQ, NE, LT, GT, LE, GE
	callStack    []uint16
	stop         bool
	reloadFlag   bool // set by NEWECL
	blockID      byte

	ops        [0x40]Operand
	opCount    int
	strRegs    [15]string
	strIndex   int
	lastOpcode byte // current opcode being executed

}

// NewVM creates a VM for the given ECL bytecode block.
func NewVM(host Host, blockID byte, data []byte) *VM {
	return &VM{
		host:    host,
		data:    data,
		blockID: blockID,
	}
}

// BlockID returns the current ECL block ID.
func (vm *VM) BlockID() byte { return vm.blockID }

// ReloadFlag returns whether NEWECL was triggered (caller should reload).
func (vm *VM) ReloadFlag() bool { return vm.reloadFlag }

// Run executes bytecode starting from the given offset until EXIT or error.
const maxVMSteps = 100_000

func (vm *VM) Run(entry uint16) {
	vm.off = entry
	vm.stop = false
	vm.strIndex = 0

	steps := 0
	for !vm.stop {
		if int(vm.off) >= len(vm.data) {
			break
		}
		opcode := vm.data[vm.off]
		vm.off++

		if int(opcode) >= len(dispatchTable) || dispatchTable[opcode] == nil {
			continue
		}
		vm.lastOpcode = opcode
		dispatchTable[opcode](vm)

		steps++
		if steps >= maxVMSteps {
			break
		}
	}
}

// resolve returns the runtime value of an operand.
func (vm *VM) resolve(op Operand) uint16 {
	switch op.Code {
	case 0x00:
		return uint16(op.Low)
	case 0x02:
		return op.Word()
	case 0x01, 0x03:
		return vm.host.GetVar(op.Word())
	default:
		return uint16(op.Low)
	}
}

// resolveString returns the string associated with an operand (codes 0x80/0x81).
func (vm *VM) resolveString(op Operand) string {
	if op.Text != "" {
		return op.Text
	}
	return ""
}

// loadOps decodes n operand sets from the bytecode at the current offset.
func (vm *VM) loadOps(n int) {
	vm.strIndex = 0
	vm.opCount = 0

	for i := 0; i < n && int(vm.off) < len(vm.data); i++ {
		op, consumed := parseOperand(vm.data, int(vm.off))
		vm.off += uint16(consumed)
		if i < len(vm.ops) {
			vm.ops[i] = op
		}
		vm.opCount = i + 1

		// Track string registers
		if op.Code == 0x80 || op.Code == 0x81 {
			if vm.strIndex < len(vm.strRegs) {
				vm.strRegs[vm.strIndex] = op.Text
			}
			vm.strIndex++
		}
	}
}

// skipNext advances past the next instruction (used by IF opcodes).
func (vm *VM) skipNext() {
	if int(vm.off) >= len(vm.data) {
		return
	}
	opcode := vm.data[vm.off]
	vm.off++

	if int(opcode) >= len(opcodeOperandCount) {
		return
	}
	n := opcodeOperandCount[opcode]
	if n >= 0 {
		vm.loadOps(n)
		return
	}

	// Dynamic operand counts
	switch opcode {
	case 0x15: // VERTMENU
		vm.loadOps(3)
		if vm.opCount >= 3 {
			cnt := int(vm.ops[2].Low)
			vm.loadOpsDynamic(cnt)
		}
	case 0x25, 0x26: // ONGOTO/ONGOSUB
		vm.loadOps(2)
		if vm.opCount >= 2 {
			cnt := int(vm.ops[1].Low)
			vm.loadOpsDynamic(cnt)
		}
	case 0x2B: // HORIZMENU
		vm.loadOps(2)
		if vm.opCount >= 2 {
			cnt := int(vm.ops[1].Low)
			vm.loadOpsDynamic(cnt)
		}
	}
}

// loadOpsDynamic loads additional operands after the fixed set.
func (vm *VM) loadOpsDynamic(count int) {
	for i := 0; i < count && int(vm.off) < len(vm.data); i++ {
		_, consumed := parseOperand(vm.data, int(vm.off))
		vm.off += uint16(consumed)
	}
}

// opcodeOperandCount maps opcode byte to number of fixed operands.
// -1 means dynamic.
var opcodeOperandCount = [0x41]int{
	0x00: 0, // EXIT
	0x01: 1, // GOTO
	0x02: 1, // GOSUB
	0x03: 2, // COMPARE
	0x04: 3, // ADD
	0x05: 3, // SUB
	0x06: 3, // DIV
	0x07: 3, // MUL
	0x08: 2, // RANDOM
	0x09: 2, // SAVE
	0x0A: 1, // LOAD_CHAR
	0x0B: 3, // LOAD_MONST
	0x0C: 3, // SETUP_MONST
	0x0D: 0, // APPROACH
	0x0E: 1, // PICTURE
	0x0F: 2, // INPUT_NUM
	0x10: 2, // INPUT_STR
	0x11: 1, // PRINT
	0x12: 1, // PRINTCLR
	0x13: 0, // RETURN
	0x14: 4, // CMPAND
	0x15: -1, // VERTMENU
	0x16: 0, // IF_EQ
	0x17: 0, // IF_NE
	0x18: 0, // IF_LT
	0x19: 0, // IF_GT
	0x1A: 0, // IF_LE
	0x1B: 0, // IF_GE
	0x1C: 0, // CLRMONST
	0x1D: 1, // PARTYSTR
	0x1E: 6, // CHECKPARTY
	0x1F: 2, // UNKNOWN_1F
	0x20: 1, // NEWECL
	0x21: 3, // LOADFILES
	0x22: 2, // PARTYSURP
	0x23: 4, // SURPRISE
	0x24: 0, // COMBAT
	0x25: -1, // ONGOTO
	0x26: -1, // ONGOSUB
	0x27: 8, // TREASURE
	0x28: 3, // ROB
	0x29: 14, // ENCNTMENU
	0x2A: 3, // GETTABLE
	0x2B: -1, // HORIZMENU
	0x2C: 6, // PARLAY
	0x2D: 1, // CALL
	0x2E: 5, // DAMAGE
	0x2F: 3, // AND
	0x30: 3, // OR
	0x31: 0, // SPRITEOFF
	0x32: 1, // FINDITEM
	0x33: 0, // PRINTRET
	0x34: 2, // ECLCLOCK
	0x35: 3, // SAVETABLE
	0x36: 2, // ADDNPC
	0x37: 3, // LOADPIECES
	0x38: 1, // PROGRAM
	0x39: 1, // WHO
	0x3A: 0, // DELAY
	0x3B: 3, // SPELL
	0x3C: 1, // PROTECT
	0x3D: 0, // CLEARBOX
	0x3E: 0, // DUMP
	0x3F: 1, // FINDSPEC
	0x40: 1, // DESTROYITEM
}

// Opcode handlers

type opcodeHandler func(vm *VM)

var dispatchTable [0x41]opcodeHandler

func init() {
	dispatchTable[0x00] = (*VM).opExit
	dispatchTable[0x01] = (*VM).opGoto
	dispatchTable[0x02] = (*VM).opGosub
	dispatchTable[0x03] = (*VM).opCompare
	dispatchTable[0x04] = (*VM).opArith
	dispatchTable[0x05] = (*VM).opArith
	dispatchTable[0x06] = (*VM).opArith
	dispatchTable[0x07] = (*VM).opArith
	dispatchTable[0x08] = (*VM).opRandom
	dispatchTable[0x09] = (*VM).opSave
	dispatchTable[0x0A] = (*VM).opLoadChar
	dispatchTable[0x0B] = (*VM).opLoadMonst
	dispatchTable[0x0C] = (*VM).opSetupMonst
	dispatchTable[0x0D] = (*VM).opApproach
	dispatchTable[0x0E] = (*VM).opPicture
	dispatchTable[0x0F] = (*VM).opInputNum
	dispatchTable[0x10] = (*VM).opInputStr
	dispatchTable[0x11] = (*VM).opPrint
	dispatchTable[0x12] = (*VM).opPrint
	dispatchTable[0x13] = (*VM).opReturn
	dispatchTable[0x14] = (*VM).opCmpAnd
	dispatchTable[0x15] = (*VM).opVertMenu
	dispatchTable[0x16] = (*VM).opIf
	dispatchTable[0x17] = (*VM).opIf
	dispatchTable[0x18] = (*VM).opIf
	dispatchTable[0x19] = (*VM).opIf
	dispatchTable[0x1A] = (*VM).opIf
	dispatchTable[0x1B] = (*VM).opIf
	dispatchTable[0x1C] = (*VM).opClrMonst
	dispatchTable[0x1D] = (*VM).opPartyStr
	dispatchTable[0x1E] = (*VM).opCheckParty
	dispatchTable[0x20] = (*VM).opNewECL
	dispatchTable[0x21] = (*VM).opLoadFiles
	dispatchTable[0x22] = (*VM).opPartySurp
	dispatchTable[0x23] = (*VM).opSurprise
	dispatchTable[0x24] = (*VM).opCombat
	dispatchTable[0x25] = (*VM).opOnGoto
	dispatchTable[0x26] = (*VM).opOnGosub
	dispatchTable[0x27] = (*VM).opTreasure
	dispatchTable[0x28] = (*VM).opRob
	dispatchTable[0x29] = (*VM).opEncntMenu
	dispatchTable[0x2A] = (*VM).opGetTable
	dispatchTable[0x2B] = (*VM).opHorizMenu
	dispatchTable[0x2C] = (*VM).opParlay
	dispatchTable[0x2D] = (*VM).opCall
	dispatchTable[0x2E] = (*VM).opDamage
	dispatchTable[0x2F] = (*VM).opAnd
	dispatchTable[0x30] = (*VM).opOr
	dispatchTable[0x31] = (*VM).opSpriteOff
	dispatchTable[0x32] = (*VM).opFindItem
	dispatchTable[0x33] = (*VM).opPrintRet
	dispatchTable[0x34] = (*VM).opECLClock
	dispatchTable[0x35] = (*VM).opSaveTable
	dispatchTable[0x36] = (*VM).opAddNPC
	dispatchTable[0x37] = (*VM).opLoadPieces
	dispatchTable[0x38] = (*VM).opProgram
	dispatchTable[0x39] = (*VM).opWho
	dispatchTable[0x3A] = (*VM).opDelay
	dispatchTable[0x3B] = (*VM).opSpell
	dispatchTable[0x3C] = (*VM).opProtect
	dispatchTable[0x3D] = (*VM).opClearBox
	dispatchTable[0x3E] = (*VM).opDump
	dispatchTable[0x3F] = (*VM).opFindSpec
	dispatchTable[0x40] = (*VM).opDestroyItem
}

// 0x00 EXIT
func (vm *VM) opExit() {
	vm.callStack = vm.callStack[:0]
	vm.stop = true
}

// 0x01 GOTO
func (vm *VM) opGoto() {
	vm.loadOps(1)
	vm.off = vm.resolve(vm.ops[0])
}

// 0x02 GOSUB
func (vm *VM) opGosub() {
	vm.loadOps(1)
	target := vm.resolve(vm.ops[0])
	vm.callStack = append(vm.callStack, vm.off)
	vm.off = target
}

// 0x03 COMPARE
func (vm *VM) opCompare() {
	vm.loadOps(2)

	// String comparison if either operand is a string
	if vm.ops[0].Code >= 0x80 || vm.ops[1].Code >= 0x80 {
		a := vm.resolveString(vm.ops[0])
		b := vm.resolveString(vm.ops[1])
		vm.compareStrings(b, a)
	} else {
		a := vm.resolve(vm.ops[0])
		b := vm.resolve(vm.ops[1])
		vm.compareVars(b, a)
	}
}

// 0x04-0x07 ADD/SUB/DIV/MUL
func (vm *VM) opArith() {
	vm.loadOps(3)
	// The current opcode is in vm.data[vm.off-1], but we need to know
	// which arithmetic op. Read from the instruction byte saved before loadOps.
	// We reconstruct: the opcode byte was read in Run(), and we don't save it.
	// Instead, look at the first operand code pattern — all arith use 3 operands.
	// We need the opcode. Let's use a different approach: store it.
	a := vm.resolve(vm.ops[0])
	b := vm.resolve(vm.ops[1])
	dst := vm.ops[2]

	// The opcode was the byte before the operands. We saved it in vm.lastOpcode.
	var result uint16
	switch vm.lastOpcode {
	case 0x04: // ADD
		result = a + b
	case 0x05: // SUB
		result = b - a
	case 0x06: // DIV
		if a != 0 {
			result = b / a
		}
	case 0x07: // MUL
		result = a * b
	}

	vm.saveResult(dst, result)
}

// 0x08 RANDOM
func (vm *VM) opRandom() {
	vm.loadOps(2)
	max := vm.resolve(vm.ops[0])
	dst := vm.ops[1]
	var val uint16
	if max > 0 {
		val = uint16(vm.host.GetRandom(int(max)))
	}
	vm.saveResult(dst, val)
}

// 0x09 SAVE
func (vm *VM) opSave() {
	vm.loadOps(2)
	val := vm.resolve(vm.ops[0])
	dst := vm.ops[1]
	vm.saveResult(dst, val)
}

// 0x0A LOAD_CHAR
func (vm *VM) opLoadChar() {
	vm.loadOps(1)
	idx := vm.resolve(vm.ops[0])
	vm.host.LoadCharacter(int(idx))
}

// 0x0B LOAD_MONST
func (vm *VM) opLoadMonst() {
	vm.loadOps(3)
	id := vm.resolve(vm.ops[0])
	count := vm.resolve(vm.ops[1])
	icon := vm.resolve(vm.ops[2])
	vm.host.LoadMonster(int(id), int(count), int(icon))
}

// 0x0C SETUP_MONST
func (vm *VM) opSetupMonst() {
	vm.loadOps(3)
	sprite := vm.resolve(vm.ops[0])
	distance := vm.resolve(vm.ops[1])
	pic := vm.resolve(vm.ops[2])
	vm.host.SetupMonster(int(sprite), int(distance), int(pic))
}

// 0x0D APPROACH
func (vm *VM) opApproach() {
	vm.host.Approach()
}

// 0x0E PICTURE
func (vm *VM) opPicture() {
	vm.loadOps(1)
	id := vm.resolve(vm.ops[0])
	vm.host.Picture(int(id))
}

// 0x0F INPUT_NUM
func (vm *VM) opInputNum() {
	vm.loadOps(2)
	// op[0] is prompt text (string operand), op[1] is destination
	prompt := vm.resolveString(vm.ops[0])
	val := vm.host.InputNumber(prompt)
	vm.saveResult(vm.ops[1], uint16(val))
}

// 0x10 INPUT_STR
func (vm *VM) opInputStr() {
	vm.loadOps(2)
	prompt := vm.resolveString(vm.ops[0])
	_ = vm.host.InputString(prompt)
	// String result stored in memory — host handles it
}

// 0x11/0x12 PRINT/PRINTCLR
func (vm *VM) opPrint() {
	vm.loadOps(1)
	text := vm.resolveString(vm.ops[0])
	clearFirst := vm.lastOpcode == 0x12
	vm.host.Print(text, clearFirst)
}

// 0x13 RETURN
func (vm *VM) opReturn() {
	if len(vm.callStack) > 0 {
		n := len(vm.callStack) - 1
		vm.off = vm.callStack[n]
		vm.callStack = vm.callStack[:n]
	} else {
		vm.opExit()
	}
}

// 0x14 CMPAND — double equality comparison
func (vm *VM) opCmpAnd() {
	vm.loadOps(4)
	a := vm.resolve(vm.ops[0])
	b := vm.resolve(vm.ops[1])
	c := vm.resolve(vm.ops[2])
	d := vm.resolve(vm.ops[3])

	if a == b && c == d {
		vm.compareFlags[0] = true  // EQ
		vm.compareFlags[1] = false // NE
	} else {
		vm.compareFlags[0] = false
		vm.compareFlags[1] = true
	}
}

// 0x15 VERTMENU
func (vm *VM) opVertMenu() {
	vm.loadOps(3)
	if vm.opCount < 3 {
		return
	}
	menuCount := int(vm.ops[2].Low)

	items := make([]string, menuCount)
	for i := 0; i < menuCount && int(vm.off) < len(vm.data); i++ {
		op, consumed := parseOperand(vm.data, int(vm.off))
		vm.off += uint16(consumed)
		items[i] = vm.resolveString(op)
	}

	_ = vm.host.ShowMenu(items, true)
	// Result goes to memory — host stores selection
}

// 0x16-0x1B IF_EQ through IF_GE
func (vm *VM) opIf() {
	flagIdx := int(vm.lastOpcode) - 0x16
	if flagIdx < 0 || flagIdx >= 6 {
		return
	}
	if !vm.compareFlags[flagIdx] {
		vm.skipNext()
	}
}

// 0x1C CLRMONST
func (vm *VM) opClrMonst() {
	vm.host.ClearMonsters()
}

// 0x1D PARTYSTR
func (vm *VM) opPartyStr() {
	vm.loadOps(1)
	dst := vm.ops[0]
	val := uint16(vm.host.PartyStrength())
	vm.saveResult(dst, val)
}

// 0x1E CHECKPARTY
func (vm *VM) opCheckParty() {
	vm.loadOps(6)
	var args [6]int
	for i := 0; i < 6; i++ {
		args[i] = int(vm.resolve(vm.ops[i]))
	}
	vm.host.CheckParty(args)
}

// 0x20 NEWECL
func (vm *VM) opNewECL() {
	vm.loadOps(1)
	blockID := byte(vm.resolve(vm.ops[0]))
	vm.blockID = blockID
	vm.host.NewECL(blockID)
	vm.stop = true
	vm.reloadFlag = true
}

// 0x21 LOADFILES
func (vm *VM) opLoadFiles() {
	vm.loadOps(3)
	a := vm.resolve(vm.ops[0])
	b := vm.resolve(vm.ops[1])
	c := vm.resolve(vm.ops[2])
	vm.host.LoadFiles(int(a), int(b), int(c))
}

// 0x22 PARTYSURP
func (vm *VM) opPartySurp() {
	vm.loadOps(2)
	a := vm.resolve(vm.ops[0])
	b := vm.resolve(vm.ops[1])
	vm.host.PartySurprise([2]int{int(a), int(b)})
}

// 0x23 SURPRISE
func (vm *VM) opSurprise() {
	vm.loadOps(4)
	var args [4]int
	for i := 0; i < 4; i++ {
		args[i] = int(vm.resolve(vm.ops[i]))
	}
	vm.host.Surprise(args)
}

// 0x24 COMBAT
func (vm *VM) opCombat() {
	vm.host.Combat()
}

// 0x25 ONGOTO
func (vm *VM) opOnGoto() {
	vm.loadOps(2)
	if vm.opCount < 2 {
		return
	}
	index := vm.resolve(vm.ops[0])
	tableSize := int(vm.ops[1].Low)

	table := make([]uint16, tableSize)
	for i := 0; i < tableSize && int(vm.off) < len(vm.data); i++ {
		op, consumed := parseOperand(vm.data, int(vm.off))
		vm.off += uint16(consumed)
		table[i] = vm.resolve(op)
	}

	if index < uint16(tableSize) {
		vm.off = table[index]
	}
}

// 0x26 ONGOSUB
func (vm *VM) opOnGosub() {
	vm.loadOps(2)
	if vm.opCount < 2 {
		return
	}
	index := vm.resolve(vm.ops[0])
	tableSize := int(vm.ops[1].Low)

	table := make([]uint16, tableSize)
	for i := 0; i < tableSize && int(vm.off) < len(vm.data); i++ {
		op, consumed := parseOperand(vm.data, int(vm.off))
		vm.off += uint16(consumed)
		table[i] = vm.resolve(op)
	}

	if index < uint16(tableSize) {
		vm.callStack = append(vm.callStack, vm.off)
		vm.off = table[index]
	}
}

// 0x27 TREASURE
func (vm *VM) opTreasure() {
	vm.loadOps(8)
	var args [8]int
	for i := 0; i < 8; i++ {
		args[i] = int(vm.resolve(vm.ops[i]))
	}
	vm.host.Treasure(args)
}

// 0x28 ROB
func (vm *VM) opRob() {
	vm.loadOps(3)
	who := vm.resolve(vm.ops[0])
	moneyPct := vm.resolve(vm.ops[1])
	itemPct := vm.resolve(vm.ops[2])
	vm.host.Rob(int(who), int(moneyPct), int(itemPct))
}

// 0x29 ENCNTMENU
func (vm *VM) opEncntMenu() {
	vm.loadOps(14)
	var args [14]int
	for i := 0; i < 14; i++ {
		args[i] = int(vm.resolve(vm.ops[i]))
	}
	vm.host.EncounterMenu(args)
}

// 0x2A GETTABLE
func (vm *VM) opGetTable() {
	vm.loadOps(3)
	table := vm.resolve(vm.ops[0])
	index := vm.resolve(vm.ops[1])
	count := vm.resolve(vm.ops[2])
	dst := vm.ops[2] // third operand is also the destination per coab
	val := vm.host.GetTable(int(table), int(index), int(count))
	vm.saveResult(dst, val)
}

// 0x2B HORIZMENU
func (vm *VM) opHorizMenu() {
	vm.loadOps(2)
	if vm.opCount < 2 {
		return
	}
	itemCount := int(vm.ops[1].Low)

	items := make([]string, itemCount)
	for i := 0; i < itemCount && int(vm.off) < len(vm.data); i++ {
		op, consumed := parseOperand(vm.data, int(vm.off))
		vm.off += uint16(consumed)
		items[i] = vm.resolveString(op)
	}

	_ = vm.host.ShowMenu(items, false)
}

// 0x2C PARLAY
func (vm *VM) opParlay() {
	vm.loadOps(6)
	var args [6]int
	for i := 0; i < 6; i++ {
		args[i] = int(vm.resolve(vm.ops[i]))
	}
	vm.host.Parlay(args)
}

// 0x2D CALL
func (vm *VM) opCall() {
	vm.loadOps(1)
	id := vm.resolve(vm.ops[0])
	vm.host.CallSub(int(id))
}

// 0x2E DAMAGE
func (vm *VM) opDamage() {
	vm.loadOps(5)
	dice := vm.resolve(vm.ops[0])
	count := vm.resolve(vm.ops[1])
	target := vm.resolve(vm.ops[2])
	a := vm.resolve(vm.ops[3])
	b := vm.resolve(vm.ops[4])
	vm.host.Damage(int(dice), int(count), int(target), int(a), int(b))
}

// 0x2F AND
func (vm *VM) opAnd() {
	vm.loadOps(3)
	a := vm.resolve(vm.ops[0])
	b := vm.resolve(vm.ops[1])
	dst := vm.ops[2]
	vm.saveResult(dst, a&b)
}

// 0x30 OR
func (vm *VM) opOr() {
	vm.loadOps(3)
	a := vm.resolve(vm.ops[0])
	b := vm.resolve(vm.ops[1])
	dst := vm.ops[2]
	vm.saveResult(dst, a|b)
}

// 0x31 SPRITEOFF
func (vm *VM) opSpriteOff() {
	vm.host.SpriteOff()
}

// 0x32 FINDITEM
func (vm *VM) opFindItem() {
	vm.loadOps(1)
	itemType := vm.resolve(vm.ops[0])
	found := vm.host.FindItem(int(itemType))
	// Result stored in compare flags per coab
	vm.compareFlags[0] = found
	vm.compareFlags[1] = !found
}

// 0x33 PRINTRET
func (vm *VM) opPrintRet() {
	vm.host.Print("", false)
}

// 0x34 ECLCLOCK
func (vm *VM) opECLClock() {
	vm.loadOps(2)
	hours := vm.resolve(vm.ops[0])
	minutes := vm.resolve(vm.ops[1])
	vm.host.Clock(int(hours), int(minutes))
}

// 0x35 SAVETABLE
func (vm *VM) opSaveTable() {
	vm.loadOps(3)
	table := vm.resolve(vm.ops[0])
	index := vm.resolve(vm.ops[1])
	val := vm.resolve(vm.ops[2])
	vm.host.SaveTable(int(table), int(index), int(val), val)
}

// 0x36 ADDNPC
func (vm *VM) opAddNPC() {
	vm.loadOps(2)
	id := vm.resolve(vm.ops[0])
	morale := vm.resolve(vm.ops[1])
	vm.host.AddNPC(int(id), int(morale))
}

// 0x37 LOADPIECES
func (vm *VM) opLoadPieces() {
	vm.loadOps(3)
	a := vm.resolve(vm.ops[0])
	b := vm.resolve(vm.ops[1])
	c := vm.resolve(vm.ops[2])
	vm.host.LoadPieces(int(a), int(b), int(c))
}

// 0x38 PROGRAM
func (vm *VM) opProgram() {
	vm.loadOps(1)
	op := vm.resolve(vm.ops[0])
	vm.host.Program(int(op))
}

// 0x39 WHO
func (vm *VM) opWho() {
	vm.loadOps(1)
	dst := vm.ops[0]
	val := vm.host.SelectPlayer()
	vm.saveResult(dst, uint16(val))
}

// 0x3A DELAY
func (vm *VM) opDelay() {
	vm.host.Delay()
}

// 0x3B SPELL
func (vm *VM) opSpell() {
	vm.loadOps(3)
	var args [3]int
	for i := 0; i < 3; i++ {
		args[i] = int(vm.resolve(vm.ops[i]))
	}
	found := vm.host.Spell(args)
	vm.compareFlags[0] = found
	vm.compareFlags[1] = !found
}

// 0x3C PROTECT
func (vm *VM) opProtect() {
	vm.loadOps(1)
	check := vm.resolve(vm.ops[0])
	vm.host.Protection(int(check))
}

// 0x3D CLEARBOX
func (vm *VM) opClearBox() {
	vm.host.ClearBox()
}

// 0x3E DUMP
func (vm *VM) opDump() {
	vm.host.Dump()
}

// 0x3F FINDSPEC
func (vm *VM) opFindSpec() {
	vm.loadOps(1)
	affect := vm.resolve(vm.ops[0])
	found := vm.host.FindSpecial(int(affect))
	vm.compareFlags[0] = found
	vm.compareFlags[1] = !found
}

// 0x40 DESTROYITEM
func (vm *VM) opDestroyItem() {
	vm.loadOps(1)
	itemType := vm.resolve(vm.ops[0])
	vm.host.DestroyItems(int(itemType))
}

// --- helpers ---

// compareVars sets all 6 compare flags for two values.
func (vm *VM) compareVars(a, b uint16) {
	vm.compareFlags[0] = a == b // EQ
	vm.compareFlags[1] = a != b // NE
	vm.compareFlags[2] = a < b  // LT
	vm.compareFlags[3] = a > b  // GT
	vm.compareFlags[4] = a <= b // LE
	vm.compareFlags[5] = a >= b // GE
}

// compareStrings sets compare flags for two strings.
func (vm *VM) compareStrings(a, b string) {
	vm.compareFlags[0] = a == b
	vm.compareFlags[1] = a != b
	vm.compareFlags[2] = a < b
	vm.compareFlags[3] = a > b
	vm.compareFlags[4] = a <= b
	vm.compareFlags[5] = a >= b
}

// saveResult writes a value to the destination specified by an operand.
func (vm *VM) saveResult(dst Operand, val uint16) {
	switch dst.Code {
	case 0x01, 0x03:
		vm.host.SetVar(dst.Word(), val)
	}
	// Code 0x00 (literal) is not a valid destination — silently ignore
}
