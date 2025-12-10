package relay

import (
	"fmt"
	"log/slog"
	"sync/atomic"
)

var (
	statsDVM    = "dvms"
	statsREQ    = "reqs"
	statsCOUNT  = "counts"
	statsCredit = "credits"
)

type stats struct {
	dvms     atomic.Uint32
	reqs     atomic.Uint32
	counts   atomic.Uint32
	credits  atomic.Uint32
	logEvery uint32
}

func (s *stats) Record(metricName string) {
	var counter *atomic.Uint32
	switch metricName {
	case statsDVM:
		counter = &s.dvms
	case statsREQ:
		counter = &s.reqs
	case statsCOUNT:
		counter = &s.counts
	case statsCredit:
		counter = &s.credits
	default:
		slog.Warn(fmt.Sprintf("Relay: Attempted to record unknown metric: %s", metricName))
		return
	}

	tot := counter.Add(1)
	if (tot % s.logEvery) == 0 {
		slog.Info("Relay metric log", "metric", metricName, "total", tot)
	}
}
