package pricing

import (
	"regexp"
	"strings"

	"github.com/Torwalt/turncost/internal/session"
)

type Price struct {
	Input        float64
	Output       float64
	CacheRead    float64
	CacheWrite5m float64
	CacheWrite1h float64
}

var Table = map[string]Price{
	"opus-5-5":   {Input: 4, Output: 20, CacheRead: 0.20, CacheWrite5m: 5, CacheWrite1h: 8},
	"opus-5":     {Input: 5, Output: 25, CacheRead: 0.50, CacheWrite5m: 6.25, CacheWrite1h: 10},
	"sonnet-5-5": {Input: 2, Output: 10, CacheRead: 0.20, CacheWrite5m: 2.50, CacheWrite1h: 4},
	"sonnet-5":   {Input: 2, Output: 10, CacheRead: 0.20, CacheWrite5m: 2.50, CacheWrite1h: 4},
	"fable-5-1":  {Input: 10, Output: 50, CacheRead: 0.25, CacheWrite5m: 12.50, CacheWrite1h: 20},
	"fable-5":    {Input: 10, Output: 50, CacheRead: 1, CacheWrite5m: 12.50, CacheWrite1h: 20},
	"haiku-4-5":  {Input: 1, Output: 5, CacheRead: 0.10, CacheWrite5m: 1.25, CacheWrite1h: 2},
}

var dateSuffix = regexp.MustCompile(`-\d{8}$`)

func Normalize(model string) string {
	m := strings.TrimPrefix(model, "claude-")
	return dateSuffix.ReplaceAllString(m, "")
}

func Lookup(model string) (Price, bool) {
	p, ok := Table[Normalize(model)]
	return p, ok
}

func (p Price) Cost(u session.Usage) float64 {
	return (float64(u.Input)*p.Input +
		float64(u.CacheRead)*p.CacheRead +
		float64(u.CacheWrite5m)*p.CacheWrite5m +
		float64(u.CacheWrite1h)*p.CacheWrite1h +
		float64(u.Output)*p.Output) / 1e6
}

func RequestCost(r session.Request) (float64, bool) {
	p, ok := Lookup(r.Model)
	if !ok {
		return 0, false
	}
	return p.Cost(r.Usage), true
}
