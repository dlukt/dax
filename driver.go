package dax

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

// handleReload processes NEWECL transitions. When the VM sets its
// reload flag, we load the new block and run its initial entry.
// This repeats until no more reloads are requested, then runs
// the per-step and search addresses to settle.
func (d *Driver) handleReload() {
	if d.vm == nil || !d.vm.ReloadFlag() {
		return
	}

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

	// After reload settles, run the step and search addresses
	d.vm.Run(d.Header.RunAddr)
	if d.vm.ReloadFlag() {
		d.handleReload()
		return
	}
	d.vm.Run(d.Header.SearchLocation)
	if d.vm.ReloadFlag() {
		d.handleReload()
	}
}
