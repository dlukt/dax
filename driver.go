package dax

import (
	"fmt"
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
}

// NewDriver creates a Driver for the given game state and DAX file.
func NewDriver(state *GameState, f *File) *Driver {
	return &Driver{State: state, File: f}
}

// LoadArea loads an ECL block, initializes the VM, and runs the initial
// entry point. Handles NEWECL reloads automatically.
func (d *Driver) LoadArea(blockID byte) {
	d.loadAndRun(blockID)
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
// NEWECL chaining until execution settles.
func (d *Driver) loadAndRun(blockID byte) {
	d.BlockID = blockID
	data := d.File.Decode(blockID)
	if data == nil {
		return
	}
	d.State.ECLData = data

	hdr, _ := DisassembleECL(data)
	d.Header = hdr

	d.vm = NewVM(d.State, blockID, data)
	d.vm.Run(hdr.InitialEntry)
	d.handleReload()
}

// handleReload processes NEWECL transitions. It runs iteratively
// to avoid stack overflow from deeply nested reload chains.
func (d *Driver) handleReload() {
	for {
		if d.vm == nil || !d.vm.ReloadFlag() {
			return
		}

		// Phase 1: process NEWECL chain
		for d.vm.ReloadFlag() {
			newID := d.vm.BlockID()
			data := d.File.Decode(newID)
			if data == nil {
				break
			}
			d.BlockID = newID
			d.State.ECLData = data

			hdr, _ := DisassembleECL(data)
			d.Header = hdr

			d.vm = NewVM(d.State, newID, data)
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
