//go:build windows

package local

import (
	"strings"
	"testing"

	"github.com/mutagen-io/mutagen/pkg/synchronization/core"
)

func TestFilterUnsupportedTransitionsFiltersWindowsReservedCharacter(t *testing.T) {
	old := &core.Entry{
		Kind: core.EntryKind_Directory,
		Contents: map[string]*core.Entry{
			"kept.php": {Kind: core.EntryKind_File, Digest: []byte{1}},
		},
	}
	new := &core.Entry{
		Kind: core.EntryKind_Directory,
		Contents: map[string]*core.Entry{
			"kept.php":                            {Kind: core.EntryKind_File, Digest: []byte{2}},
			"primary|global|plugin-list.php":      {Kind: core.EntryKind_File, Digest: []byte{3}},
			"primary|frontend|hyva|plugin.php":    {Kind: core.EntryKind_File, Digest: []byte{4}},
			"primary|frontend|base|di-config.php": {Kind: core.EntryKind_File, Digest: []byte{5}},
		},
	}

	filtered, problems, err := (&endpoint{}).FilterUnsupportedTransitions([]*core.Change{{
		Path: "metadata",
		Old:  old,
		New:  new,
	}})
	if err != nil {
		t.Fatal("filter failed:", err)
	}
	if len(problems) != 3 {
		t.Fatalf("unexpected problem count: %d", len(problems))
	}
	for _, problem := range problems {
		if !strings.HasPrefix(problem.Path, "metadata/primary|") {
			t.Error("unexpected problem path:", problem.Path)
		}
		if !strings.Contains(problem.Error, "reserved character") {
			t.Error("unexpected problem error:", problem.Error)
		}
	}
	if len(filtered) != 1 {
		t.Fatalf("unexpected filtered transition count: %d", len(filtered))
	}
	expected := &core.Entry{
		Kind: core.EntryKind_Directory,
		Contents: map[string]*core.Entry{
			"kept.php": {Kind: core.EntryKind_File, Digest: []byte{2}},
		},
	}
	if !filtered[0].New.Equal(expected, true) {
		t.Error("reserved-character content not filtered from transition")
	}
}
