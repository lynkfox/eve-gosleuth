package retrieval

import (
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/fitting"
)

/*
	FittingInput reads the fit as the dogma engine takes it. Boosters are left out, as one in cargo is not in the pilot.

Returns:
  - fitting.Input: the hull, every fitted module with its charge, and the implants
*/
func (f *Fit) FittingInput() fitting.Input {
	in := fitting.Input{}
	if f.Hull != nil {
		in.Hull = f.Hull.ItemID
	}
	for _, rack := range []struct {
		modules []*FittedModule
		slot    comm.FitSlot
	}{
		{f.Highs, comm.SlotHigh}, {f.Mids, comm.SlotMid}, {f.Lows, comm.SlotLow}, {f.Rigs, comm.SlotRig}, {f.Subsystems, comm.SlotSubsystem}, {f.Services, comm.SlotService},
	} {
		for _, fitted := range rack.modules {
			if fitted == nil || fitted.Module == nil {
				continue
			}
			module := fitting.Module{Type: fitted.Module.ItemID, Slot: rack.slot}
			if fitted.Charge != nil {
				module.Charge = fitted.Charge.ItemID
			}
			in.Modules = append(in.Modules, module)
		}
	}
	for _, implant := range f.Implants {
		if implant != nil {
			in.Implants = append(in.Implants, implant.ItemID)
		}
	}
	return in
}
