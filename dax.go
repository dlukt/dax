// Package dax provides a reader for SSI Gold Box .DAX container files.
//
// DAX is an indexed container format with RLE-compressed records. It was used
// across all SSI Gold Box games (Pool of Radiance, Curse of the Azure Bonds, etc.)
// for sprites, maps, encounters, wall definitions, and other game data.
//
// Container layout:
//
//	[0x00] u16le  toc_size (in bytes)
//	[0x02] toc    toc_size / 9 entries, each 9 bytes:
//	  u8    record_id
//	  u32le relative_offset (from payload_base)
//	  i16le raw_size (decompressed)
//	  u16le comp_size (stored size)
//	[0x02 + toc_size] payloads
package dax

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const tocEntrySize = 9

// HeaderEntry describes one record in the DAX table of contents.
type HeaderEntry struct {
	ID             byte
	Offset         uint32 // relative to payload base
	RawSize        int16  // decompressed size
	CompressedSize uint16 // stored size in file
}

// File represents a parsed DAX container.
type File struct {
	entries []HeaderEntry
	data    []byte // raw file bytes
}

// Open reads and parses a DAX file from disk.
func Open(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("dax: read file: %w", err)
	}
	return Parse(raw)
}

// Parse parses a DAX container from raw bytes.
func Parse(data []byte) (*File, error) {
	if len(data) < 2 {
		return nil, errors.New("dax: file too small")
	}

	tocSize := int(binary.LittleEndian.Uint16(data[0:2]))
	if tocSize%tocEntrySize != 0 {
		return nil, fmt.Errorf("dax: toc size %d is not a multiple of %d", tocSize, tocEntrySize)
	}

	payloadBase := 2 + tocSize
	if payloadBase > len(data) {
		return nil, errors.New("dax: toc extends beyond file")
	}

	recordCount := tocSize / tocEntrySize
	entries := make([]HeaderEntry, recordCount)

	off := 2
	for i := range entries {
		if off+tocEntrySize > len(data) {
			return nil, fmt.Errorf("dax: truncated toc at entry %d", i)
		}
		entries[i].ID = data[off]
		entries[i].Offset = binary.LittleEndian.Uint32(data[off+1 : off+5])
		entries[i].RawSize = int16(binary.LittleEndian.Uint16(data[off+5 : off+7]))
		entries[i].CompressedSize = binary.LittleEndian.Uint16(data[off+7 : off+9])
		off += tocEntrySize
	}

	// Validate offsets
	for i, e := range entries {
		start := payloadBase + int(e.Offset)
		end := start + int(e.CompressedSize)
		if start < payloadBase || end > len(data) {
			return nil, fmt.Errorf("dax: entry %d (id=%d) offsets out of bounds", i, e.ID)
		}
	}

	return &File{entries: entries, data: data}, nil
}

// Entries returns all table-of-contents entries.
func (f *File) Entries() []HeaderEntry {
	return f.entries
}

// RecordCount returns the number of records in the container.
func (f *File) RecordCount() int {
	return len(f.entries)
}

// payloadBase is the byte offset where payloads begin.
func (f *File) payloadBase() int {
	return 2 + len(f.entries)*tocEntrySize
}

// Compressed returns the raw compressed bytes for a given record ID.
// Returns nil if the record is not found.
func (f *File) Compressed(id byte) []byte {
	_, raw, ok := f.findRecord(id)
	if !ok {
		return nil
	}
	return raw
}

// Decode decompresses the record with the given ID.
// Returns nil if the record is not found.
func (f *File) Decode(id byte) []byte {
	entry, compressed, ok := f.findRecord(id)
	if !ok {
		return nil
	}
	return decompress(int(entry.RawSize), compressed)
}

// DecodeAll decompresses all records and returns them keyed by record ID.
func (f *File) DecodeAll() map[byte][]byte {
	result := make(map[byte][]byte, len(f.entries))
	for _, e := range f.entries {
		result[e.ID] = f.Decode(e.ID)
	}
	return result
}

func (f *File) findRecord(id byte) (HeaderEntry, []byte, bool) {
	pb := f.payloadBase()
	for _, e := range f.entries {
		if e.ID == id {
			start := pb + int(e.Offset)
			end := start + int(e.CompressedSize)
			return e, f.data[start:end], true
		}
	}
	return HeaderEntry{}, nil, false
}

// decompress implements the Gold Box DAX RLE decompression algorithm.
//
// The compressed stream is a sequence of chunks:
//
//	[byte >= 0] [N+1 literal bytes]   — copy N+1 bytes as-is
//	[byte <  0] [1 byte]              — repeat that byte (-N) times
//
// This matches the confirmed implementation in Simeon Pilgrim's coab.
func decompress(rawSize int, compressed []byte) []byte {
	if rawSize <= 0 || len(compressed) == 0 {
		return nil
	}

	output := make([]byte, 0, rawSize)
	inIdx := 0

	for inIdx < len(compressed) {
		b := int8(compressed[inIdx])

		if b >= 0 {
			// Literal run: copy b+1 bytes
			count := int(b) + 1
			inIdx++
			end := inIdx + count
			if end > len(compressed) {
				break
			}
			output = append(output, compressed[inIdx:end]...)
			inIdx = end
		} else {
			// Fill run: repeat next byte (-b) times
			count := int(-b)
			inIdx++
			if inIdx >= len(compressed) {
				break
			}
			val := compressed[inIdx]
			inIdx++
			for j := 0; j < count; j++ {
				output = append(output, val)
			}
		}
	}

	return output
}

// Dump writes a human-readable summary of the DAX container to w.
func (f *File) Dump(w io.Writer) {
	pb := f.payloadBase()
	fmt.Fprintf(w, "DAX container: %d entries, payload base at 0x%04X\n", len(f.entries), pb)
	fmt.Fprintf(w, "%-6s %-8s %-8s %-8s %-10s\n", "ID", "RelOff", "RawSz", "CompSz", "AbsOff")
	for _, e := range f.entries {
		fmt.Fprintf(w, "0x%02X   %-8d %-8d %-8d 0x%04X\n",
			e.ID, e.Offset, e.RawSize, e.CompressedSize, pb+int(e.Offset))
	}
}
