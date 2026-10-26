// Package staticdata embeds the static EVE data json files so they ship inside the binary
package staticdata

import "embed"

//go:embed *.json
var Files embed.FS
