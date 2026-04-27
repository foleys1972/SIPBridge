package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	"sipbridge/internal/config"
)

type principal struct {
	Username string      `json:"username"`
	Provider string      `json:"provider"`
	Role     config.Role `json:"role"`
}

type sessionRecord struct {
	Principal principal
	ExpiresAt time.Time
}

type authService struct {
	mu       sync.Mutex
	sessions map[string]sessionRecord
}

func newAuthService() *authService {
	return &authService{sessions: make(map[string]sessionRecord)}
}

func (a *authService) issue(p principal, ttl time.Duration) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	a.mu.Lock()
	a.sessions[tok] = sessionRecord{Principal: p, ExpiresAt: time.Now().Add(ttl)}
	a.mu.Unlock()
	return tok, nil
}

func (a *authService) verify(tok string) (principal, bool) {
	if strings.TrimSpace(tok) == "" {
		return principal{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	rec, ok := a.sessions[tok]
	if !ok {
		return principal{}, false
	}
	if time.Now().After(rec.ExpiresAt) {
		delete(a.sessions, tok)
		return principal{}, false
	}
	return rec.Principal, true
}

func bearerToken(h string) string {
	v := strings.TrimSpace(h)
	if !strings.HasPrefix(strings.ToLower(v), "bearer ") {
		return ""
	}
	return strings.TrimSpace(v[7:])
}

func roleAllowed(role config.Role, allow []config.Role) bool {
	for _, r := range allow {
		if role == r {
			return true
		}
	}
	return false
}

func (s *Server) currentAuthSpec() *config.AuthSpec {
	if s == nil || s.cm == nil {
		return nil
	}
	return s.cm.Current().Spec.Auth
}

func authTTL(spec *config.AuthSpec) time.Duration {
	mins := 480
	if spec != nil && spec.SessionTTLMinutes > 0 {
		mins = spec.SessionTTLMinutes
	}
	return time.Duration(mins) * time.Minute
}

func chooseHigherRole(a, b config.Role) config.Role {
	rank := func(r config.Role) int {
		switch r {
		case config.RoleAdmin:
			return 3
		case config.RoleOperator:
			return 2
		default:
			return 1
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}

func validateLocal(spec *config.LocalAuthSpec, username, password string) (principal, bool) {
	if spec == nil || !spec.Enabled {
		return principal{}, false
	}
	for _, u := range spec.Users {
		if subtle.ConstantTimeCompare([]byte(u.Username), []byte(username)) != 1 {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(u.Password), []byte(password)) != 1 {
			return principal{}, false
		}
		role := u.Role
		if role == "" {
			role = config.RoleReadonly
		}
		return principal{Username: u.Username, Provider: "local", Role: role}, true
	}
	return principal{}, false
}

func validateADLDS(spec *config.ADLDSSpec, username, password string) (principal, bool, error) {
	if spec == nil || !spec.Enabled {
		return principal{}, false, nil
	}
	if strings.TrimSpace(spec.URL) == "" || strings.TrimSpace(spec.BaseDN) == "" {
		return principal{}, false, errors.New("adlds config incomplete")
	}
	conn, err := ldap.DialURL(spec.URL)
	if err != nil {
		return principal{}, false, err
	}
	defer conn.Close()

	bindDN := strings.TrimSpace(spec.BindDN)
	if bindDN != "" {
		pwd := ""
		if env := strings.TrimSpace(spec.BindPasswordEnvVar); env != "" {
			pwd = os.Getenv(env)
		}
		if err := conn.Bind(bindDN, pwd); err != nil {
			return principal{}, false, err
		}
	}

	filterTpl := strings.TrimSpace(spec.UserFilter)
	if filterTpl == "" {
		filterTpl = "(sAMAccountName=%s)"
	}
	filter := strings.ReplaceAll(filterTpl, "%s", ldap.EscapeFilter(username))
	req := ldap.NewSearchRequest(
		spec.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 5, false,
		filter,
		[]string{"dn", "memberOf"},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return principal{}, false, err
	}
	if len(res.Entries) != 1 {
		return principal{}, false, nil
	}
	userDN := res.Entries[0].DN
	if err := conn.Bind(userDN, password); err != nil {
		return principal{}, false, nil
	}

	role := config.RoleReadonly
	memberOf := res.Entries[0].GetAttributeValues("memberOf")
	for _, g := range memberOf {
		for mapDN, mapRole := range spec.GroupRoleMap {
			if strings.EqualFold(strings.TrimSpace(mapDN), strings.TrimSpace(g)) {
				role = chooseHigherRole(role, mapRole)
			}
		}
	}
	return principal{Username: username, Provider: "adlds", Role: role}, true, nil
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/v1/auth/") {
			next.ServeHTTP(w, r)
			return
		}
		spec := s.currentAuthSpec()
		if spec == nil || !spec.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		token := bearerToken(r.Header.Get("Authorization"))
		p, ok := s.authn.verify(token)
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "unauthorized"})
			return
		}

		allow := []config.Role{config.RoleReadonly, config.RoleOperator, config.RoleAdmin}
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/settings/"), strings.HasPrefix(r.URL.Path, "/v1/config"):
			allow = []config.Role{config.RoleAdmin}
		case strings.HasPrefix(r.URL.Path, "/v1/mi/"), strings.HasPrefix(r.URL.Path, "/v1/bridges"), strings.HasPrefix(r.URL.Path, "/v1/conference-groups/"):
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
				allow = []config.Role{config.RoleAdmin, config.RoleOperator}
			}
		case strings.HasPrefix(r.URL.Path, "/v1/iptv/"):
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
				allow = []config.Role{config.RoleAdmin, config.RoleOperator}
			}
		}
		if !roleAllowed(p.Role, allow) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "forbidden"})
			return
		}
		r.Header.Set("X-Auth-User", p.Username)
		r.Header.Set("X-Auth-Role", string(p.Role))
		r.Header.Set("X-Auth-Provider", p.Provider)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(req.Username)
	password := req.Password
	spec := s.currentAuthSpec()
	if spec == nil || !spec.Enabled {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if p, ok := validateLocal(spec.Local, username, password); ok {
		tok, err := s.authn.issue(p, authTTL(spec))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "token": tok, "user": p})
		return
	}
	p, ok, err := validateADLDS(spec.ADLDS, username, password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if ok {
		tok, err := s.authn.issue(p, authTTL(spec))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "token": tok, "user": p})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid credentials"})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	spec := s.currentAuthSpec()
	if spec == nil || !spec.Enabled {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false, "authenticated": false})
		return
	}
	token := bearerToken(r.Header.Get("Authorization"))
	p, ok := s.authn.verify(token)
	if !ok {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true, "authenticated": false})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true, "authenticated": true, "user": p})
}
