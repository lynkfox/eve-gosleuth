package comm

// FitSlot is where a killmail item sat on the victim's ship, read from its inventory flag with FitSlotOf
type FitSlot string

const (
	SlotHigh        FitSlot = "high"
	SlotMid         FitSlot = "mid"
	SlotLow         FitSlot = "low"
	SlotRig         FitSlot = "rig"
	SlotSubsystem   FitSlot = "subsystem"
	SlotService     FitSlot = "service" // upwell structure service modules
	SlotDroneBay    FitSlot = "drone_bay"
	SlotFighterBay  FitSlot = "fighter_bay"
	SlotFighterTube FitSlot = "fighter_tube" // launched or loaded fighters
	SlotImplant     FitSlot = "implant"
	SlotBooster     FitSlot = "booster" // an active booster (drug) in the pilot
	SlotCargo       FitSlot = "cargo"   // the cargo hold, and holds read as cargo ie the ammo hold
	SlotIgnored     FitSlot = "ignored" // holds a fit does not show, ie the fleet hangar or ship maintenance bay
	SlotUnknown     FitSlot = "unknown" // a flag inventory_flags.json does not list
)

// FitType is what a fit needs to know about an item type, from fit_types.json (generated from the SDE by cmd/sdegen)
type FitType struct {
	TypeID     int     `json:"-"` // the key in fit_types.json
	Name       string  `json:"name"`
	GroupID    int     `json:"group"`
	CategoryID int     `json:"category"`
	Slot       FitSlot `json:"slot,omitempty"`      // the slot a module fits in: high, mid, low, rig, subsystem, or service. Empty for anything that is not a module, ie a charge.
	Abyssal    bool    `json:"abyssal,omitempty"`   // a mutated (abyssal) module, whose rolled stats a killmail does not record
	HeadSlot   int     `json:"head_slot,omitempty"` // an implant's slot (1-10), or a booster's (1-4)
	Market     int     `json:"market,omitempty"`    // the market group it is listed under, 0 for a type not on the market (ie mutated modules). InMarketGroup reads its parents.

	// a hull's slots and hardpoints
	HighSlots      int      `json:"high_slots,omitempty"`
	MidSlots       int      `json:"mid_slots,omitempty"`
	LowSlots       int      `json:"low_slots,omitempty"`
	RigSlots       int      `json:"rig_slots,omitempty"`
	SubsystemSlots int      `json:"subsystem_slots,omitempty"`
	ServiceSlots   int      `json:"service_slots,omitempty"`
	TurretSlots    int      `json:"turret_slots,omitempty"`
	LauncherSlots  int      `json:"launcher_slots,omitempty"`
	RigSize        int      `json:"rig_size,omitempty"` // the hull's size by the rigs it takes: 1 small, 2 medium, 3 large, 4 capital
	Bonuses        []string `json:"bonuses,omitempty"`  // the hull's role and skill bonuses as text without the numbers, ie "bonus to Energy Nosferatu and Energy Neutralizer drain amount"

	// slots a T3 cruiser subsystem adds to its hull
	HighSlotModifier int `json:"high_slot_modifier,omitempty"`
	MidSlotModifier  int `json:"mid_slot_modifier,omitempty"`
	LowSlotModifier  int `json:"low_slot_modifier,omitempty"`
}
