package identify

// catalogObject is a fixed-position entry (star or deep-sky object) — RA/Dec
// don't change meaningfully on human timescales, unlike Sun/Moon/planets.
type catalogObject struct {
	Key     string
	Name    string
	Type    string // "star" | "deep_sky"
	Context string
	RADeg   float64
	DecDeg  float64
}

// catalog is a small, curated set of bright/notable naked-eye objects.
// Extend this list rather than pulling in a full star catalog — Fieldwork's
// scan is meant to identify "what you can plausibly see and recognize",
// not do plate-solving against thousands of faint stars.
var catalog = []catalogObject{
	{Key: "sirius", Name: "Sirius", Type: "star", RADeg: 101.287, DecDeg: -16.716,
		Context: "The brightest star in the night sky, in Canis Major."},
	{Key: "vega", Name: "Vega", Type: "star", RADeg: 279.234, DecDeg: 38.784,
		Context: "A bright blue-white star in Lyra, one corner of the Summer Triangle."},
	{Key: "betelgeuse", Name: "Betelgeuse", Type: "star", RADeg: 88.793, DecDeg: 7.407,
		Context: "A red supergiant marking Orion's shoulder, nearing the end of its life."},
	{Key: "rigel", Name: "Rigel", Type: "star", RADeg: 78.634, DecDeg: -8.202,
		Context: "A blue supergiant marking Orion's foot, one of the most luminous nearby stars."},
	{Key: "polaris", Name: "Polaris", Type: "star", RADeg: 37.955, DecDeg: 89.264,
		Context: "The North Star — within about a degree of true celestial north."},
	{Key: "arcturus", Name: "Arcturus", Type: "star", RADeg: 213.915, DecDeg: 19.182,
		Context: "An orange giant in Boötes, one of the brightest stars visible from Earth."},
	{Key: "capella", Name: "Capella", Type: "star", RADeg: 79.172, DecDeg: 45.998,
		Context: "A bright yellow star system in Auriga."},
	{Key: "aldebaran", Name: "Aldebaran", Type: "star", RADeg: 68.980, DecDeg: 16.509,
		Context: "An orange giant marking the eye of Taurus, near the Hyades cluster."},
	{Key: "antares", Name: "Antares", Type: "star", RADeg: 247.352, DecDeg: -26.432,
		Context: "A red supergiant marking the heart of Scorpius."},
	{Key: "andromeda_galaxy", Name: "Andromeda Galaxy (M31)", Type: "deep_sky", RADeg: 10.685, DecDeg: 41.269,
		Context: "The nearest large galaxy to the Milky Way, faintly visible to the naked eye under dark skies."},
	{Key: "orion_nebula", Name: "Orion Nebula (M42)", Type: "deep_sky", RADeg: 83.822, DecDeg: -5.391,
		Context: "A bright star-forming region visible as a fuzzy patch below Orion's belt."},
	{Key: "pleiades", Name: "Pleiades (M45)", Type: "deep_sky", RADeg: 56.750, DecDeg: 24.117,
		Context: "The Seven Sisters — a bright, compact open star cluster in Taurus."},
}

var solarSystemObjects = []struct {
	Key     string
	Name    string
	Context string
}{
	{Key: "moon", Name: "the Moon", Context: "Earth's only natural satellite."},
	{Key: "mercury", Name: "Mercury", Context: "The innermost planet — small and only visible low near the horizon soon after sunset or before sunrise."},
	{Key: "venus", Name: "Venus", Context: "The brightest planet in the sky, often called the morning or evening star."},
	{Key: "mars", Name: "Mars", Context: "The red planet — recognizable by its distinct reddish-orange color."},
	{Key: "jupiter", Name: "Jupiter", Context: "The largest planet — bright and steady, often outshining every star."},
	{Key: "saturn", Name: "Saturn", Context: "The ringed planet — steady, pale gold light; rings need a telescope to resolve."},
}
