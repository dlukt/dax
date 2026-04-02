package dax

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisassembleECL(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	f, err := Open(filepath.Join(dir, "ecl1.dax"))
	if err != nil {
		t.Fatal(err)
	}

	all := f.DisassembleAllECL()
	if len(all) == 0 {
		t.Fatal("no records disassembled")
	}

	for id, rec := range all {
		if len(rec.Code) == 0 {
			t.Errorf("record 0x%02X: no instructions", id)
		}

		// Header entry points should be non-zero (at least the initial entry)
		if rec.Header.InitialEntry == 0 {
			t.Errorf("record 0x%02X: initial entry point is 0", id)
		}

		// Every instruction should have a name
		for _, inst := range rec.Code {
			if inst.Name == "" {
				t.Errorf("record 0x%02X addr %04X: empty name", id, inst.Addr)
			}
		}
	}
}

func TestDisassembleECLString(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	f, err := Open(filepath.Join(dir, "ecl1.dax"))
	if err != nil {
		t.Fatal(err)
	}

	all := f.DisassembleAllECL()

	// ECL files should contain text strings (dialogue, prompts)
	foundString := false
	for _, rec := range all {
		for _, inst := range rec.Code {
			for _, op := range inst.Operands {
				if op.Text != "" {
					foundString = true
					break
				}
			}
			if foundString {
				break
			}
		}
		if foundString {
			break
		}
	}

	if !foundString {
		t.Error("no text strings found in ECL scripts (expected dialogue text)")
	}
}

func TestDisassembleECLAllFiles(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("ecl%d.dax", i)
		t.Run(name, func(t *testing.T) {
			f, err := Open(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}

			all := f.DisassembleAllECL()
			totalInsts := 0
			for _, rec := range all {
				totalInsts += len(rec.Code)
			}

			if totalInsts == 0 {
				t.Error("no instructions disassembled")
			}
			t.Logf("%d records, %d total instructions", len(all), totalInsts)
		})
	}
}

func TestDisassembleECLKnownOpcodes(t *testing.T) {
	dir := filepath.Join("..", "pool-remake", "dos", "pool-of-radiance")

	f, err := Open(filepath.Join(dir, "ecl1.dax"))
	if err != nil {
		t.Fatal(err)
	}

	all := f.DisassembleAllECL()

	// Verify we find expected opcode types
	opcodeNames := map[string]bool{
		"GOTO": false, "GOSUB": false, "PRINT": false,
		"COMPARE": false, "RETURN": false,
	}

	for _, rec := range all {
		for _, inst := range rec.Code {
			if _, ok := opcodeNames[inst.Name]; ok {
				opcodeNames[inst.Name] = true
			}
		}
	}

	for name, found := range opcodeNames {
		if !found {
			t.Errorf("expected to find %s opcode in ecl1", name)
		}
	}
}

func TestDisassembleInstructionFormat(t *testing.T) {
	inst := Instruction{
		Addr:   0x8005,
		Name:   "GOTO",
		Opcode: 0x01,
		Operands: []Operand{
			{Code: 0x02, Low: 0x34, High: 0x80},
		},
	}

	s := inst.String()
	if !strings.Contains(s, "GOTO") {
		t.Errorf("expected GOTO in output, got: %s", s)
	}
	if !strings.Contains(s, "$8034") {
		t.Errorf("expected $8034 in output, got: %s", s)
	}
}
