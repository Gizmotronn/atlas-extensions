// Package advice provides device-specific astrophotography guidance.
//
// The preset data in presets.json is ported from atlas/src/lib/devicePresets.ts
// (the same device enum as the shared PocketBase `users.device_models` field,
// see backend/migrations/33_atlas_device_models.go). It is a manual copy, not
// a live sync — Atlas's TypeScript presets are the source of truth; update
// both when device guidance changes.
package advice

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
)

//go:embed presets.json
var presetsJSON []byte

// Preset is camera guidance for a specific phone model.
type Preset struct {
	Value        string   `json:"value"`
	Label        string   `json:"label"`
	Instructions []string `json:"instructions"`
}

var presets map[string]Preset

func init() {
	var raw map[string]struct {
		Label        string   `json:"label"`
		Instructions []string `json:"instructions"`
	}
	if err := json.Unmarshal(presetsJSON, &raw); err != nil {
		panic(fmt.Errorf("advice: invalid embedded presets.json: %w", err))
	}
	presets = make(map[string]Preset, len(raw))
	for value, p := range raw {
		presets[value] = Preset{Value: value, Label: p.Label, Instructions: p.Instructions}
	}
}

// ForDevice returns the camera preset for a device_models enum value
// (e.g. "iphone_16_pro"), falling back to "other" if the device is unknown
// or empty.
func ForDevice(deviceModel string) Preset {
	if p, ok := presets[deviceModel]; ok {
		return p
	}
	return presets["other"]
}

// All returns every known device preset, keyed by device_models value.
func All() map[string]Preset {
	out := make(map[string]Preset, len(presets))
	maps.Copy(out, presets)
	return out
}
