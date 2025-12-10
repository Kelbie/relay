package api

import (
	"fmt"
	"log/slog"
	"sync/atomic"
)

var (
	statsDVM    = "dvms"
	statsCredit = "credits"
)

type stats struct {
	dvms     atomic.Uint32
	credits  atomic.Uint32
	logEvery uint32
}

func (s *stats) Record(metricName string) {
	var counter *atomic.Uint32

	switch metricName {
	case statsDVM:
		counter = &s.dvms
	case statsCredit:
		counter = &s.credits
	default:
		slog.Warn(fmt.Sprintf("API: Attempted to record unknown metric: %s", metricName))
		return
	}

	tot := counter.Add(1)
	if (tot % s.logEvery) == 0 {
		slog.Info("API record", "metric", metricName, "total", tot)
	}
}
