package retrieval

import (
	"maps"
	"time"
)

// systemNicknamesKey is the one row of sectionSystemNicknames
const systemNicknamesKey = "all"

// SystemNicknames are the user's names for systems shown everywhere, ie "Velvet Anchor". A war can have its own, see WarEdits.SystemNames.
type SystemNicknames struct {
	Names map[int]string `json:"names"`
}

/*
	SystemNicknamesOf returns the user's names for systems shown everywhere.

Returns:
  - map[int]string: a copy, by system id, never nil
*/
func (c *CachedEveIDs) SystemNicknamesOf() map[int]string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if saved := cachedAs[SystemNicknames](c, sectionSystemNicknames, c.Nicknames, systemNicknamesKey); saved != nil && saved.Names != nil {
		return maps.Clone(saved.Names)
	}
	return map[int]string{}
}

/*
	SetSystemNickname names a system everywhere. It is Permanent and one of userSections, so a clear leaves it unless told not to.

Parameters:
  - systemID: the system
  - nickname: its name, empty to remove it
*/
func (c *CachedEveIDs) SetSystemNickname(systemID int, nickname string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	names := map[int]string{}
	if saved := cachedAs[SystemNicknames](c, sectionSystemNicknames, c.Nicknames, systemNicknamesKey); saved != nil && saved.Names != nil {
		names = maps.Clone(saved.Names)
	}
	if nickname == "" {
		delete(names, systemID)
	} else {
		names[systemID] = nickname
	}
	put(c, sectionSystemNicknames, c.Nicknames, systemNicknamesKey, &CacheEntry{Value: &SystemNicknames{Names: names}, CachedAt: time.Now().UTC(), Permanent: true})
}
