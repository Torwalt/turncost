package pricing

import (
	"math"
	"testing"

	"github.com/Torwalt/turncost/internal/session"
)

func TestNormalizeStripsPrefixAndDate(t *testing.T) {
	for in, want := range map[string]string{
		"claude-opus-5-5":           "opus-5-5",
		"claude-haiku-4-5-20251001": "haiku-4-5",
		"sonnet-5-5":                "sonnet-5-5",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCostPricesEachTokenClass(t *testing.T) {
	p, ok := Lookup("claude-opus-5-5")
	if !ok {
		t.Fatal("opus-5-5 missing from the table")
	}
	u := session.Usage{Input: 1e6, CacheRead: 1e6, CacheWrite5m: 1e6, CacheWrite1h: 1e6, Output: 1e6}
	if got, want := p.Cost(u), 4+0.2+5+8+20.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("Cost = %v, want %v", got, want)
	}
}

func TestUnknownModelIsReported(t *testing.T) {
	if _, ok := RequestCost(session.Request{Model: "<synthetic>"}); ok {
		t.Error("an unknown model must not be priced")
	}
}
