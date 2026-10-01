package fields

import (
	"maps"
	"path/filepath"

	"github.com/vika2603/ccs/internal/config"
)

type Category int

const (
	Isolated Category = iota
	Shared
)

func (c Category) String() string {
	switch c {
	case Shared:
		return "shared"
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

type Registry struct {
	shared     map[string]Classification
	isolated   map[string]Classification
	known      map[string]Classification
	sharedList []Classification
}

func NewRegistry(cfg config.Config) *Registry {
	r := &Registry{
		shared:   set(cfg.Shared, Shared),
		isolated: set(cfg.Isolated, Isolated),
		known:    map[string]Classification{},
	}
	for _, name := range cfg.Export.Exclude {
		if _, ok := r.shared[name]; ok {
			continue
		}
		if _, ok := r.isolated[name]; ok {
			continue
		}
		r.isolated[name] = Classification{Name: name, Category: Isolated, Kind: inferKind(name)}
	}
	for _, bucket := range []map[string]Classification{r.shared, r.isolated} {
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

var kindOverrides = map[string]Kind{
	"CLAUDE.md":                 KindFile,
	"settings.json":             KindFile,
	"history.jsonl":             KindFile,
	".credentials.json":         KindFile,
	".claude.json":              KindFile,
	"mcp-needs-auth-cache.json": KindFile,
	"policy-limits.json":        KindFile,
}

func inferKind(name string) Kind {
	if k, ok := kindOverrides[name]; ok {
		return k
	}
	if filepath.Ext(name) != "" {
		return KindFile
	}
	return KindDir
}

func (r *Registry) Describe(name string) Classification {
	if class, ok := r.shared[name]; ok {
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
	maps.Copy(out, r.known)
	return out
}
