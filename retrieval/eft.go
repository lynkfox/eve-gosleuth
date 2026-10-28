package retrieval

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

/*
	EFT writes the fit in EFT format, which the in-game "Import from clipboard" and pyfa read. It follows pyfa's exporter (service/port/eft.py):
	sections are two blank lines apart, the slot groups within the modules one blank line apart.

	- The header, then lows, mids, highs, rigs, subsystems and services, each slot on its own line. A group the hull has no slots for is left out.
	- Drones then fighters, as Name xN.
	- Cargo as Name xN, with the boosters in it, since the in-game importer is fussier about a separate booster section. Implants are not exported.

Parameters:
  - title: the fit's name in the header, see EFTTitle

Returns:
  - string: the fit as EFT text
*/
func (f *Fit) EFT(title string) string {
	hull := unknownTypeName
	if f.Hull != nil {
		hull = f.Hull.Name
	}
	header := fmt.Sprintf("[%s, %s]", hull, title)

	sections := []string{}

	racks := []string{}
	for _, rack := range []struct {
		name  string
		slots []*FittedModule
	}{{"Low", f.Lows}, {"Med", f.Mids}, {"High", f.Highs}, {"Rig", f.Rigs}, {"Subsystem", f.Subsystems}, {"Service", f.Services}} {
		lines := []string{}
		for _, fitted := range rack.slots {
			switch {
			case fitted == nil:
				lines = append(lines, fmt.Sprintf("[Empty %s slot]", rack.name))
			case fitted.Charge != nil:
				lines = append(lines, fitted.Module.Name+", "+fitted.Charge.Name)
			default:
				lines = append(lines, fitted.Module.Name)
			}
		}
		if len(lines) > 0 {
			racks = append(racks, strings.Join(lines, "\n"))
		}
	}
	if len(racks) > 0 {
		sections = append(sections, strings.Join(racks, "\n\n"))
	}

	minions := []string{}
	if drones := eftStacks(f.Drones); drones != "" {
		minions = append(minions, drones)
	}
	if fighters := eftStacks(f.Fighters); fighters != "" {
		minions = append(minions, fighters)
	}
	if len(minions) > 0 {
		sections = append(sections, strings.Join(minions, "\n\n"))
	}

	// copied, so a booster merged into a stack leaves the fit's own cargo alone
	cargo := make([]*FitCargo, len(f.Cargo))
	for index, stack := range f.Cargo {
		cargo[index] = &FitCargo{Item: stack.Item, Quantity: stack.Quantity}
	}
	for _, booster := range f.Boosters {
		cargo = addFitCargo(cargo, booster, 1)
	}
	if text := eftStacks(cargo); text != "" {
		sections = append(sections, text)
	}

	return header + "\n\n" + strings.Join(sections, "\n\n\n")
}

// eftStacks writes stacks as Name xN, one per line, sorted the way pyfa sorts cargo: by category, group, then name
func eftStacks(stacks []*FitCargo) string {
	sorted := slices.Clone(stacks)
	slices.SortStableFunc(sorted, func(a, b *FitCargo) int {
		// known types by category and group, then those fit_types.json does not have, each by name
		aType, bType := comm.FitTypeOf(a.Item.ItemID), comm.FitTypeOf(b.Item.ItemID)
		if aType == nil || bType == nil {
			if byKnown := cmp.Compare(boolOrder(aType == nil), boolOrder(bType == nil)); byKnown != 0 {
				return byKnown
			}
			return cmp.Compare(a.Item.Name, b.Item.Name)
		}
		if byCategory := cmp.Compare(aType.CategoryID, bType.CategoryID); byCategory != 0 {
			return byCategory
		}
		if byGroup := cmp.Compare(aType.GroupID, bType.GroupID); byGroup != 0 {
			return byGroup
		}
		return cmp.Compare(a.Item.Name, b.Item.Name)
	})
	lines := make([]string, len(sorted))
	for index, stack := range sorted {
		lines[index] = fmt.Sprintf("%s x%d", stack.Item.Name, stack.Quantity)
	}
	return strings.Join(lines, "\n")
}

// boolOrder sorts false before true
func boolOrder(value bool) int {
	if value {
		return 1
	}
	return 0
}

/*
	EFTTitle names a killmail's fit for its EFT header: GoSleuth - <pilot>. A structure or deployable, which has no pilot, is named by its owning corp.

Parameters:
  - killmail: the victim's killmail

Returns:
  - string: the title
*/
func EFTTitle(killmail *Killmail) string {
	owner := unknownTypeName
	switch victim := killmail.Victim; {
	case victim.Pilot != nil && victim.Pilot.Name != "":
		owner = victim.Pilot.Name
	case victim.CorpAtKill != nil && victim.CorpAtKill.Name != "":
		owner = victim.CorpAtKill.Name
	}
	return fmt.Sprintf("GoSleuth - %s", owner)
}
