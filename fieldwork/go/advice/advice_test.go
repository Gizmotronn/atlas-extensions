package advice

import "testing"

func TestForDeviceKnownAndUnknown(t *testing.T) {
	known := []string{
		"nothing_phone_1", "nothing_phone_2", "nothing_phone_2a", "nothing_phone_3",
		"iphone_15_pro", "iphone_16_pro", "pixel_8_pro", "pixel_9_pro",
		"galaxy_s24_ultra", "galaxy_s25_ultra", "other",
	}
	for _, v := range known {
		p := ForDevice(v)
		if p.Value != v {
			t.Errorf("ForDevice(%q).Value = %q, want %q", v, p.Value, v)
		}
		if len(p.Instructions) == 0 {
			t.Errorf("ForDevice(%q) has no instructions", v)
		}
	}

	unknown := ForDevice("some_future_phone")
	if unknown.Value != "other" {
		t.Errorf("ForDevice(unknown) = %q, want fallback %q", unknown.Value, "other")
	}
}

func TestAllReturnsEveryPreset(t *testing.T) {
	all := All()
	if len(all) < 11 {
		t.Errorf("expected at least 11 presets, got %d", len(all))
	}
}
