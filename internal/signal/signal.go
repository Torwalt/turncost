package signal

import (
	"time"

	"github.com/Torwalt/turncost/internal/pricing"
	"github.com/Torwalt/turncost/internal/session"
)

type TurnCost struct {
	Index       int
	Turn        session.Turn
	Cost        float64
	Cumulative  float64
	Context     int64
	HistoryCost float64
	Gap         time.Duration
	Cold        bool
	Unpriced    int
}

type Summary struct {
	Session  session.Session
	Turns    []TurnCost
	Cost     float64
	Baseline int64
	Peak     int64
	Requests int
	Usage    session.Usage
}

func (s Summary) HitRate() float64 {
	if p := s.Usage.Prompt(); p > 0 {
		return float64(s.Usage.CacheRead) / float64(p)
	}
	return 0
}

func Analyze(s session.Session) Summary {
	sum := Summary{Session: s}
	sum.Baseline = baseline(s)
	var last time.Time
	for i, t := range s.Turns {
		tc := TurnCost{Index: i, Turn: t}
		seen := false
		for _, r := range t.Requests {
			sum.Requests++
			sum.Usage = sum.Usage.Add(r.Usage)
			if r.Sidechain {
				c, ok := pricing.RequestCost(r)
				if !ok {
					tc.Unpriced++
				}
				tc.Cost += c
				continue
			}
			prompt := r.Usage.Prompt()
			if prompt > sum.Peak {
				sum.Peak = prompt
			}
			if !seen {
				seen = true
				tc.Context = prompt
				if !last.IsZero() {
					tc.Gap = r.Time.Sub(last)
					tc.Cold = tc.Gap > ttl(r.Usage)
				}
			}
			last = r.Time
			p, ok := pricing.Lookup(r.Model)
			if !ok {
				tc.Unpriced++
				continue
			}
			tc.Cost += p.Cost(r.Usage)
			if extra := prompt - sum.Baseline; extra > 0 {
				tc.HistoryCost += float64(extra) * p.CacheRead / 1e6
			}
		}
		sum.Cost += tc.Cost
		tc.Cumulative = sum.Cost
		sum.Turns = append(sum.Turns, tc)
	}
	return sum
}

func baseline(s session.Session) int64 {
	for _, t := range s.Turns {
		for _, r := range t.Requests {
			if !r.Sidechain {
				return r.Usage.Prompt()
			}
		}
	}
	return 0
}

func ttl(u session.Usage) time.Duration {
	if u.CacheWrite1h > 0 || u.CacheWrite5m == 0 {
		return time.Hour
	}
	return 5 * time.Minute
}
