package comm_test

import (
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/stretchr/testify/assert"
)

func flagsFS(contents string) fstest.MapFS {
	return fstest.MapFS{"inventory_flags.json": &fstest.MapFile{Data: []byte(contents)}}
}

const testFlags = `{"slots": [
	{"slot": "low", "first_flag": 11, "last_flag": 18, "name": "Low slots"},
	{"slot": "cargo", "first_flag": 5, "last_flag": 5, "name": "Cargo hold"},
	{"slot": "ignored", "first_flag": 155, "last_flag": 155, "name": "Fleet hangar"}
]}`

func TestLoadInventoryFlags(t *testing.T) {
	t.Run("reads slots and positions", func(t *testing.T) {
		assert.NoError(t, comm.LoadInventoryFlags(flagsFS(testFlags), "inventory_flags.json"))

		slot, position := comm.FitSlotOf(11)
		assert.Equal(t, comm.SlotLow, slot)
		assert.Equal(t, 0, position)

		slot, position = comm.FitSlotOf(13)
		assert.Equal(t, comm.SlotLow, slot)
		assert.Equal(t, 2, position)

		slot, position = comm.FitSlotOf(155)
		assert.Equal(t, comm.SlotIgnored, slot)
		assert.Equal(t, 0, position)
		assert.Equal(t, "Fleet hangar", comm.FitSlotName(155))
	})

	t.Run("an unlisted flag is unknown", func(t *testing.T) {
		assert.NoError(t, comm.LoadInventoryFlags(flagsFS(testFlags), "inventory_flags.json"))

		slot, position := comm.FitSlotOf(19)
		assert.Equal(t, comm.SlotUnknown, slot)
		assert.Equal(t, 0, position)
		assert.Equal(t, "Unknown (flag 19)", comm.FitSlotName(19))
	})

	failures := []struct {
		name     string
		contents string
	}{
		{"overlapping ranges", `{"slots": [{"slot": "low", "first_flag": 11, "last_flag": 18, "name": "Low"}, {"slot": "mid", "first_flag": 18, "last_flag": 26, "name": "Mid"}]}`},
		{"an unknown slot value", `{"slots": [{"slot": "lowish", "first_flag": 11, "last_flag": 18, "name": "Low"}]}`},
		{"the unknown slot itself", `{"slots": [{"slot": "unknown", "first_flag": 11, "last_flag": 18, "name": "Low"}]}`},
		{"a range that ends before it starts", `{"slots": [{"slot": "low", "first_flag": 18, "last_flag": 11, "name": "Low"}]}`},
		{"invalid json", `{"slots": [`},
	}
	for _, c := range failures {
		t.Run("rejects "+c.name+" and keeps the previous flags", func(t *testing.T) {
			assert.NoError(t, comm.LoadInventoryFlags(flagsFS(testFlags), "inventory_flags.json"))

			assert.Error(t, comm.LoadInventoryFlags(flagsFS(c.contents), "inventory_flags.json"))

			slot, _ := comm.FitSlotOf(5)
			assert.Equal(t, comm.SlotCargo, slot)
			slot, _ = comm.FitSlotOf(20)
			assert.Equal(t, comm.SlotUnknown, slot)
		})
	}

	t.Run("a missing file is an error", func(t *testing.T) {
		assert.Error(t, comm.LoadInventoryFlags(fstest.MapFS{}, "inventory_flags.json"))
	})
}

func TestEmbeddedInventoryFlags(t *testing.T) {
	assert.NoError(t, comm.LoadInventoryFlags(staticdata.Files, "inventory_flags.json"))

	cases := []struct {
		flag     int
		slot     comm.FitSlot
		position int
		name     string
	}{
		{11, comm.SlotLow, 0, "Low slots"},
		{34, comm.SlotHigh, 7, "High slots"},
		{19, comm.SlotMid, 0, "Mid slots"},
		{94, comm.SlotRig, 2, "Rig slots"},
		{128, comm.SlotSubsystem, 3, "Subsystems"},
		{165, comm.SlotService, 1, "Service slots"},
		{87, comm.SlotDroneBay, 0, "Drone bay"},
		{158, comm.SlotFighterBay, 0, "Fighter bay"},
		{161, comm.SlotFighterTube, 2, "Fighter tubes"},
		{88, comm.SlotBooster, 0, "Boosters"},
		{89, comm.SlotImplant, 0, "Implants"},
		{5, comm.SlotCargo, 0, "Cargo hold"},
		{143, comm.SlotCargo, 0, "Ammo hold"},
		{155, comm.SlotIgnored, 0, "Fleet hangar"},
		{90, comm.SlotIgnored, 0, "Ship maintenance bay"},
		{9999, comm.SlotUnknown, 0, "Unknown (flag 9999)"},
	}
	for _, c := range cases {
		slot, position := comm.FitSlotOf(c.flag)
		assert.Equal(t, c.slot, slot, "flag %d", c.flag)
		assert.Equal(t, c.position, position, "flag %d", c.flag)
		assert.Equal(t, c.name, comm.FitSlotName(c.flag), "flag %d", c.flag)
	}
}
