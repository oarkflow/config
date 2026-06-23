package config

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	json "github.com/oarkflow/fastjson"
)

type SourceMeta struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Required bool      `json:"required"`
	LoadedAt time.Time `json:"loaded_at"`
	Error    string    `json:"error,omitempty"`
}

type Warning struct{ Path, Message string }

type Snapshot struct {
	Version  uint64       `json:"version"`
	Hash     string       `json:"hash"`
	LoadedAt time.Time    `json:"loaded_at"`
	Tree     Tree         `json:"tree"`
	Sources  []SourceMeta `json:"sources"`
	Warnings []Warning    `json:"warnings,omitempty"`
}

func newSnapshot(version uint64, tree Tree, sources []SourceMeta, warnings []Warning) *Snapshot {
	cp := tree.Clone()
	return &Snapshot{Version: version, Hash: HashTree(cp), LoadedAt: time.Now(), Tree: cp, Sources: append([]SourceMeta(nil), sources...), Warnings: append([]Warning(nil), warnings...)}
}
func HashTree(t Tree) string {
	flat := t.Flatten()
	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([][2]any, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, [2]any{k, flat[k]})
	}
	b, _ := json.Marshal(pairs)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
