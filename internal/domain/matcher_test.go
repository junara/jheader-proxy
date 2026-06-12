package domain

import "testing"

func TestMatcherIsTarget(t *testing.T) {
	m := NewMatcher([]string{"example.test"})

	targets := []string{
		"example.test",
		"app.example.test",
		"api.example.test",
		"foo.bar.example.test",
		"EXAMPLE.TEST",
		"app.example.test:443",
		" example.test ",
	}
	for _, host := range targets {
		if !m.IsTarget(host) {
			t.Errorf("IsTarget(%q) = false, want true", host)
		}
	}

	nonTargets := []string{
		"evilexample.test",
		"example.test.evil.com",
		"example.com",
		"",
	}
	for _, host := range nonTargets {
		if m.IsTarget(host) {
			t.Errorf("IsTarget(%q) = true, want false", host)
		}
	}
}

func TestMatcherMultipleDomains(t *testing.T) {
	m := NewMatcher([]string{"example.test", "example.dev"})

	if !m.IsTarget("app.example.dev") {
		t.Error(`IsTarget("app.example.dev") = false, want true`)
	}
	if m.IsTarget("example.org") {
		t.Error(`IsTarget("example.org") = true, want false`)
	}
}
