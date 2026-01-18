package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mylab-api-go/internal/routes/auth"
	"mylab-api-go/internal/routes/shared"
)

type PluginConfig struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Mount           string            `json:"mount"`
	Upstream        string            `json:"upstream"`
	TimeoutMS       int               `json:"timeout_ms"`
	AuthMode        string            `json:"auth_mode"`         // forward_jwt | gateway_verified
	KeepMountPrefix bool              `json:"keep_mount_prefix"` // default false: strip mount prefix
	ForwardHeaders  []string          `json:"forward_headers"`
	InjectHeaders   map[string]string `json:"inject_headers"`
}

type PluginProxyController struct {
	dir string

	mu       sync.Mutex
	lastLoad time.Time
	plugins  []PluginConfig
	loadErr  error
}

func NewPluginProxyController() *PluginProxyController {
	dir := strings.TrimSpace(os.Getenv("PLUGIN_DIR"))
	return &PluginProxyController{dir: dir}
}

func (c *PluginProxyController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/plugins/") {
		shared.WriteError(w, http.StatusNotFound, "Not found.", nil)
		return
	}

	plugins, err := c.listPlugins(2 * time.Second)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "Plugin registry error.", map[string]string{"plugins": err.Error()})
		return
	}
	if len(plugins) == 0 {
		shared.WriteError(w, http.StatusNotFound, "No plugin configured.", nil)
		return
	}

	cfg, ok := matchPlugin(plugins, r.URL.Path)
	if !ok {
		shared.WriteError(w, http.StatusNotFound, "Plugin not found.", nil)
		return
	}

	target, err := url.Parse(cfg.Upstream)
	if err != nil || target.Scheme == "" || target.Host == "" {
		shared.WriteError(w, http.StatusInternalServerError, "Invalid plugin upstream.", map[string]string{"upstream": "invalid URL"})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		shared.WriteError(w, http.StatusBadGateway, "Upstream error.", map[string]string{"upstream": e.Error()})
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Preserve the incoming path; by default we strip the mount prefix.
		inPath := r.URL.Path
		if !cfg.KeepMountPrefix {
			if strings.HasPrefix(inPath, cfg.Mount) {
				inPath = strings.TrimPrefix(inPath, cfg.Mount)
				if inPath == "" {
					inPath = "/"
				}
			}
		}

		req.URL.Path = singleJoiningSlash(target.Path, inPath)
		req.URL.RawPath = req.URL.EscapedPath()

		// Forward request id.
		if rid := shared.RequestIDFromContext(r.Context()); rid != "" {
			req.Header.Set("X-Request-Id", rid)
		}

		// Auth handling.
		switch strings.ToLower(strings.TrimSpace(cfg.AuthMode)) {
		case "gateway_verified":
			if info, ok := auth.AuthInfoFromContext(r.Context()); ok {
				if info.UserID > 0 {
					req.Header.Set("X-User-Id", strconv.FormatInt(info.UserID, 10))
				}
				if info.CompanyID > 0 {
					req.Header.Set("X-Company-Id", strconv.FormatInt(info.CompanyID, 10))
				}
				if info.Role != "" {
					req.Header.Set("X-Role", info.Role)
				}
			}
		case "forward_jwt", "":
			// default: forward Authorization (already in req.Header)
		default:
			// unknown mode: keep safe behavior (forward JWT)
		}

		// Optional explicit forward headers from original request.
		for _, h := range cfg.ForwardHeaders {
			h = http.CanonicalHeaderKey(strings.TrimSpace(h))
			if h == "" {
				continue
			}
			if v := r.Header.Get(h); v != "" {
				req.Header.Set(h, v)
			}
		}

		// Inject fixed headers.
		for k, v := range cfg.InjectHeaders {
			k = http.CanonicalHeaderKey(strings.TrimSpace(k))
			if k == "" {
				continue
			}
			req.Header.Set(k, v)
		}
	}

	// Enforce per-plugin timeout by wrapping request context.
	if cfg.TimeoutMS > 0 {
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(cfg.TimeoutMS)*time.Millisecond)
		defer cancel()
		r = r.WithContext(ctx)
	}

	proxy.ServeHTTP(w, r)
}

func (c *PluginProxyController) listPlugins(ttl time.Duration) ([]PluginConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dir == "" {
		return nil, nil
	}

	now := time.Now()
	if now.Sub(c.lastLoad) < ttl && c.plugins != nil {
		return c.plugins, c.loadErr
	}

	plugins, err := loadPluginConfigs(c.dir)
	c.plugins = plugins
	c.loadErr = err
	c.lastLoad = now
	return plugins, err
}

func loadPluginConfigs(dir string) ([]PluginConfig, error) {
	var files []string
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if len(files) == 0 {
		return nil, nil
	}

	var out []PluginConfig
	for _, p := range files {
		cfg, err := readPluginConfigJSON(p)
		if err != nil {
			return nil, fmt.Errorf("read plugin config %s: %w", p, err)
		}
		cfg.Mount = strings.TrimSpace(cfg.Mount)
		cfg.Upstream = strings.TrimSpace(cfg.Upstream)
		if cfg.Mount == "" || !strings.HasPrefix(cfg.Mount, "/") {
			return nil, fmt.Errorf("invalid mount in %s", p)
		}
		if !strings.HasPrefix(cfg.Mount, "/v1/plugins/") {
			return nil, fmt.Errorf("mount must start with /v1/plugins/ in %s", p)
		}
		if cfg.Upstream == "" {
			return nil, fmt.Errorf("missing upstream in %s", p)
		}
		out = append(out, cfg)
	}

	// Longest mount wins.
	sort.Slice(out, func(i, j int) bool {
		return len(out[i].Mount) > len(out[j].Mount)
	})

	return out, nil
}

func matchPlugin(plugins []PluginConfig, path string) (PluginConfig, bool) {
	for _, p := range plugins {
		if strings.HasPrefix(path, p.Mount) {
			return p, true
		}
	}
	return PluginConfig{}, false
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func readPluginConfigJSON(path string) (PluginConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return PluginConfig{}, err
	}
	defer f.Close()

	b, err := io.ReadAll(io.LimitReader(f, 256*1024))
	if err != nil {
		return PluginConfig{}, err
	}

	var cfg PluginConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return PluginConfig{}, err
	}
	return cfg, nil
}
