package config

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

type Manager struct {
	path    string
	schema  *jsonschema.Schema
	cur     atomic.Value // RootConfig

	httpURL        string
	httpBearer     string
	httpTLSInsecure bool
	pollSeconds    int
	httpMu         sync.Mutex
	lastHTTPHash   string
}

func NewManager(cfg Config) (*Manager, error) {
	schemaBytes, err := os.ReadFile("internal/config/schema_v1alpha1.json")
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", bytes.NewReader(schemaBytes)); err != nil {
		return nil, fmt.Errorf("schema resource: %w", err)
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	m := &Manager{
		path:            cfg.ConfigPath,
		schema:          schema,
		httpURL:         cfg.ConfigHTTPURL,
		httpBearer:      cfg.ConfigHTTPBearerToken,
		httpTLSInsecure: cfg.ConfigHTTPTLSInsecure,
		pollSeconds:     cfg.ConfigHTTPPollSeconds,
	}
	return m, nil
}

// ConfigPath returns the local path used when not loading from CONFIG_HTTP_URL.
func (m *Manager) ConfigPath() string { return m.path }

// ConfigHTTPURL returns the HTTP config source URL, if configured.
func (m *Manager) ConfigHTTPURL() string { return m.httpURL }

// ConfigReadOnly is true when CONFIG_HTTP_URL is set (API cannot persist YAML to disk).
func (m *Manager) ConfigReadOnly() bool { return m.httpURL != "" }

// ConfigHTTPPollSeconds returns CONFIG_HTTP_POLL_SECONDS when using HTTP config.
func (m *Manager) ConfigHTTPPollSeconds() int { return m.pollSeconds }

// StartHTTPPoll periodically refetches CONFIG_HTTP_URL and applies changes in memory.
func (m *Manager) StartHTTPPoll(ctx context.Context) {
	if m.httpURL == "" || m.pollSeconds <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(time.Duration(m.pollSeconds) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if _, err := m.loadFromHTTP("poll"); err != nil {
					log.Printf("config poll: %v", err)
				}
			}
		}
	}()
}

func (m *Manager) Current() RootConfig {
	v := m.cur.Load()
	if v == nil {
		return RootConfig{}
	}
	return v.(RootConfig)
}

func (m *Manager) LoadFromFile() (RootConfig, error) {
	if m.httpURL != "" {
		return m.loadFromHTTP("startup")
	}
	root, err := LoadAppConfig(m.path)
	if err != nil {
		return RootConfig{}, err
	}
	if err := m.ValidateRoot(root); err != nil {
		return RootConfig{}, err
	}
	m.cur.Store(root)
	return root, nil
}

func (m *Manager) httpClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if m.httpTLSInsecure {
		tr.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // CONFIG_HTTP_TLS_INSECURE for internal endpoints only
		}
	}
	return &http.Client{Timeout: 45 * time.Second, Transport: tr}
}

func (m *Manager) loadFromHTTP(reason string) (RootConfig, error) {
	req, err := http.NewRequest(http.MethodGet, m.httpURL, nil)
	if err != nil {
		return RootConfig{}, err
	}
	req.Header.Set("User-Agent", "sipbridge-config-loader/1")
	if m.httpBearer != "" {
		req.Header.Set("Authorization", "Bearer "+m.httpBearer)
	}
	resp, err := m.httpClient().Do(req)
	if err != nil {
		return RootConfig{}, fmt.Errorf("GET %s: %w", m.httpURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RootConfig{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return RootConfig{}, fmt.Errorf("GET %s: HTTP %d", m.httpURL, resp.StatusCode)
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])

	m.httpMu.Lock()
	if hash == m.lastHTTPHash && reason == "poll" {
		m.httpMu.Unlock()
		return m.Current(), nil
	}
	m.httpMu.Unlock()

	root, err := parseRootYAML(body)
	if err != nil {
		return RootConfig{}, err
	}
	if err := m.ValidateRoot(root); err != nil {
		return RootConfig{}, err
	}

	m.httpMu.Lock()
	m.lastHTTPHash = hash
	m.httpMu.Unlock()
	m.cur.Store(root)
	if reason != "" {
		log.Printf("config loaded from CONFIG_HTTP_URL (%s), bytes=%d", reason, len(body))
	}
	return root, nil
}

func parseRootYAML(b []byte) (RootConfig, error) {
	var root RootConfig
	if err := yaml.Unmarshal(b, &root); err != nil {
		return RootConfig{}, fmt.Errorf("parse yaml: %w", err)
	}
	if root.APIVersion == "" || root.Kind == "" {
		return RootConfig{}, fmt.Errorf("CONFIG_HTTP_URL must return versioned YAML (apiVersion/kind/spec)")
	}
	ApplyVersionedRootDefaults(&root)
	return root, nil
}

// ApplyYAML validates a versioned config document, persists it to disk, and activates it.
func (m *Manager) ApplyYAML(yamlBytes []byte) (RootConfig, error) {
	if m.httpURL != "" {
		return RootConfig{}, fmt.Errorf("config is read-only: CONFIG_HTTP_URL is set; publish changes to that URL or unset CONFIG_HTTP_URL to allow writes to %s", m.path)
	}
	var root RootConfig
	if err := yaml.Unmarshal(yamlBytes, &root); err != nil {
		return RootConfig{}, fmt.Errorf("parse yaml: %w", err)
	}
	if root.APIVersion == "" || root.Kind == "" {
		return RootConfig{}, fmt.Errorf("missing apiVersion/kind; expected sipbridge.io/v1alpha1 SIPBridgeConfig")
	}
	ApplyVersionedRootDefaults(&root)
	if err := m.ValidateRoot(root); err != nil {
		return RootConfig{}, err
	}
	if err := writeFileAtomic(m.path, yamlBytes, 0o644); err != nil {
		return RootConfig{}, err
	}
	m.cur.Store(root)
	return root, nil
}

func (m *Manager) ValidateYAML(yamlBytes []byte) error {
	var root RootConfig
	if err := yaml.Unmarshal(yamlBytes, &root); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	// For legacy files without apiVersion/kind/spec, LoadAppConfig handles conversion; for validate endpoint we require versioned.
	if root.APIVersion == "" || root.Kind == "" {
		return fmt.Errorf("missing apiVersion/kind; expected sipbridge.io/v1alpha1 SIPBridgeConfig")
	}
	ApplyVersionedRootDefaults(&root)
	return m.ValidateRoot(root)
}

func (m *Manager) ValidateRoot(root RootConfig) error {
	jb, err := json.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	var v any
	if err := json.Unmarshal(jb, &v); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	if err := m.schema.Validate(v); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	if err := ValidateManagedServers(root.Spec.Servers); err != nil {
		return err
	}
	if err := ValidateClusterSpec(root.Spec.Cluster); err != nil {
		return err
	}
	if err := ValidateDatabaseSpec(root.Spec.Database); err != nil {
		return err
	}
	if err := ValidateRecordingSpec(root.Spec.Recording); err != nil {
		return err
	}
	for _, u := range root.Spec.Users {
		if err := ValidateUserDeviceList(u.Devices); err != nil {
			return fmt.Errorf("user %q: %w", u.ID, err)
		}
	}
	if err := ValidateRecordingLinks(root); err != nil {
		return err
	}
	return nil
}

func (m *Manager) SchemaBytes() ([]byte, error) {
	return os.ReadFile("internal/config/schema_v1alpha1.json")
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp := filepath.Join(dir, "."+base+".tmp")

	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	// Windows doesn't allow rename over existing; remove first.
	_ = os.Remove(path)
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
