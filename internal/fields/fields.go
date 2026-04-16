package fields

import "github.com/vika2603/ccs/internal/config"

type Category int

const (
	Isolated Category = iota
	Shared
	Transient
)

func (c Category) String() string {
	switch c {
	case Shared:
		return "shared"
	case Transient:
		return "transient"
	default:
		return "isolated"
	}
}

type Kind int

const (
	KindDir Kind = iota
	KindFile
)

func (k Kind) String() string {
	switch k {
	case KindFile:
		return "file"
	default:
		return "dir"
	}
}

type Classification struct {
	Name     string
	Category Category
	Kind     Kind
}

type ExportMode int

const (
	ExportDefault ExportMode = iota
	ExportWithCredentials
	ExportFull
)

type Entry struct {
	Name string
	Path string
	Kind Kind
}

type Registry struct {
	shared     map[string]Classification
	isolated   map[string]Classification
	transient  map[string]Classification
	known      map[string]Classification
	sharedList []Classification
}

func NewRegistry(f config.Fields) *Registry {
	r := &Registry{
		shared:    set(f.Shared, Shared),
		isolated:  set(f.Isolated, Isolated),
		transient: set(f.Transient, Transient),
		known:     map[string]Classification{},
	}
	for _, bucket := range []map[string]Classification{r.shared, r.isolated, r.transient} {
		for name, class := range bucket {
			r.known[name] = class
			if class.Category == Shared {
				r.sharedList = append(r.sharedList, class)
			}
		}
	}
	return r
}

func set(vs []string, category Category) map[string]Classification {
	m := make(map[string]Classification, len(vs))
	for _, v := range vs {
		m[v] = Classification{Name: v, Category: category, Kind: inferKind(v)}
	}
	return m
}

func inferKind(name string) Kind {
	switch name {
	case "CLAUDE.md",
		"settings.json",
		"history.jsonl",
		".credentials.json",
		".claude.json",
		"mcp-needs-auth-cache.json",
		"policy-limits.json":
		return KindFile
	default:
		return KindDir
	}
}

func (r *Registry) Describe(name string) Classification {
	if class, ok := r.shared[name]; ok {
		return class
	}
	if class, ok := r.transient[name]; ok {
		return class
	}
	if class, ok := r.isolated[name]; ok {
		return class
	}
	return Classification{Name: name, Category: Isolated, Kind: inferKind(name)}
}

func (r *Registry) Classify(name string) Category {
	return r.Describe(name).Category
}

func (r *Registry) IsUnknown(name string) bool {
	_, ok := r.known[name]
	return !ok
}

func (r *Registry) Shared() []Classification {
	out := make([]Classification, len(r.sharedList))
	copy(out, r.sharedList)
	return out
}

func (r *Registry) All() map[string]Classification {
	out := make(map[string]Classification, len(r.known))
	for name, class := range r.known {
		out[name] = class
	}
	return out
}
