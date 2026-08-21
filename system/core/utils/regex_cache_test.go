package utils

import (
	"sync"
	"testing"
)

func TestCachedRegex_ReusesCompiledRegexp(t *testing.T) {
	compiledRegexCache = sync.Map{}

	first, err := cachedRegex("荒らし")
	if err != nil {
		t.Fatalf("cachedRegex() first call error = %v", err)
	}
	second, err := cachedRegex("荒らし")
	if err != nil {
		t.Fatalf("cachedRegex() second call error = %v", err)
	}

	if first != second {
		t.Fatal("cachedRegex() returned different regexp pointers for the same pattern")
	}
}

func TestContainsRegexWithIndex_UsesCachedRegexAndPreservesBehavior(t *testing.T) {
	compiledRegexCache = sync.Map{}

	patterns := []string{"荒らし", "要確認"}

	found, index, err := ContainsRegexWithIndex(patterns, "これは要確認です")
	if err != nil {
		t.Fatalf("ContainsRegexWithIndex() error = %v", err)
	}
	if !found {
		t.Fatal("ContainsRegexWithIndex() found = false, want true")
	}
	if index != 1 {
		t.Fatalf("ContainsRegexWithIndex() index = %d, want 1", index)
	}

	first, err := cachedRegex("要確認")
	if err != nil {
		t.Fatalf("cachedRegex() error = %v", err)
	}
	second, err := cachedRegex("要確認")
	if err != nil {
		t.Fatalf("cachedRegex() second call error = %v", err)
	}
	if first != second {
		t.Fatal("expected cached regexp to be reused")
	}
}

func TestContainsRegexWithIndex_InvalidRegexStillReturnsCompileError(t *testing.T) {
	compiledRegexCache = sync.Map{}

	found, index, err := ContainsRegexWithIndex([]string{"ok", "["}, "no match")
	if err == nil {
		t.Fatal("ContainsRegexWithIndex() error = nil, want compile error")
	}
	if found {
		t.Fatal("ContainsRegexWithIndex() found = true, want false")
	}
	if index != 0 {
		t.Fatalf("ContainsRegexWithIndex() index = %d, want 0 on error", index)
	}
}
