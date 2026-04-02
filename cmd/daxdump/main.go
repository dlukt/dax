// daxdump is a CLI tool for inspecting SSI Gold Box DAX container files.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dlukt/dax"
)

func main() {
	extractID := flag.Int("e", -1, "extract and write record with this ID to stdout")
	dumpAll := flag.Bool("a", false, "decompress and hex-dump all records")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: daxdump [-e id] [-a] <file.dax>\n")
		os.Exit(1)
	}

	path := flag.Arg(0)
	d, err := dax.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	d.Dump(os.Stdout)

	if *extractID >= 0 {
		data := d.Decode(byte(*extractID))
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

	if *dumpAll {
		all := d.DecodeAll()
		for _, e := range d.Entries() {
			data := all[e.ID]
			fmt.Printf("\n--- Record 0x%02X (%d bytes) ---\n", e.ID, len(data))
			hexDump(data, 64)
		}
	}
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
