package dax

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

// TerminalHost wraps a GameState and provides real terminal I/O
// for the ECL VM. Text output goes to stdout, input reads from stdin.
type TerminalHost struct {
	*GameState
	reader *bufio.Reader
	rng    *rand.Rand
}

// NewTerminalHost creates a Host with terminal I/O backed by the given GameState.
func NewTerminalHost(gs *GameState) *TerminalHost {
	return &TerminalHost{
		GameState: gs,
		reader:    bufio.NewReader(os.Stdin),
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Print displays text. If clearFirst, clear the console first.
// After printing, wait for a keypress (matching original engine behavior).
func (t *TerminalHost) Print(text string, clearFirst bool) {
	if clearFirst {
		fmt.Print("\033[2J\033[H") // clear screen
	}
	if text != "" {
		fmt.Println(text)
	}
	// Original engine waits for key after PRINT
	fmt.Print("  [Press Enter]")
	t.reader.ReadString('\n')
}

// Picture displays a picture (stub — no terminal rendering for images).
func (t *TerminalHost) Picture(id int) {
	fmt.Printf("[Picture %d]\n", id)
}

// InputNumber prompts for and reads an integer.
func (t *TerminalHost) InputNumber(prompt string) int {
	if prompt != "" {
		fmt.Print(prompt)
		fmt.Print(" ")
	}
	for {
		fmt.Print("> ")
		line, _ := t.reader.ReadString('\n')
		line = strings.TrimSpace(line)
		var n int
		if _, err := fmt.Sscanf(line, "%d", &n); err == nil {
			return n
		}
		fmt.Println("Enter a number.")
	}
}

// InputString prompts for and reads a string.
func (t *TerminalHost) InputString(prompt string) string {
	if prompt != "" {
		fmt.Print(prompt)
		fmt.Print(" ")
	}
	fmt.Print("> ")
	line, _ := t.reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// ShowMenu presents a vertical or horizontal menu and returns the selected index (0-based).
func (t *TerminalHost) ShowMenu(items []string, vertical bool) int {
	if len(items) == 0 {
		return 0
	}

	fmt.Println()
	for i, item := range items {
		if vertical {
			fmt.Printf("  %d) %s\n", i+1, item)
		} else {
			if i > 0 {
				fmt.Print("  ")
			}
			fmt.Printf("%d) %s", i+1, item)
		}
	}
	if !vertical {
		fmt.Println()
	}
	fmt.Println()

	for {
		fmt.Print("Choose > ")
		line, _ := t.reader.ReadString('\n')
		line = strings.TrimSpace(line)
		var n int
		if _, err := fmt.Sscanf(line, "%d", &n); err == nil && n >= 1 && n <= len(items) {
			return n - 1
		}
		fmt.Printf("Enter 1-%d.\n", len(items))
	}
}

// WaitKey waits for a keypress.
func (t *TerminalHost) WaitKey() {
	fmt.Print("  [Press Enter]")
	t.reader.ReadString('\n')
}

// Delay pauses briefly.
func (t *TerminalHost) Delay() {
	time.Sleep(500 * time.Millisecond)
}

// SelectPlayer prompts the user to choose a party member.
func (t *TerminalHost) SelectPlayer() int {
	fmt.Println("Select a party member:")
	for i, p := range t.Players {
		name := strings.TrimRight(string(p.Raw[:15]), "\x00")
		if name == "" {
			name = fmt.Sprintf("Player %d", i+1)
		}
		fmt.Printf("  %d) %s (HP %d/%d)\n", i+1, name, p.Raw[0x1A4], p.Raw[0x78])
	}
	for {
		fmt.Print("Choose > ")
		line, _ := t.reader.ReadString('\n')
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(line), "%d", &n); err == nil && n >= 1 && n <= len(t.Players) {
			t.SelectedIdx = n - 1
			return n - 1
		}
	}
}

// GetRandom returns a random number 0..max-1.
func (t *TerminalHost) GetRandom(max int) int {
	if max <= 0 {
		return 0
	}
	return t.rng.Intn(max)
}

// Combat runs combat. Returns true if party won.
func (t *TerminalHost) Combat() bool {
	fmt.Println("*** COMBAT ***")
	fmt.Println("  [Combat not yet implemented - auto-win]")
	return true
}

// LoadCharacter selects a player by index.
func (t *TerminalHost) LoadCharacter(idx int) {
	t.GameState.LoadCharacter(idx)
	name := "???"
	if p := t.selectedPlayer(); p != nil {
		name = strings.TrimRight(string(p.Raw[:15]), "\x00")
	}
	fmt.Printf("[Selected: %s]\n", name)
}

// Treasure generates treasure.
func (t *TerminalHost) Treasure(args [8]int) {
	fmt.Printf("[Treasure: type=%d count=%d]\n", args[0], args[1])
}

// EncounterMenu shows the encounter menu.
func (t *TerminalHost) EncounterMenu(args [14]int) {
	fmt.Println("[Encounter Menu]")
	t.ShowMenu([]string{"Fight", "Flee"}, true)
}

// Parlay runs the parley menu.
func (t *TerminalHost) Parlay(args [6]int) {
	fmt.Println("[Parlay]")
}

// Program handles meta operations (encamp, menu, etc).
func (t *TerminalHost) Program(op int) {
	switch op {
	case 0: // encamp / rest
		fmt.Println("[Encamp/Rest]")
	case 1: // main menu
		fmt.Println("[Main Menu]")
	default:
		fmt.Printf("[Program op=%d]\n", op)
	}
}

// CallSub calls a native engine subroutine.
func (t *TerminalHost) CallSub(id int) {
	fmt.Printf("[Call sub %d]\n", id)
}

// FindItem checks if party has an item.
func (t *TerminalHost) FindItem(itemType int) bool {
	found := t.GameState.FindItem(itemType)
	fmt.Printf("[FindItem type=%d -> %v]\n", itemType, found)
	return found
}

// DestroyItems removes items by type from the selected player.
func (t *TerminalHost) DestroyItems(itemType int) {
	fmt.Printf("[DestroyItems type=%d]\n", itemType)
	t.GameState.DestroyItems(itemType)
}

// FindSpecial checks if the selected player has an active affect.
func (t *TerminalHost) FindSpecial(affect int) bool {
	found := t.GameState.FindSpecial(affect)
	return found
}

// PartyStrength calculates party combat strength.
func (t *TerminalHost) PartyStrength() int {
	// Rough approximation based on party size
	return len(t.Players) * 10
}

// CheckParty checks party affects/skills.
func (t *TerminalHost) CheckParty(args [6]int) {}

// PartySurprise checks surprise.
func (t *TerminalHost) PartySurprise(args [2]int) (bool, bool) {
	return false, false
}

// Surprise rolls surprise for both sides.
func (t *TerminalHost) Surprise(args [4]int) {}

// Spell searches party for a memorized spell.
func (t *TerminalHost) Spell(args [3]int) bool {
	return t.GameState.Spell(args)
}

// Damage deals dice-based damage.
func (t *TerminalHost) Damage(dice, count, target, a, b int) {
	fmt.Printf("[Damage: %dd%d to target %d]\n", count, dice, target)
}

// GetTable reads from memory table.
func (t *TerminalHost) GetTable(table, index, count int) uint16 {
	return t.GameState.GetTable(table, index, count)
}

// SaveTable writes to memory table.
func (t *TerminalHost) SaveTable(table, index, count int, val uint16) {
	t.GameState.SaveTable(table, index, count, val)
}

// Clock advances game time.
func (t *TerminalHost) Clock(hours, minutes int) {
	fmt.Printf("[Time +%dh %dm]\n", hours, minutes)
}

// AddNPC adds an NPC.
func (t *TerminalHost) AddNPC(id, morale int) {
	fmt.Printf("[AddNPC id=%d morale=%d]\n", id, morale)
}

// LoadPieces loads wall definitions.
func (t *TerminalHost) LoadPieces(a, b, c int) {}

// LoadFiles loads area resources.
func (t *TerminalHost) LoadFiles(a, b, c int) {
	fmt.Printf("[LoadFiles %d %d %d]\n", a, b, c)
}

// Dump refreshes party summary.
func (t *TerminalHost) Dump() {
	fmt.Println("--- Party ---")
	for i, p := range t.Players {
		name := strings.TrimRight(string(p.Raw[:15]), "\x00")
		if name == "" {
			name = fmt.Sprintf("Player %d", i+1)
		}
		fmt.Printf("  %s HP %d/%d\n", name, p.Raw[0x1A4], p.Raw[0x78])
	}
}

// ClearBox clears the text area.
func (t *TerminalHost) ClearBox() {
	fmt.Println()
}

// Protection is copy protection check.
func (t *TerminalHost) Protection(check int) {}

// LoadMonster loads monster(s).
func (t *TerminalHost) LoadMonster(id, count, icon int) {
	fmt.Printf("[LoadMonster id=%d count=%d]\n", id, count)
}

// SetupMonster sets encounter sprite.
func (t *TerminalHost) SetupMonster(sprite, distance, pic int) {
	fmt.Printf("[SetupMonster sprite=%d dist=%d]\n", sprite, distance)
}

// ClearMonsters clears encounter data.
func (t *TerminalHost) ClearMonsters() {}

// Approach decrements encounter distance.
func (t *TerminalHost) Approach() {}

// SpriteOff turns off encounter sprite.
func (t *TerminalHost) SpriteOff() {}

// Rob takes money/items from party.
func (t *TerminalHost) Rob(who, moneyPct, itemPct int) {
	fmt.Printf("[Rob who=%d money=%d%% items=%d%%]\n", who, moneyPct, itemPct)
}

// NewECL signals a script block change.
func (t *TerminalHost) NewECL(blockID byte) {
	fmt.Printf("[NewECL block=0x%02X]\n", blockID)
}
