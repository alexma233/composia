package agent

import "testing"

func TestPaginateDockerList(t *testing.T) {
	t.Parallel()

	items, total := paginateDockerList([]int{1, 2, 3, 4, 5}, 2, 2)
	if total != 5 || len(items) != 2 || items[0] != 3 || items[1] != 4 {
		t.Fatalf("page 2 = %+v total=%d", items, total)
	}
	items, total = paginateDockerList([]int{1, 2, 3}, 0, 2)
	if total != 3 || len(items) != 2 || items[0] != 1 || items[1] != 2 {
		t.Fatalf("page 0 normalized = %+v total=%d", items, total)
	}
	items, total = paginateDockerList([]int{1, 2, 3}, 3, 2)
	if total != 3 || len(items) != 0 {
		t.Fatalf("out of range page = %+v total=%d", items, total)
	}
}

func TestDockerSearchAndStringHelpers(t *testing.T) {
	t.Parallel()

	if !dockerSearchMatches("WEB", "api", "web-1") {
		t.Fatalf("expected case-insensitive search match")
	}
	if dockerSearchMatches("db", "api", "web-1") {
		t.Fatalf("unexpected search match")
	}
	if got := firstNonEmpty("", "  ", "value"); got != "value" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
	if got := joinStrings([]string{"a", "b"}); got != "a b" {
		t.Fatalf("joinStrings = %q", got)
	}
}

func TestDockerCompareHelpers(t *testing.T) {
	t.Parallel()

	if boolCompare(true, false) <= 0 || boolCompare(false, true) >= 0 || boolCompare(true, true) != 0 {
		t.Fatalf("unexpected boolCompare results")
	}
	if int64Compare(1, 2) != -1 || int64Compare(2, 1) != 1 || int64Compare(2, 2) != 0 {
		t.Fatalf("unexpected int64Compare results")
	}
	if uint32Compare(1, 2) != -1 || uint32Compare(2, 1) != 1 || uint32Compare(2, 2) != 0 {
		t.Fatalf("unexpected uint32Compare results")
	}
	if stringCompare("Alpha", "bravo") >= 0 {
		t.Fatalf("expected case-insensitive string comparison")
	}
	if !dockerSortResult(1, true) || dockerSortResult(1, false) || dockerSortResult(0, true) {
		t.Fatalf("unexpected dockerSortResult")
	}
}
