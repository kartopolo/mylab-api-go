package observability

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu sync.Mutex

	requestsTotal map[requestKey]uint64
	durationCount map[requestKey]uint64
	durationSum   map[requestKey]time.Duration
}

type requestKey struct {
	Method string
	Path   string
	Status int
}

func NewMetrics() *Metrics {
	return &Metrics{
		requestsTotal: make(map[requestKey]uint64),
		durationCount: make(map[requestKey]uint64),
		durationSum:   make(map[requestKey]time.Duration),
	}
}

func (m *Metrics) Observe(method, path string, status int, dur time.Duration) {
	key := requestKey{Method: method, Path: path, Status: status}

	m.mu.Lock()
	m.requestsTotal[key]++
	m.durationCount[key]++
	m.durationSum[key] += dur
	m.mu.Unlock()
}

func (m *Metrics) RenderPrometheus() string {
	m.mu.Lock()
	keys := make([]requestKey, 0, len(m.requestsTotal))
	for k := range m.requestsTotal {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Path != keys[j].Path {
			return keys[i].Path < keys[j].Path
		}
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		return keys[i].Status < keys[j].Status
	})

	var b strings.Builder
	b.WriteString("# HELP http_requests_total Total number of HTTP requests.\n")
	b.WriteString("# TYPE http_requests_total counter\n")
	for _, k := range keys {
		b.WriteString(fmt.Sprintf(
			"http_requests_total{method=%q,path=%q,status=%q} %d\n",
			k.Method,
			k.Path,
			fmt.Sprintf("%d", k.Status),
			m.requestsTotal[k],
		))
	}

	b.WriteString("# HELP http_request_duration_seconds_sum Total sum of request durations in seconds.\n")
	b.WriteString("# TYPE http_request_duration_seconds_sum counter\n")
	for _, k := range keys {
		sum := m.durationSum[k]
		b.WriteString(fmt.Sprintf(
			"http_request_duration_seconds_sum{method=%q,path=%q,status=%q} %.6f\n",
			k.Method,
			k.Path,
			fmt.Sprintf("%d", k.Status),
			sum.Seconds(),
		))
	}

	b.WriteString("# HELP http_request_duration_seconds_count Total number of observed request durations.\n")
	b.WriteString("# TYPE http_request_duration_seconds_count counter\n")
	for _, k := range keys {
		b.WriteString(fmt.Sprintf(
			"http_request_duration_seconds_count{method=%q,path=%q,status=%q} %d\n",
			k.Method,
			k.Path,
			fmt.Sprintf("%d", k.Status),
			m.durationCount[k],
		))
	}

	m.mu.Unlock()
	return b.String()
}
