package plugins

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PluginHealth struct {
	Name       string          `json:"name"`
	Mount      string          `json:"mount"`
	Upstream   string          `json:"upstream"`
	OK         bool            `json:"ok"`
	Status     int             `json:"status"`
	DurationMS int64           `json:"duration_ms"`
	Error      string          `json:"error,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
}

type GatewayHealth struct {
	OK      bool           `json:"ok"`
	Message string         `json:"message"`
	Plugins []PluginHealth `json:"plugins,omitempty"`
}

func (c *PluginProxyController) AggregatePluginsHealth(ctx context.Context) (GatewayHealth, int) {
	plugins, err := c.listPlugins(2 * time.Second)
	if err != nil {
		return GatewayHealth{OK: false, Message: "Plugin registry error."}, http.StatusServiceUnavailable
	}
	if len(plugins) == 0 {
		return GatewayHealth{OK: true, Message: "ok"}, http.StatusOK
	}

	report := GatewayHealth{OK: true, Message: "ok"}
	client := &http.Client{}

	anyFail := false
	for _, p := range plugins {
		ph := PluginHealth{Name: p.Name, Mount: p.Mount, Upstream: p.Upstream}
		start := time.Now()

		target, perr := url.Parse(strings.TrimSpace(p.Upstream))
		if perr != nil || target.Scheme == "" || target.Host == "" {
			ph.OK = false
			ph.Status = 0
			ph.Error = "invalid upstream"
			anyFail = true
			report.Plugins = append(report.Plugins, ph)
			continue
		}

		hURL := *target
		hURL.Path = singleJoiningSlash(target.Path, "/healthz")

		timeout := 2 * time.Second
		if p.TimeoutMS > 0 {
			timeout = time.Duration(p.TimeoutMS) * time.Millisecond
		}
		pctx, cancel := context.WithTimeout(ctx, timeout)
		req, _ := http.NewRequestWithContext(pctx, http.MethodGet, hURL.String(), nil)
		resp, reqErr := client.Do(req)
		cancel()

		ph.DurationMS = time.Since(start).Milliseconds()
		if reqErr != nil {
			ph.OK = false
			ph.Error = reqErr.Error()
			anyFail = true
			report.Plugins = append(report.Plugins, ph)
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()

		ph.Status = resp.StatusCode
		ph.OK = resp.StatusCode >= 200 && resp.StatusCode < 300
		// Keep raw JSON if possible, else string.
		if json.Valid(b) {
			ph.Body = json.RawMessage(b)
			// If payload contains {"ok": false}, treat as unhealthy.
			var probe struct {
				OK *bool `json:"ok"`
			}
			if err := json.Unmarshal(b, &probe); err == nil && probe.OK != nil {
				ph.OK = ph.OK && *probe.OK
			}
		} else if len(b) > 0 {
			ph.Body = json.RawMessage([]byte("\"" + escapeJSONString(string(b)) + "\""))
		}
		if !ph.OK {
			anyFail = true
		}

		report.Plugins = append(report.Plugins, ph)
	}

	if anyFail {
		report.OK = false
		report.Message = "Some plugins are unhealthy."
		return report, http.StatusServiceUnavailable
	}
	return report, http.StatusOK
}

func escapeJSONString(s string) string {
	// minimal escape for embedding arbitrary text into JSON string
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
