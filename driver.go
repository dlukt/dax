package dax

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Driver manages the ECL execution lifecycle, orchestrating when
// the VM runs and handling script transitions (NEWECL).
type Driver struct {
	State   *GameState
	File    *File // DAX file containing ECL blocks
	BlockID byte
	Header  ECLHeader
	vm      *VM

	// DataDir is the game data directory (containing geo*.dax, walldef*.dax, etc.)
	DataDir string

	// blockData stores each decoded ECL block with the 2-byte DAX header
	// stripped (matching coab's load_ecl_dax SetData(block_mem, 2, ...)).
	// Each block is loaded independently into the VM — in coab, only one
	// block is active at a time in the EclBlock buffer.
	blockData   map[byte][]byte
	blockLoaded bool

	// geoMap caches the currently loaded GEO map (nil if not loaded).
	geoMap *GeoMap

	// Trace enables VM execution tracing.
	Trace bool

	// saveECLData holds pre-loaded ECL data from a save game.
	// When set, loadAndRun uses this instead of loading from the DAX file.
	// This is needed because the save stores the EclBlock buffer content,
	// which may have been modified by SAVE instructions during gameplay.
	saveECLData []byte
}

// NewDriver creates a Driver for the given game state and DAX file.
func NewDriver(state *GameState, f *File) *Driver {
	return &Driver{State: state, File: f}
}

// loadBlocks decodes all ECL blocks from the DAX file, stripping the
// 2-byte DAX block header from each (matching coab's load_ecl_dax).
func (d *Driver) loadBlocks() {
	if d.blockLoaded {
		return
	}
	d.blockLoaded = true
	d.blockData = make(map[byte][]byte)

	for _, e := range d.File.Entries() {
		raw := d.File.Decode(e.ID)
		if raw == nil || len(raw) < 3 {
			continue
		}
		// Strip 2-byte DAX block header (coab: SetData(block_mem, 2, block_size-2))
		d.blockData[e.ID] = raw[2:]
	}
}

// LoadGeo loads the GEO map for the current game area and updates
// the game state's WallRoof and WallType values.
func (d *Driver) LoadGeo(blockID byte) {
	if d.DataDir == "" {
		return
	}
	area := d.State.GameArea
	if area == 0 {
		area = 1
	}
	geoPath := filepath.Join(d.DataDir, fmt.Sprintf("geo%d.dax", area))
	geoFile, err := Open(geoPath)
	if err != nil {
		return
	}
	geo := geoFile.DecodeGeo(blockID)
	if geo == nil {
		return
	}
	d.geoMap = geo
	d.State.UpdateWallInfo(geo)
}

// LoadArea loads an ECL block, initializes the VM, and runs the initial
// entry point. Handles NEWECL reloads automatically.
func (d *Driver) LoadArea(blockID byte) {
	d.loadAndRun(blockID)
}

// SetSaveECLData provides pre-loaded ECL block data from a save game.
// When set, the next LoadArea call uses this data instead of loading
// from the DAX file. This matches coab's behavior of restoring the
// EclBlock buffer directly from the save file.
func (d *Driver) SetSaveECLData(data []byte) {
	d.saveECLData = data
}

// Step runs the per-step VM entry point (called each time the player
// moves or takes an action).
func (d *Driver) Step() {
	if d.vm == nil {
		return
	}
	d.vm.Run(d.Header.RunAddr)
	d.handleReload()
}

// Search runs the search/look VM entry point.
func (d *Driver) Search() {
	if d.vm == nil {
		return
	}
	d.vm.Run(d.Header.SearchLocation)
	d.handleReload()
}

// PreCamp runs the pre-camp check entry point.
func (d *Driver) PreCamp() {
	if d.vm == nil {
		return
	}
	d.vm.Run(d.Header.PreCampCheck)
	d.handleReload()
}

// CampInterrupted runs the camp-interrupted entry point.
func (d *Driver) CampInterrupted() {
	if d.vm == nil {
		return
	}
	d.vm.Run(d.Header.CampInterrupted)
	d.handleReload()
}

// MoveForward advances the party one step in the current facing direction.
func (d *Driver) MoveForward() {
	switch d.State.MapDir {
	case DirNorth:
		d.State.MapY--
	case DirEast:
		d.State.MapX++
	case DirSouth:
		d.State.MapY++
	case DirWest:
		d.State.MapX--
	}
	// Update wall info from GEO map after movement
	d.State.UpdateWallInfo(d.geoMap)
}

// TurnLeft rotates the party 90 degrees counter-clockwise.
func (d *Driver) TurnLeft() {
	d.State.MapDir = (d.State.MapDir + 6) % 8 // 0→6→4→2→0
}

// TurnRight rotates the party 90 degrees clockwise.
func (d *Driver) TurnRight() {
	d.State.MapDir = (d.State.MapDir + 2) % 8
}

// AboutFace reverses the party's facing direction.
func (d *Driver) AboutFace() {
	d.State.MapDir = (d.State.MapDir + 4) % 8
}

// DirName returns a human-readable direction name.
func DirName(dir int) string {
	switch dir {
	case DirNorth:
		return "North"
	case DirEast:
		return "East"
	case DirSouth:
		return "South"
	case DirWest:
		return "West"
	default:
		return "???"
	}
}

// GameLoop runs the interactive game loop: prompt for commands,
// update game state, and run the appropriate ECL entry points.
// The host is used for any VM I/O (print, menus, etc).
func (d *Driver) GameLoop(host Host) {
	for {
		fmt.Printf("\n--- Position (%d,%d) facing %s ---\n",
			d.State.MapX, d.State.MapY, DirName(d.State.MapDir))
		fmt.Print("Command (F=forward L=left R=right B=back S=search E=encamp Q=quit): ")

		var cmd string
		fmt.Scanln(&cmd)
		cmd = strings.ToUpper(strings.TrimSpace(cmd))

		if len(cmd) == 0 {
			continue
		}

		switch cmd[0] {
		case 'Q':
			fmt.Println("Goodbye!")
			return

		case 'F':
			d.MoveForward()
			d.Step()
			d.Search()

		case 'L':
			d.TurnLeft()
			// Turning doesn't trigger ECL in the original engine

		case 'R':
			d.TurnRight()

		case 'B':
			d.AboutFace()

		case 'S':
			d.Search()

		case 'E':
			d.PreCamp()
			host.Program(0) // encamp menu stub

		default:
			fmt.Println("Unknown command. F/L/R/B/S/E/Q")
		}

		// Clear position changed flag after processing
		d.State.PosChanged = false
	}
}

// loadAndRun loads an ECL block, runs its initial entry, and handles
// NEWECL chaining. After init settles, runs RunAddr and SearchLocation
// to match coab's sub_29677 flow.
func (d *Driver) loadAndRun(blockID byte) {
	var data []byte

	// Always load DAX blocks so NEWECL can find them during execution.
	d.loadBlocks()

	// Use save game's ECL data if available (it's the full EclBlock buffer
	// with runtime modifications from SAVE instructions).
	if d.saveECLData != nil {
		data = d.saveECLData
		d.saveECLData = nil // consume it
	} else {
		var ok bool
		data, ok = d.blockData[blockID]
		if !ok {
			return
		}
	}
	d.BlockID = blockID

	// Data is already header-stripped (loadBlocks or save game both pre-strip).
	hdr, _ := DisassembleECL(data)
	d.Header = hdr

	// Load GEO map for wall type computation
	d.LoadGeo(blockID)

	// data is already stripped of the 2-byte DAX header in loadBlocks.
	// Pass it directly — the VM places it at 0x8000 in flat memory,
	// matching coab where EclBlock.data[0] = first ECL byte.
	d.State.ECLData = data
	d.vm = NewVM(d.State, blockID, data)
	d.vm.Trace = d.Trace

	// Phase 1: Run init entry, handle NEWECL chain
	d.vm.Run(hdr.InitialEntry)
	if d.handleReload() {
		return // NEWECL processed — handleReload already ran Phase 2/3
	}

	// Phase 2: Run step address (coab: RunEclVm(vm_run_addr_1))
	if d.vm != nil {
		d.vm.Run(d.Header.RunAddr)
		if d.handleReload() {
			return // NEWECL processed
		}
	}

	// Phase 3: Run search address (coab: RunEclVm(SearchLocationAddr))
	if d.vm != nil && !d.vm.ReloadFlag() {
		d.vm.Run(d.Header.SearchLocation)
		d.handleReload()
	}
}

// handleReload processes NEWECL transitions. It runs iteratively
// to avoid stack overflow from deeply nested reload chains.
// Returns true if any NEWECL was processed (caller should skip its
// own Phase 2/3 since handleReload already ran them).
func (d *Driver) handleReload() bool {
	hadReload := false
	for {
		if d.vm == nil || !d.vm.ReloadFlag() {
			return hadReload
		}
		hadReload = true

		// Phase 1: process NEWECL chain
		for d.vm.ReloadFlag() {
			newID := d.vm.BlockID()
			data, ok := d.blockData[newID]
			if !ok {
				break
			}
			d.BlockID = newID

			hdr, _ := DisassembleECL(data)
			d.Header = hdr

			// Load the new block independently (coab replaces EclBlock)
			d.State.ECLData = data
			d.LoadGeo(newID)
			d.vm = NewVM(d.State, newID, data)
			d.vm.Trace = d.Trace
			d.vm.Run(hdr.InitialEntry)
		}

		// Phase 2: Run step address
		d.vm.Run(d.Header.RunAddr)
		if !d.vm.ReloadFlag() {
			// Phase 3: Run Search address
			d.vm.Run(d.Header.SearchLocation)
		}
		// Loop back if another reload was triggered
	}
}
