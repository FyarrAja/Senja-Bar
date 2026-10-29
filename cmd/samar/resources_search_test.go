//go:build windows

package main

import (
	"samar/internal/resources"
	"testing"
)

func TestResourceSearchKeepsOriginalRowsAndSelections(t *testing.T) {
	r := nativeResources{rows: []resourceRow{{path: "a", name: "ChatGPT.exe"}, {path: "b", name: "codex.exe"}, {path: "c", name: "Notes.exe"}}, config: resources.Config{Paths: []string{"a", "c"}}, offset: 6}
	r.setQuery(" CODEX ")
	if len(r.visibleRows()) != 1 || r.visibleRows()[0].path != "b" || r.offset != 0 {
		t.Fatal("case-insensitive filter failed")
	}
	r.setQuery("not-present")
	if len(r.visibleRows()) != 0 || len(r.config.Paths) != 2 || len(r.rows) != 3 {
		t.Fatal("filter changed actual apps/selections")
	}
	r.setQuery("")
	if len(r.visibleRows()) != 3 {
		t.Fatal("clear did not restore list")
	}
	n := nativeGlassPanel{resourcePage: true, resources: r}
	if n.controlAt(440, 192) != 25 {
		t.Fatal("empty search should focus field")
	}
	n.resources.setQuery("notes")
	if n.controlAt(440, 192) != 26 || n.controlEnabled(13) || n.controlEnabled(19) {
		t.Fatal("filtered hit targets/page bounds wrong")
	}
}
func TestResourceSearchEditingAndLength(t *testing.T) {
	r := nativeResources{}
	r.setQuery("café.exe")
	r.caret = 4
	r.insertSearch("_test")
	if r.query != "café_test.exe" || r.caret != 9 {
		t.Fatal(r.query, r.caret)
	}
	r.selectAll = true
	r.insertSearch("Notes\r\n.exe")
	if r.query != "Notes.exe" {
		t.Fatal("paste control characters were retained")
	}
	r.setQuery("café")
	n := nativeGlassPanel{resourcePage: true, focus: 25, resources: r}
	n.resourceSearchChar(8)
	if n.resources.query != "caf" {
		t.Fatal("backspace corrupted UTF-8")
	}
	n.resources.selectAll = true
	n.resourceSearchChar('x')
	if n.resources.query != "x" {
		t.Fatal("selection not replaced")
	}
	many := make([]rune, 150)
	for i := range many {
		many[i] = 'a'
	}
	r.setQuery(string(many))
	r.insertSearch("more")
	if len([]rune(r.query)) != 128 {
		t.Fatal("unbounded search input")
	}
}
