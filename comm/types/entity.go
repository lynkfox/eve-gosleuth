package types

type EntityType int

const (
	PILOT         EntityType = iota // 1
	ALLIANCE                        // 2
	CORPORATION                     // 3
	SYSTEM                          // 4
	ITEM                            // 5
	CONSTELLATION                   // 6
	REGION                          // 7
)

func (e EntityType) String() string {
	switch e {
	case PILOT:
		return "PILOT"
	case ALLIANCE:
		return "ALLIANCE"
	case CORPORATION:
		return "CORPORATION"
	case SYSTEM:
		return "SYSTEM"
	case ITEM:
		return "ITEM"
	case CONSTELLATION:
		return "CONSTELLATION"
	case REGION:
		return "REGION"
	default:
		return "UNKNOWN"
	}
}
