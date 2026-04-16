package fields

type Preset int

const (
	PresetDefault Preset = iota
	PresetWithCreds
	PresetFull
)

func (p Preset) String() string {
	switch p {
	case PresetWithCreds:
		return "with-credentials"
	case PresetFull:
		return "full"
	default:
		return "default"
	}
}

type PresetSeed struct {
	Entries     map[string]bool
	Credentials bool
}

func PresetSelection(items []ProfileEntry, p Preset) PresetSeed {
	seed := PresetSeed{Entries: map[string]bool{}}
	for _, it := range items {
		switch {
		case it.Category == Shared:
			seed.Entries[it.Name] = true
		case p == PresetWithCreds && it.Name == ".claude.json":
			seed.Entries[it.Name] = true
		case p == PresetFull && it.Category == Isolated:
			seed.Entries[it.Name] = true
		}
	}
	seed.Credentials = p == PresetWithCreds || p == PresetFull
	return seed
}
