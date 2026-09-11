package api

import "github.com/lynkfox/go_eve_sleuth/comm/types"

type TargetDisplay struct {
	Name              string
	Type              types.EntityType
	TargetImageUrl    string
	LeftMajorTitle    string
	LeftMajorValue    string
	RightMajorTitle   string
	RightMajorValue   string
	LeftDetailsTitle  string
	LeftDetailsValue  string
	RightDetailsTitle string
	RightDetailsValue string
}
