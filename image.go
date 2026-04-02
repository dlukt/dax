package dax

import "encoding/binary"

// Picture represents a decoded Gold Box image block (used for BODY, HEAD,
// combat sprites, and similar single-item images).
//
// The decompressed record layout is:
//
//	[0x00] u16le  height (pixels)
//	[0x02] u16le  width (in 8-pixel units)
//	[0x04] u16le  x_pos
//	[0x06] u16le  y_pos
//	[0x08] u8     item_count
//	[0x09] u8[8]  field_9 (palette/metadata)
//	[0x11] ...    packed 4-bit pixel data (height * width * 4 bytes per item)
//
// Each byte encodes two pixels: high nibble first, low nibble second.
// Pixel values are 4-bit EGA color indices (0-15).
type Picture struct {
	Height    int      // pixels
	Width     int      // in 8-pixel units (actual width = Width*8)
	XPos      int      // column position (in 8-pixel units)
	YPos      int      // row position (in 8-pixel units)
	ItemCount int      // number of sub-images
	Field9    [8]byte  // metadata/palette flags
	Pixels    []byte   // decoded pixel data: ItemCount * Height * Width*8 bytes, values 0-15
}

// PixelWidth returns the actual pixel width of each sub-image.
func (p *Picture) PixelWidth() int { return p.Width * 8 }

// ItemPixels returns the decoded pixels for the given item index (0-based).
// Returns a flat slice of PixelWidth()*Height bytes with values 0-15.
func (p *Picture) ItemPixels(index int) []byte {
	bpp := p.Height * p.PixelWidth()
	start := index * bpp
	if start+bpp > len(p.Pixels) {
		return nil
	}
	return p.Pixels[start : start+bpp]
}

// DecodePicture decodes a single-item image record (BODY, HEAD, combat icons).
// maskColor specifies which color index to treat as transparent (0 = none).
func (f *File) DecodePicture(id byte, maskColor int) *Picture {
	entry, compressed, ok := f.findRecord(id)
	if !ok {
		return nil
	}
	raw := decompress(int(entry.RawSize), compressed)
	if len(raw) < 17 {
		return nil
	}

	p := &Picture{
		Height:    int(binary.LittleEndian.Uint16(raw[0:2])),
		Width:     int(binary.LittleEndian.Uint16(raw[2:4])),
		XPos:      int(binary.LittleEndian.Uint16(raw[4:6])),
		YPos:      int(binary.LittleEndian.Uint16(raw[6:8])),
		ItemCount: int(raw[8]),
	}
	copy(p.Field9[:], raw[9:17])

	pixelWidth := p.Width * 8
	bpp := p.Height * pixelWidth
	totalPixels := p.ItemCount * bpp
	p.Pixels = make([]byte, totalPixels)

	srcOff := 17
	dstOff := 0

	for item := 0; item < p.ItemCount; item++ {
		for y := 0; y < p.Height; y++ {
			for x := 0; x < p.Width*4; x++ { // 4 bytes per 8 pixels
				if srcOff >= len(raw) {
					return p
				}
				b := raw[srcOff]

				hi := b >> 4
				lo := b & 0x0F

				if maskColor >= 0 && hi == byte(maskColor) {
					p.Pixels[dstOff] = 16 // transparent sentinel
				} else {
					p.Pixels[dstOff] = hi
				}
				dstOff++

				if maskColor >= 0 && lo == byte(maskColor) {
					p.Pixels[dstOff] = 16
				} else {
					p.Pixels[dstOff] = lo
				}
				dstOff++
				srcOff++
			}
		}
	}

	return p
}

// AnimationFrame holds one frame of a PIC/FINAL animated background.
type AnimationFrame struct {
	Delay   int      // frame delay (in game ticks)
	Height  int      // pixels
	Width   int      // in 8-pixel units
	XPos    int      // column (8-pixel units)
	YPos    int      // row
	Field9  [8]byte  // metadata
	Pixels  []byte   // decoded pixel data
}

// Animation represents a decoded PIC/FINAL multi-frame animation.
type Animation struct {
	Frames []AnimationFrame
}

// DecodeAnimation decodes a PIC or FINAL multi-frame animation record.
// These files have a different layout than simple Picture records:
//
//	[0x00] u8      frame_count
//	[0x01] per frame:
//	  u32le  delay
//	  u16le  height
//	  u16le  width (8-pixel units)
//	  u16le  x_pos
//	  u16le  y_pos + 1 byte padding
//	  u8[8]  field_9
//	  ...    packed 4-bit pixel data
//
// Frames after the first are XOR-delta encoded against the first frame.
func (f *File) DecodeAnimation(id byte) *Animation {
	entry, compressed, ok := f.findRecord(id)
	if !ok {
		return nil
	}
	raw := decompress(int(entry.RawSize), compressed)
	if len(raw) == 0 {
		return nil
	}

	off := 0
	frameCount := int(raw[off])
	off++

	if frameCount == 0 {
		return &Animation{}
	}

	anim := &Animation{Frames: make([]AnimationFrame, frameCount)}

	var firstFrameEGA []byte

	for i := 0; i < frameCount; i++ {
		if off+8 > len(raw) {
			break
		}

		frame := AnimationFrame{}
		frame.Delay = int(binary.LittleEndian.Uint32(raw[off : off+4]))
		off += 4

		frame.Height = int(binary.LittleEndian.Uint16(raw[off : off+2]))
		off += 2

		frame.Width = int(binary.LittleEndian.Uint16(raw[off : off+2]))
		off += 2

		if off+4 > len(raw) {
			break
		}
		frame.XPos = int(binary.LittleEndian.Uint16(raw[off : off+2]))
		off += 2

		frame.YPos = int(binary.LittleEndian.Uint16(raw[off : off+2]))
		off += 3 // 2 bytes + 1 padding byte

		if off+8 > len(raw) {
			break
		}
		copy(frame.Field9[:], raw[off:off+8])
		off += 8

		pixelWidth := frame.Width * 8
		bpp := frame.Height * pixelWidth
		egaSize := (bpp / 2) - 1

		if off+egaSize+1 > len(raw) {
			break
		}

		// Decode packed 4-bit pixels
		frame.Pixels = make([]byte, bpp)
		dstOff := 0
		for y := 0; y < frame.Height; y++ {
			for x := 0; x < frame.Width*4; x++ {
				if off >= len(raw) {
					break
				}
				b := raw[off]
				frame.Pixels[dstOff] = b >> 4
				dstOff++
				frame.Pixels[dstOff] = b & 0x0F
				dstOff++
				off++
			}
		}

		// For PIC/FINAL: frames after first are XOR deltas against first frame
		if i == 0 {
			firstFrameEGA = make([]byte, egaSize+1)
			copy(firstFrameEGA, raw[off-(egaSize+1):off])
		} else if firstFrameEGA != nil {
			pixelSrcStart := off - (egaSize + 1)
			for j := 0; j < egaSize && pixelSrcStart+j < len(raw); j++ {
				raw[pixelSrcStart+j] ^= firstFrameEGA[j]
			}
			// Re-decode this frame's pixels with the un-XOR'd data
			dstOff = 0
			srcOff := pixelSrcStart
			for y := 0; y < frame.Height; y++ {
				for x := 0; x < frame.Width*4; x++ {
					if srcOff >= len(raw) {
						break
					}
					b := raw[srcOff]
					frame.Pixels[dstOff] = b >> 4
					dstOff++
					frame.Pixels[dstOff] = b & 0x0F
					dstOff++
					srcOff++
				}
			}
		}

		anim.Frames[i] = frame
	}

	return anim
}
