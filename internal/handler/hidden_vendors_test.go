package handler

import (
	"testing"

	modelruntime "github.com/Tencent/WeKnora/internal/models/runtime"
)

// the editor must not offer a cloud this deployment cannot call
func TestVisibleVendorsDropsHiddenOnes(t *testing.T) {
	if len(hiddenVendors) == 0 {
		t.Fatal("no vendor is hidden; this test no longer guards anything")
	}
	got := map[string]bool{}
	for _, v := range visibleVendors(modelruntime.List()) {
		got[v.ID] = true
	}
	for id := range hiddenVendors {
		if got[id] {
			t.Errorf("hidden vendor %q is offered by the editor", id)
		}
	}
	if !got["openai"] {
		t.Error("visibleVendors dropped openai; the filter is too wide")
	}
}

// hiding is an editor-only view, so upstream specs and rows written earlier keep resolving
func TestHiddenVendorsStayRegistered(t *testing.T) {
	for id := range hiddenVendors {
		if _, ok := modelruntime.Get(id); !ok {
			t.Errorf("hidden vendor %q is no longer registered", id)
		}
	}
}
