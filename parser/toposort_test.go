package parser

import (
	"strconv"
	"testing"
)

func TestTopoSortEmpty(t *testing.T) {
	sorted := topoSort(nil)
	if len(sorted) != 0 {
		t.Fatalf("expected empty slice, got %v", sorted)
	}
}

func TestTopoSortSimple(t *testing.T) {
	sorted := topoSort(map[string]map[string]struct{}{
		"a": {"b": struct{}{}},
		"b": {"c": struct{}{}},
	})
	if len(sorted) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sorted))
	}
	assertBefore(t, sorted, "c", "b")
	assertBefore(t, sorted, "b", "a")
}

func TestTopoSortComplex(t *testing.T) {
	sorted := topoSort(map[string]map[string]struct{}{
		"a": {"b": struct{}{}, "c": struct{}{}},
		"c": {"d": struct{}{}},
		"f": {"g": struct{}{}, "h": struct{}{}},
		"g": {},
		"h": {},
	})
	if len(sorted) != 7 {
		t.Fatalf("expected 7 items, got %d", len(sorted))
	}
	assertBefore(t, sorted, "g", "f")
	assertBefore(t, sorted, "h", "f")
	assertBefore(t, sorted, "d", "c")
	assertBefore(t, sorted, "c", "a")
	assertBefore(t, sorted, "b", "a")
}

func assertBefore(t *testing.T, sorted []string, x, y string) {
	xi := strIndex(sorted, x)
	if xi < 0 {
		t.Fatalf("expected %q to be in result", x)
	}
	yi := strIndex(sorted, y)
	if yi < 0 {
		t.Fatalf("expected %q to be in result", y)
	}
	if xi >= yi {
		t.Fatalf("expected %q to come before %q, got indexes %d and %d", x, y, xi, yi)
	}
}

func strIndex(slice []string, s string) int {
	for i, item := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func TestTopoSortCycle(t *testing.T) {
	sorted := topoSort(map[string]map[string]struct{}{
		"a": {"b": struct{}{}, "c": struct{}{}},
		"c": {"a": struct{}{}},
	})
	if len(sorted) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sorted))
	}
	assertBefore(t, sorted, "b", "a")
	c := strIndex(sorted, "a")
	if c < 0 {
		t.Fatalf("expected %q to be in result", c)
	}
}

func TestTopoSortLarge(t *testing.T) {
	const num = 1000
	graph := make(map[string]map[string]struct{})
	for i := 0; i < num; i++ {
		graph[strconv.Itoa(i)] = map[string]struct{}{strconv.Itoa(i + 1): {}}
	}
	graph[strconv.Itoa(num)] = map[string]struct{}{}
	sorted := topoSort(graph)
	if len(sorted) != num+1 {
		t.Fatalf("expected %d items, got %d", num+1, len(sorted))
	}
	for i := 0; i < num+1; i++ {
		expected := num - i
		if sorted[i] != strconv.Itoa(expected) {
			t.Fatalf("expected %d to be at index %d, got %s", num-1, i, sorted[i])
		}
	}
}
