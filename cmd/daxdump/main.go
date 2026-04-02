// daxdump is a CLI tool for inspecting SSI Gold Box DAX container files.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dlukt/dax"
)

func main() {
	extractID := flag.Int("e", -1, "extract and write record with this ID to file")
	dumpAll := flag.Bool("a", false, "decompress and hex-dump all records")
	disasmECL := flag.Bool("d", false, "disassemble ECL bytecode")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: daxdump [-e id] [-a] [-d] <file.dax>\n")
		os.Exit(1)
	}

	path := flag.Arg(0)
	f, err := dax.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	f.Dump(os.Stdout)

	if *extractID >= 0 {
		data := f.Decode(byte(*extractID))
		if data == nil {
			fmt.Fprintf(os.Stderr, "record 0x%02X not found\n", *extractID)
			os.Exit(1)
		}
		outPath := fmt.Sprintf("%s_rec0x%02X.bin", filepath.Base(path), *extractID)
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote %d bytes to %s\n", len(data), outPath)
	}

	if *disasmECL {
		all := f.DisassembleAllECL()
		for _, e := range f.Entries() {
			rec, ok := all[e.ID]
			if !ok {
				continue
			}
			fmt.Printf("\n=== Record 0x%02X ===\n", e.ID)
			fmt.Printf("  Entry points: run=%04X search=%04X precamp=%04X campint=%04X init=%04X\n",
				rec.Header.RunAddr, rec.Header.SearchLocation,
				rec.Header.PreCampCheck, rec.Header.CampInterrupted,
				rec.Header.InitialEntry)

			// Print strings found in this record
			var strings_found []string
			for _, inst := range rec.Code {
				for _, op := range inst.Operands {
					if op.Text != "" {
						strings_found = append(strings_found, op.Text)
					}
				}
			}
			if len(strings_found) > 0 {
				fmt.Printf("  --- Strings (%d) ---\n", len(strings_found))
				for i, s := range strings_found {
					if len(s) > 60 {
						s = s[:60] + "..."
					}
					fmt.Printf("  [%d] \"%s\"\n", i, s)
				}
			}

			// Print disassembly (first 40 instructions)
			fmt.Printf("  --- Code (%d instructions) ---\n", len(rec.Code))
			limit := len(rec.Code)
			if limit > 40 {
				limit = 40
			}
			for _, inst := range rec.Code[:limit] {
				fmt.Printf("  %s\n", inst.String())
			}
			if len(rec.Code) > 40 {
				fmt.Printf("  ... %d more instructions\n", len(rec.Code)-40)
			}
		}
	}

	if *dumpAll && !*disasmECL {
		all := f.DecodeAll()
		for _, e := range f.Entries() {
			data := all[e.ID]
			fmt.Printf("\n--- Record 0x%02X (%d bytes) ---\n", e.ID, len(data))
			hexDump(data, 64)
		}
	}

	_ = strings.TrimSpace
}

func hexDump(data []byte, limit int) {
	n := len(data)
	if n > limit {
		n = limit
	}
	for i := 0; i < n; i += 16 {
		end := i + 16
		if end > n {
			end = n
		}
		fmt.Printf("%06x  ", i)
		for j := i; j < end; j++ {
			fmt.Printf("%02x ", data[j])
		}
		for j := end; j < i+16; j++ {
			fmt.Print("   ")
		}
		fmt.Print(" ")
		for j := i; j < end; j++ {
			b := data[j]
			if b >= 0x20 && b < 0x7f {
				fmt.Printf("%c", b)
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println()
	}
	if len(data) > limit {
		fmt.Printf("  ... (%d more bytes)\n", len(data)-limit)
	}
}
