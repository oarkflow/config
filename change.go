package config

import "reflect"

type ChangeKind string

const (
	Added   ChangeKind = "added"
	Removed ChangeKind = "removed"
	Updated ChangeKind = "updated"
)

type PathChange struct {
	Path      string
	Kind      ChangeKind
	OldValue  any
	NewValue  any
	Sensitive bool
}
type Change struct {
	OldVersion uint64
	NewVersion uint64
	OldHash    string
	NewHash    string
	Paths      []PathChange
}

func (c Change) Empty() bool { return len(c.Paths) == 0 }
func (c Change) Changed(prefix string) bool {
	for _, p := range c.Paths {
		if p.Path == prefix || len(p.Path) > len(prefix) && p.Path[:len(prefix)] == prefix && p.Path[len(prefix)] == '.' {
			return true
		}
	}
	return false
}
func (c Change) Redacted() Change {
	out := c
	out.Paths = append([]PathChange(nil), c.Paths...)
	for i := range out.Paths {
		if out.Paths[i].Sensitive {
			out.Paths[i].OldValue = "[REDACTED]"
			out.Paths[i].NewValue = "[REDACTED]"
		}
	}
	return out
}
func diffSnapshots(old, next *Snapshot, secretPaths map[string]bool, matcher *SensitiveMatcher) Change {
	of := old.Tree.Flatten()
	nf := next.Tree.Flatten()
	ch := Change{OldVersion: old.Version, NewVersion: next.Version, OldHash: old.Hash, NewHash: next.Hash}
	seen := map[string]bool{}
	for k, ov := range of {
		seen[k] = true
		nv, ok := nf[k]
		if !ok {
			ch.Paths = append(ch.Paths, PathChange{Path: k, Kind: Removed, OldValue: ov, Sensitive: isSensitivePath(k, secretPaths, matcher)})
			continue
		}
		if !reflect.DeepEqual(ov, nv) {
			ch.Paths = append(ch.Paths, PathChange{Path: k, Kind: Updated, OldValue: ov, NewValue: nv, Sensitive: isSensitivePath(k, secretPaths, matcher)})
		}
	}
	for k, nv := range nf {
		if !seen[k] {
			ch.Paths = append(ch.Paths, PathChange{Path: k, Kind: Added, NewValue: nv, Sensitive: isSensitivePath(k, secretPaths, matcher)})
		}
	}
	return ch
}
