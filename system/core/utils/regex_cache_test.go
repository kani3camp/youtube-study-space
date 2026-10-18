package utils

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"sync"
	"testing"
)

func TestCachedRegex_ReusesCompiledRegexp(t *testing.T) {
	compiledRegexCache.Clear()

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
	if stored, ok := compiledRegexCache.Load("荒らし"); !ok || stored != first {
		t.Fatal("cachedRegex() did not store the compiled regexp")
	}
}

func TestContainsRegexWithIndex_UsesCachedRegexAndPreservesBehavior(t *testing.T) {
	compiledRegexCache.Clear()

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

	// Verify that ContainsRegexWithIndex itself populated the cache.
	// Calling cachedRegex before checking would allow a non-caching regression to pass.
	stored, ok := compiledRegexCache.Load("要確認")
	if !ok {
		t.Fatal("ContainsRegexWithIndex() did not cache the matching pattern")
	}
	compiled, ok := stored.(*regexp.Regexp)
	if !ok {
		t.Fatalf("cached value type = %T, want *regexp.Regexp", stored)
	}
	reused, err := cachedRegex("要確認")
	if err != nil {
		t.Fatalf("cachedRegex() error = %v", err)
	}
	if compiled != reused {
		t.Fatal("cachedRegex() did not reuse the regexp stored by ContainsRegexWithIndex")
	}
}

func TestContainsRegexWithIndex_PreservesFirstMatchAndShortCircuit(t *testing.T) {
	compiledRegexCache.Clear()

	// A matching first pattern must prevent compilation of the invalid later pattern.
	found, index, err := ContainsRegexWithIndex([]string{"荒らし", "["}, "これは荒らしです")
	if err != nil {
		t.Fatalf("ContainsRegexWithIndex() error = %v", err)
	}
	if !found || index != 0 {
		t.Fatalf("ContainsRegexWithIndex() = (%t, %d), want (true, 0)", found, index)
	}
	if _, cached := compiledRegexCache.Load("["); cached {
		t.Fatal("invalid pattern after the first match should not have been visited")
	}
}

func TestContainsRegexWithIndex_NoMatchOrPatterns(t *testing.T) {
	for _, tc := range []struct {
		name     string
		patterns []string
		message  string
	}{
		{name: "no match", patterns: []string{"荒らし", "要確認"}, message: "普通のコメント"},
		{name: "empty patterns", patterns: nil, message: "荒らし"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found, index, err := ContainsRegexWithIndex(tc.patterns, tc.message)
			if err != nil {
				t.Fatalf("ContainsRegexWithIndex() error = %v", err)
			}
			if found || index != 0 {
				t.Fatalf("ContainsRegexWithIndex() = (%t, %d), want (false, 0)", found, index)
			}
		})
	}
}

func TestContainsRegexWithIndex_InvalidRegexStillReturnsCompileError(t *testing.T) {
	compiledRegexCache.Clear()

	patterns := []string{"ok", "["}
	found, index, err := ContainsRegexWithIndex(patterns, "no match")
	if err == nil {
		t.Fatal("ContainsRegexWithIndex() error = nil, want compile error")
	}
	if found || index != 0 {
		t.Fatalf("ContainsRegexWithIndex() = (%t, %d), want (false, 0) on error", found, index)
	}

	_, rawErr := regexp.Compile(patterns[1])
	want := fmt.Sprintf("compile regex at index 1: %v", rawErr)
	if err.Error() != want {
		t.Fatalf("ContainsRegexWithIndex() error = %q, want %q", err, want)
	}
	var syntaxErr *syntax.Error
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("ContainsRegexWithIndex() error type = %T, want wrapped *syntax.Error", err)
	}
	if _, cached := compiledRegexCache.Load("["); cached {
		t.Fatal("invalid regex should not be cached")
	}
}

func TestCachedRegex_ConcurrentAccess(t *testing.T) {
	compiledRegexCache.Clear()

	const workers = 32
	type result struct {
		compiled *regexp.Regexp
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, workers)
	var wg sync.WaitGroup

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			compiled, err := cachedRegex("a+b")
			results <- result{compiled: compiled, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var first *regexp.Regexp
	for got := range results {
		if got.err != nil {
			t.Fatalf("cachedRegex() concurrent call error = %v", got.err)
		}
		if first == nil {
			first = got.compiled
		}
		if got.compiled != first || !got.compiled.MatchString("aaab") {
			t.Fatal("concurrent calls failed to share the same matching regexp")
		}
	}
	if first == nil {
		t.Fatal("expected a compiled regexp from concurrent calls")
	}
}
