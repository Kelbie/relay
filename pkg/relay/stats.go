package relay

import (
	"fmt"
	"log/slog"
	"sync/atomic"
)

const (
	statsDVM    = "dvms"
	statsREQ    = "reqs"
	statsSearch = "search"
	statsCredit = "credits"
	statsCOUNT  = "counts"

	logDVM    uint32 = 1000
	logREQ    uint32 = 100_000
	logSearch uint32 = 1000
	logCredit uint32 = 1000
	logCOUNT  uint32 = 1000
)

type stats struct {
	dvms    atomic.Uint32
	reqs    atomic.Uint32
	search  atomic.Uint32
	credits atomic.Uint32
	counts  atomic.Uint32
}

func (s *stats) Record(metricName string) {
	var counter *atomic.Uint32
	var logEvery uint32

	switch metricName {
	case statsDVM:
		counter = &s.dvms
		logEvery = logDVM

	case statsREQ:
		counter = &s.reqs
		logEvery = logREQ

	case statsSearch:
		counter = &s.search
		logEvery = logSearch

	case statsCredit:
		counter = &s.credits
		logEvery = logCredit

	case statsCOUNT:
		counter = &s.counts
		logEvery = logCOUNT

	default:
		slog.Warn(fmt.Sprintf("Relay: Attempted to record unknown metric: %s", metricName))
		return
	}

	tot := counter.Add(1)
	if (tot % logEvery) == 0 {
		slog.Info("Relay record", "metric", metricName, "total", tot)
	}
}
