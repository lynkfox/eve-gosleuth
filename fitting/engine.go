package fitting

import (
	"math"
	"sort"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// Role is what an item is to the fit, which decides who its modifiers reach and whether they are stacking penalized (only modules' and charges' are)
type Role int

const (
	RoleCharacter Role = iota
	RoleSkill
	RoleImplant
	RoleHull
	RoleMode      // a tactical destroyer's mode
	RoleSubsystem // a T3 cruiser's subsystem
	RoleModule    // highs, mids, lows, rigs, and services
	RoleCharge    // loaded in a module
)

// State is how far a module is switched on, which decides which of its effects apply
type State int

const (
	StateOffline State = iota
	StateOnline
	StateActive
	StateOverheated
)

// Item is one item in the fit: the character, a skill, an implant, the hull, a mode, a subsystem, a module, or a charge
type Item struct {
	Type   *comm.DogmaType
	Role   Role
	Slot   comm.FitSlot // a module's or subsystem's slot, empty otherwise
	State  State        // a module's; everything else applies its effects whatever this is
	Charge *Item        // a module's loaded charge, nil if none
	Module *Item        // a charge's module, nil for anything else

	base map[int]float64 // attribute values set over the type's own, ie a skill's level
}

// Fit is a fit loaded into the engine. Attributes are worked out when first read and kept.
type Fit struct {
	Character *Item
	Skills    []*Item // every skill in dogma.json, at V
	Implants  []*Item
	Ship      *Item
	Mode      *Item   // nil if none
	Modules   []*Item // subsystems and modules; their charges hang off them
	Unknown   []int   // types in the Input dogma.json doesn't have, left out

	all    []*Item                // every item above and the charges
	skills map[int]*Item          // the skills by type id
	index  map[attrKey][]modEntry // every modifier in force, by the item and attribute it changes; built on first use
	cache  *attrCache             // the worked out attributes
}

// HandModifier is a modifier that dogma.json can't express, added by code (ie the Reactive Armor Hardener). It is applied like a modifier from Source: stacking penalized if Source is a module or charge.
type HandModifier struct {
	Source    *Item
	Target    *Item // the item changed, ie fit.Ship
	Attribute int   // the attribute changed
	Operation int   // as DogmaModifier.Operation
	Value     func(fit *Fit) float64
}

/*
	NewFit loads a fit into the engine: the character with every skill at V, the hull, its mode, subsystems, modules (all active) with their charges, and implants

Parameters:
  - in: the fit

Returns:
  - *Fit: the fit, with nothing worked out yet
  - error: ErrUnknownHull if dogma.json has no type for the hull
*/
func NewFit(in Input) (*Fit, error) {
	hull := comm.DogmaTypeOf(in.Hull)
	if hull == nil {
		return nil, ErrUnknownHull
	}
	f := &Fit{skills: map[int]*Item{}, cache: newAttrCache(nil)}

	if character := comm.DogmaCharacterType(); character != nil {
		f.Character = &Item{Type: character, Role: RoleCharacter}
		f.all = append(f.all, f.Character)
	} else {
		// no character type: an empty one, so domain lookups never meet a nil item
		f.Character = &Item{Type: &comm.DogmaType{}, Role: RoleCharacter}
	}

	level := comm.DogmaAttributeNamed(skillLevelName)
	for _, skillType := range comm.DogmaSkills() {
		skill := &Item{Type: skillType, Role: RoleSkill}
		if level != nil {
			skill.base = map[int]float64{level.AttributeID: maxSkillLevel}
		}
		f.Skills = append(f.Skills, skill)
		f.skills[skillType.TypeID] = skill
		f.all = append(f.all, skill)
	}

	for _, implantID := range in.Implants {
		implantType := comm.DogmaTypeOf(implantID)
		if implantType == nil {
			f.Unknown = append(f.Unknown, implantID)
			continue
		}
		implant := &Item{Type: implantType, Role: RoleImplant}
		f.Implants = append(f.Implants, implant)
		f.all = append(f.all, implant)
	}

	f.Ship = &Item{Type: hull, Role: RoleHull}
	f.all = append(f.all, f.Ship)

	if in.Mode != 0 {
		if modeType := comm.DogmaTypeOf(in.Mode); modeType != nil {
			f.Mode = &Item{Type: modeType, Role: RoleMode}
			f.all = append(f.all, f.Mode)
		} else {
			f.Unknown = append(f.Unknown, in.Mode)
		}
	}

	for _, fitted := range in.Modules {
		moduleType := comm.DogmaTypeOf(fitted.Type)
		if moduleType == nil {
			f.Unknown = append(f.Unknown, fitted.Type)
			continue
		}
		role := RoleModule
		if fitted.Slot == comm.SlotSubsystem {
			role = RoleSubsystem
		}
		module := &Item{Type: moduleType, Role: role, Slot: fitted.Slot, State: StateActive}
		if fitted.State != moduleStateActive && brieflyActive(moduleType) {
			module.State = StateOnline
		}
		f.Modules = append(f.Modules, module)
		f.all = append(f.all, module)

		if fitted.Charge == 0 {
			continue
		}
		chargeType := comm.DogmaTypeOf(fitted.Charge)
		if chargeType == nil {
			f.Unknown = append(f.Unknown, fitted.Charge)
			continue
		}
		charge := &Item{Type: chargeType, Role: RoleCharge, Slot: fitted.Slot, State: StateActive, Module: module}
		module.Charge = charge
		f.all = append(f.all, charge)
	}
	return f, nil
}

/*
	Attr works out an attribute of an item in the fit, with every modifier that reaches it, capped by its max attribute

Each attribute starts at its base value (the type's, else the attribute's default) and the modifiers reaching it are applied in stages: pre-assign, then the multiplications (operations 0 and 1), additions (2), subtractions (3), the post multiplications (4, 5, and 6, a percent), and post-assign last. When several modifiers assign in one stage the last one added wins. Modifiers that are stacking penalized are chained as stackingChain says. A modifier's value is the source's own attribute, itself worked out. A hull or subsystem bonus is multiplied by the level of the skill it belongs to (comm.DogmaBonusSkill), at V.

Reading an attribute that is being worked out (a loop in the data) gives its base value, and nothing that touched the loop is kept, so it can't loop forever or leave a wrong number behind. A Fit is not safe for use by several goroutines at once.

Parameters:
  - item: an item of this fit
  - attributeID: the attribute

Returns:
  - float64: the value, or the attribute's default if the type doesn't list it
*/
func (f *Fit) Attr(item *Item, attributeID int) float64 {
	f.build()
	return f.attr(f.cache, item, attributeID)
}

/*
	AttrNamed is Attr by the attribute's SDE name, ie shieldCapacity

Parameters:
  - item: an item of this fit
  - name: the attribute's name

Returns:
  - float64: the value, 0 if dogma.json has no attribute of that name
*/
func (f *Fit) AttrNamed(item *Item, name string) float64 {
	attribute := comm.DogmaAttributeNamed(name)
	if attribute == nil {
		return 0
	}
	return f.Attr(item, attribute.AttributeID)
}

/*
	AttrExcluding works out an attribute as Attr does, leaving out every modifier from one source, ie the ship's resonances without the Reactive Armor Hardener that adapts to them. It is not kept.

It works in a cache of its own that skips the source's modifiers, so everything the attribute depends on is worked out again without them (an attribute the source never reaches is worked out again too). That costs one pass over the attribute's dependencies for each call, which is small beside a whole fit.

Parameters:
  - item: an item of this fit
  - attributeID: the attribute
  - source: the item whose modifiers are left out

Returns:
  - float64: the value
*/
func (f *Fit) AttrExcluding(item *Item, attributeID int, source *Item) float64 {
	f.build()
	return f.attr(newAttrCache(source), item, attributeID)
}

/*
	AddHand adds modifiers written in code, before anything is worked out

Parameters:
  - modifiers: the modifiers
*/
func (f *Fit) AddHand(modifiers ...HandModifier) {
	f.build()
	for i := range modifiers {
		hand := modifiers[i]
		if hand.Target == nil {
			continue
		}
		key := attrKey{hand.Target, hand.Attribute}
		f.index[key] = append(f.index[key], modEntry{source: hand.Source, operation: hand.Operation, hand: &hand})
	}
	// anything already worked out may have missed these
	f.cache = newAttrCache(nil)
}

/*
	Applies reports whether one of an item's effects is in force: a passive effect always, an online one when online or more, an active one when active or more, an overload one only when overheated. Effects of anything but a module or charge always apply.

The effect named online is an active effect in the data but means online, so it applies at online. A charge's effects apply while its module is at least online. A target effect (category 2, ie a remote repairer) applies when active, so the stats can count what it does, but build never takes modifiers from it. The area, dungeon, and system categories (3, 6, 7) never apply.

Parameters:
  - item: an item of this fit
  - effect: one of its type's effects

Returns:
  - bool: true if the effect applies
*/
func (f *Fit) Applies(item *Item, effect *comm.DogmaEffect) bool {
	if item.Role != RoleModule && item.Role != RoleCharge {
		return true
	}
	state := item.State
	if item.Role == RoleCharge {
		if item.Module == nil {
			return false
		}
		return item.Module.State >= StateOnline
	}
	category := effect.Category
	if effect.Name == onlineEffectName {
		category = categoryOnline
	}
	switch category {
	case categoryPassive:
		return true
	case categoryOnline:
		return state >= StateOnline
	case categoryActive, categoryTarget:
		return state >= StateActive
	case categoryOverload:
		return state >= StateOverheated
	}
	return false
}

const (
	skillLevelName   = "skillLevel"
	maxSkillLevel    = 5.0
	onlineEffectName = "online"

	categoryPassive  = 0
	categoryActive   = 1
	categoryTarget   = 2
	categoryOnline   = 4
	categoryOverload = 5

	opPreAssign   = -1
	opPreMul      = 0
	opPreDiv      = 1
	opModAdd      = 2
	opModSub      = 3
	opPostMul     = 4
	opPostDiv     = 5
	opPostPercent = 6
	opPostAssign  = 7

	stackingSteepness = 2.67
)

// requiredSkillNames are the attributes listing the skills a type requires
var requiredSkillNames = []string{"requiredSkill1", "requiredSkill2", "requiredSkill3", "requiredSkill4", "requiredSkill5", "requiredSkill6"}

// attrKey is one attribute of one item
type attrKey struct {
	item      *Item
	attribute int
}

// modEntry is one modifier reaching an item's attribute: either from the data (modifying is the source attribute) or by hand
type modEntry struct {
	source    *Item
	modifying int
	operation int
	hand      *HandModifier

	unpenalized bool // its effect is listed in dogma.json's unpenalized for this attribute
}

// attrCache holds worked out attributes for one way of reading the fit (all modifiers, or all but one source's)
type attrCache struct {
	exclude    *Item
	values     map[attrKey]float64
	inProgress map[attrKey]bool
	cycles     int // how many times a loop was hit, so what touched one is not kept
}

func newAttrCache(exclude *Item) *attrCache {
	return &attrCache{exclude: exclude, values: map[attrKey]float64{}, inProgress: map[attrKey]bool{}}
}

/*
	build finds every modifier in force once, by the item and attribute each changes

Which effects apply depends only on the module states, which don't change after NewFit, so this runs on first use.

Targets, by the modifier's func and domain (the domain says where, the func says what in it):
  - ItemModifier: itemID is the source itself, shipID (and structureID) the hull, charID the character, otherID the source's charge (for a module) or module (for a charge).
  - Location modifiers (LocationModifier, LocationGroupModifier, LocationRequiredSkillModifier): the items fitted in the domain. For shipID (and structureID) that is every module and subsystem, not the hull, a mode, or charges (a charge is reached through its module's otherID, or by an owner modifier, as Pyfa does). For charID it is the same fitted modules and subsystems plus charges, and the implants, which an implant set's bonus multiplies; never the skills. For itemID it is the source's charge. otherID has no location.
  - OwnerRequiredSkillModifier: whatever the character owns that requires the skill: modules, subsystems, and charges, not the hull, whatever the domain says.
  - EffectStopper and the target domains act on something else and are left out.
*/
func (f *Fit) build() {
	if f.index != nil {
		return
	}
	f.index = map[attrKey][]modEntry{}
	for _, source := range f.all {
		for _, effectID := range source.Type.Effects {
			effect := comm.DogmaEffectOf(effectID)
			if effect == nil || !f.Applies(source, effect) || actsOnOthers(effect) {
				continue
			}
			for _, modifier := range effect.Modifiers {
				for _, target := range f.targets(source, modifier) {
					key := attrKey{target, modifier.Modified}
					f.index[key] = append(f.index[key], modEntry{source: source, modifying: modifier.Modifying, operation: modifier.Operation,
						unpenalized: comm.DogmaUnpenalized(effectID, modifier.Modified)})
				}
			}
		}
	}
}

// targets lists the items one modifier of a source's effect changes
func (f *Fit) targets(source *Item, modifier comm.DogmaModifier) []*Item {
	if modifier.Func == "OwnerRequiredSkillModifier" {
		return f.requiring(f.fitted(true), modifier.Skill)
	}

	var domainItem *Item
	var location []*Item
	switch modifier.Domain {
	case "itemID":
		domainItem = source
		if source.Charge != nil {
			location = []*Item{source.Charge}
		}
	case "shipID", "structureID":
		domainItem = f.Ship
		location = f.fitted(false)
	case "charID":
		domainItem = f.Character
		// implants too, as an implant set's bonus multiplies the other implants of the set
		location = append(f.fitted(true), f.Implants...)
	case "otherID":
		domainItem = source.Charge
		if source.Role == RoleCharge {
			domainItem = source.Module
		}
	default:
		return nil
	}

	switch modifier.Func {
	case "ItemModifier":
		if domainItem == nil {
			return nil
		}
		return []*Item{domainItem}
	case "LocationModifier":
		return location
	case "LocationGroupModifier":
		var matched []*Item
		for _, item := range location {
			if item.Type.Group == modifier.Group {
				matched = append(matched, item)
			}
		}
		return matched
	case "LocationRequiredSkillModifier":
		return f.requiring(location, modifier.Skill)
	}
	return nil
}

// fitted lists the modules and subsystems, and their charges too if withCharges
func (f *Fit) fitted(withCharges bool) []*Item {
	items := make([]*Item, 0, len(f.Modules)*2)
	for _, module := range f.Modules {
		items = append(items, module)
		if withCharges && module.Charge != nil {
			items = append(items, module.Charge)
		}
	}
	return items
}

// requiring keeps the items whose type requires a skill, by its requiredSkill1 to requiredSkill6 attributes
func (f *Fit) requiring(items []*Item, skill int) []*Item {
	var matched []*Item
	for _, item := range items {
		for _, name := range requiredSkillNames {
			attribute := comm.DogmaAttributeNamed(name)
			if attribute == nil {
				continue
			}
			if value, ok := item.Type.Attributes[attribute.AttributeID]; ok && int(value) == skill {
				matched = append(matched, item)
				break
			}
		}
	}
	return matched
}

// attr is Attr in one cache
func (f *Fit) attr(cache *attrCache, item *Item, attributeID int) float64 {
	key := attrKey{item, attributeID}
	if value, ok := cache.values[key]; ok {
		return value
	}
	base := f.baseValue(item, attributeID)
	if cache.inProgress[key] {
		cache.cycles++
		return base
	}
	cache.inProgress[key] = true
	cyclesBefore := cache.cycles
	value := f.compute(cache, key, base)
	delete(cache.inProgress, key)
	if cache.cycles == cyclesBefore {
		cache.values[key] = value
	}
	return value
}

// baseValue is an item's attribute before modifiers: set on the item, else on its type, else the attribute's default
func (f *Fit) baseValue(item *Item, attributeID int) float64 {
	if value, ok := item.base[attributeID]; ok {
		return value
	}
	if value, ok := item.Type.Attributes[attributeID]; ok {
		return value
	}
	if attribute := comm.DogmaAttributeOf(attributeID); attribute != nil {
		return attribute.Default
	}
	return 0
}

// multiplier is one multiplying modifier, ready to be chained or multiplied in
type multiplier struct {
	factor    float64
	penalized bool
}

// compute applies the modifiers reaching key to its base value
func (f *Fit) compute(cache *attrCache, key attrKey, base float64) float64 {
	attribute := comm.DogmaAttributeOf(key.attribute)
	value := base

	var preAssign, postAssign *float64
	var adds, subs float64
	var pre, post []multiplier

	for _, entry := range f.index[key] {
		if cache.exclude != nil && entry.source == cache.exclude {
			continue
		}
		modValue := f.entryValue(cache, entry)
		switch entry.operation {
		case opPreAssign:
			preAssign = &modValue
		case opPostAssign:
			postAssign = &modValue
		case opModAdd:
			adds += modValue
		case opModSub:
			subs += modValue
		case opPreMul, opPreDiv, opPostMul, opPostDiv, opPostPercent:
			factor := modValue
			switch entry.operation {
			case opPreDiv, opPostDiv:
				if modValue == 0 {
					continue
				}
				factor = 1 / modValue
			case opPostPercent:
				factor = 1 + modValue/100
			}
			m := multiplier{factor: factor, penalized: attribute != nil && !attribute.Stackable && !entry.unpenalized && entry.source != nil &&
				(entry.source.Role == RoleModule || entry.source.Role == RoleCharge)}
			if entry.operation == opPreMul || entry.operation == opPreDiv {
				pre = append(pre, m)
			} else {
				post = append(post, m)
			}
		}
	}

	if preAssign != nil {
		value = *preAssign
	}
	value *= stackingProduct(pre)
	value += adds
	value -= subs
	value *= stackingProduct(post)
	if postAssign != nil {
		value = *postAssign
	}

	if attribute != nil && attribute.Max != 0 {
		if limit := f.attr(cache, key.item, attribute.Max); value > limit {
			value = limit
		}
	}
	return value
}

// entryValue is what one modifier adds: a hand modifier's own value, else the source's modifying attribute, times the skill level for a hull or subsystem bonus
func (f *Fit) entryValue(cache *attrCache, entry modEntry) float64 {
	if entry.hand != nil {
		return entry.hand.Value(f)
	}
	value := f.attr(cache, entry.source, entry.modifying)
	if entry.source.Role == RoleHull || entry.source.Role == RoleSubsystem {
		if skillID := comm.DogmaBonusSkill(entry.modifying); skillID != 0 {
			level := maxSkillLevel
			if skill := f.skills[skillID]; skill != nil {
				if levelAttribute := comm.DogmaAttributeNamed(skillLevelName); levelAttribute != nil {
					level = f.attr(cache, skill, levelAttribute.AttributeID)
				}
			}
			value *= level
		}
	}
	return value
}

/*
	stackingProduct multiplies the factors of one stage together, stacking penalizing those that call for it

Penalized factors above 1 and below 1 are chained apart. Each chain is sorted by how far it is from 1, strongest first, and the i-th (from 0) counts for 1 + (factor-1) * exp(-(i/2.67)^2).
*/
func stackingProduct(factors []multiplier) float64 {
	product := 1.0
	var boosts, cuts []float64
	for _, m := range factors {
		switch {
		case !m.penalized:
			product *= m.factor
		case m.factor > 1:
			boosts = append(boosts, m.factor)
		case m.factor < 1:
			cuts = append(cuts, m.factor)
		}
	}
	for _, chain := range [][]float64{boosts, cuts} {
		sort.Slice(chain, func(i, j int) bool { return math.Abs(chain[i]-1) > math.Abs(chain[j]-1) })
		for i, factor := range chain {
			product *= 1 + (factor-1)*math.Exp(-math.Pow(float64(i)/stackingSteepness, 2))
		}
	}
	return product
}

// actsOnOthers is true for the target, area, dungeon, and system effects, whose modifiers change something other than this fit
func actsOnOthers(effect *comm.DogmaEffect) bool {
	switch effect.Category {
	case categoryTarget, 3, 6, 7:
		return effect.Name != onlineEffectName
	}
	return false
}
