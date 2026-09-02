package transfer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	Protocol = "tavern-shelf-transfer"
	Version  = 1
)

var ErrNotFound = errors.New("transfer session not found")

type Source struct {
	Kind        string
	ID          string
	Name        string
	Subtype     string
	Filename    string
	Path        string
	Size        int64
	SHA256      string
	ContentType string
}

type Resolver func(ctx context.Context, kind, id string) (Source, error)

type Session struct {
	ID        string    `json:"id"`
	Protocol  string    `json:"protocol"`
	Version   int       `json:"version"`
	Kind      string    `json:"kind"`
	Subtype   string    `json:"subtype,omitempty"`
	Name      string    `json:"name"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
	URL       string    `json:"url"`
	Addresses []string  `json:"addresses"`
	ExpiresAt time.Time `json:"expiresAt"`

	source Source
}

type Manifest struct {
	Protocol  string    `json:"protocol"`
	Version   int       `json:"version"`
	Kind      string    `json:"kind"`
	Subtype   string    `json:"subtype,omitempty"`
	Name      string    `json:"name"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
	MediaType string    `json:"mediaType"`
	SourceURL string    `json:"sourceUrl"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Server struct {
	resolver Resolver
	ttl      time.Duration
	now      func() time.Time

	mu       sync.Mutex
	sessions map[string]Session
	listener net.Listener
	http     *http.Server
	port     int
}

type addressCandidate struct {
	host          string
	interfaceName string
	virtual       bool
	preferred     bool
}

func NewServer(resolver Resolver) *Server {
	return &Server{
		resolver: resolver,
		ttl:      10 * time.Minute,
		now:      time.Now,
		sessions: make(map[string]Session),
	}
}

func (s *Server) Create(ctx context.Context, kind, id string) (Session, error) {
	if s == nil || s.resolver == nil {
		return Session{}, errors.New("transfer service is unavailable")
	}
	source, err := s.resolver(ctx, strings.TrimSpace(kind), strings.TrimSpace(id))
	if err != nil {
		return Session{}, err
	}
	if source.ContentType == "" {
		source.ContentType = contentType(source.Filename)
	}
	if err := validateSource(&source); err != nil {
		return Session{}, err
	}
	if err := s.ensureStarted(); err != nil {
		return Session{}, err
	}
	token, err := randomToken()
	if err != nil {
		return Session{}, fmt.Errorf("create transfer token: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeExpiredLocked()
	addresses := localAddresses(s.port, token)
	if len(addresses) == 0 {
		return Session{}, errors.New("no usable local network address was found")
	}
	session := Session{
		ID: token, Protocol: Protocol, Version: Version,
		Kind: source.Kind, Subtype: source.Subtype, Name: source.Name,
		Filename: source.Filename, Size: source.Size, SHA256: source.SHA256,
		URL: addresses[0], Addresses: addresses, ExpiresAt: s.now().Add(s.ttl), source: source,
	}
	s.sessions[token] = session
	return session, nil
}

func (s *Server) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return ErrNotFound
	}
	delete(s.sessions, id)
	return nil
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	server := s.http
	s.http = nil
	s.listener = nil
	s.port = 0
	s.sessions = make(map[string]Session)
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("stop transfer server: %w", err)
	}
	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := strings.Trim(strings.TrimSpace(r.URL.Path), "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[0] != "v1" || parts[1] != "transfers" || len(parts) > 4 {
		http.NotFound(w, r)
		return
	}
	session, ok := s.session(parts[2])
	if !ok {
		writeError(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	if len(parts) == 3 {
		s.serveManifest(w, r, session)
		return
	}
	if parts[3] != "source" {
		http.NotFound(w, r)
		return
	}
	s.serveSource(w, r, session)
}

func (s *Server) ensureStarted() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		return fmt.Errorf("start local transfer server: %w", err)
	}
	server := &http.Server{
		Handler: s, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 30 * time.Second,
	}
	s.listener = listener
	s.http = server
	s.port = listener.Addr().(*net.TCPAddr).Port
	go func() { _ = server.Serve(listener) }()
	return nil
}

func (s *Server) session(id string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeExpiredLocked()
	session, ok := s.sessions[id]
	return session, ok
}

func (s *Server) removeExpiredLocked() {
	now := s.now()
	for id, session := range s.sessions {
		if !now.Before(session.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}

func (s *Server) serveManifest(w http.ResponseWriter, r *http.Request, session Session) {
	manifest := Manifest{
		Protocol: Protocol, Version: Version, Kind: session.Kind, Subtype: session.Subtype,
		Name: session.Name, Filename: session.Filename, Size: session.Size, SHA256: session.SHA256,
		MediaType: session.source.ContentType,
		SourceURL: transferSourceURL(session, r.Host),
		ExpiresAt: session.ExpiresAt,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(manifest)
}

func (s *Server) serveSource(w http.ResponseWriter, r *http.Request, session Session) {
	file, err := os.Open(session.source.Path)
	if err != nil {
		writeError(w, http.StatusGone, "shared source is no longer available")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusGone, "shared source is no longer available")
		return
	}
	if info.Size() != session.Size {
		writeError(w, http.StatusGone, "shared source changed after the transfer session was created")
		return
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), session.SHA256) {
		writeError(w, http.StatusGone, "shared source changed after the transfer session was created")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusGone, "shared source is no longer available")
		return
	}
	w.Header().Set("Content-Type", session.source.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFilename(session.Filename)))
	http.ServeContent(w, r, session.Filename, info.ModTime(), file)
}

func validateSource(source *Source) error {
	if source.Kind != "character" && source.Kind != "worldbook" && source.Kind != "preset" {
		return errors.New("unsupported transfer resource kind")
	}
	if source.ID == "" || source.Name == "" || source.Path == "" || source.Filename == "" || source.SHA256 == "" {
		return errors.New("transfer source metadata is incomplete")
	}
	info, err := os.Stat(source.Path)
	if err != nil {
		return fmt.Errorf("inspect transfer source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("transfer source is not a regular file")
	}
	source.Size = info.Size()
	file, err := os.Open(source.Path)
	if err != nil {
		return fmt.Errorf("open transfer source: %w", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("hash transfer source: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close transfer source: %w", closeErr)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), source.SHA256) {
		return errors.New("transfer source failed its SHA-256 integrity check")
	}
	return nil
}

func randomToken() (string, error) {
	buffer := make([]byte, 24)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func localAddresses(port int, token string) []string {
	preferred := preferredLocalIPv4()
	candidates := make([]addressCandidate, 0, 4)
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || !isPrivateIPv4(ip) {
				continue
			}
			candidates = append(candidates, addressCandidate{
				host: ip.String(), interfaceName: iface.Name,
				virtual: isLikelyVirtualInterface(iface), preferred: ip.String() == preferred,
			})
		}
	}
	candidates = rankAddressCandidates(candidates)
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, fmt.Sprintf("http://%s:%d/v1/transfers/%s", candidate.host, port, token))
	}
	return result
}

func rankAddressCandidates(candidates []addressCandidate) []addressCandidate {
	byHost := make(map[string]addressCandidate, len(candidates))
	for _, candidate := range candidates {
		existing, ok := byHost[candidate.host]
		if !ok || addressCandidateScore(candidate) < addressCandidateScore(existing) {
			byHost[candidate.host] = candidate
		}
	}
	result := make([]addressCandidate, 0, len(byHost))
	for _, candidate := range byHost {
		result = append(result, candidate)
	}
	sort.SliceStable(result, func(left, right int) bool {
		leftScore, rightScore := addressCandidateScore(result[left]), addressCandidateScore(result[right])
		if leftScore != rightScore {
			return leftScore < rightScore
		}
		return result[left].host < result[right].host
	})
	return result
}

func addressCandidateScore(candidate addressCandidate) int {
	score := addressRank(candidate.host) * 10
	if candidate.virtual {
		score += 100
	}
	if candidate.preferred {
		score -= 50
	}
	return score
}

func isPrivateIPv4(ip net.IP) bool {
	ip = ip.To4()
	return ip != nil && (ip[0] == 10 || ip[0] == 192 && ip[1] == 168 || ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31)
}

func isLikelyVirtualInterface(iface net.Interface) bool {
	if iface.Flags&net.FlagPointToPoint != 0 {
		return true
	}
	name := strings.ToLower(iface.Name)
	for _, marker := range []string{"vethernet", "hyper-v", "wsl", "docker", "vmware", "virtualbox", "tailscale", "zerotier", "wireguard", "vpn", "tunnel", "tap", "tun"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func preferredLocalIPv4() string {
	connection, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer connection.Close()
	address, ok := connection.LocalAddr().(*net.UDPAddr)
	if !ok || address.IP.To4() == nil || address.IP.IsLoopback() {
		return ""
	}
	return address.IP.String()
}

func addressRank(value string) int {
	ip := net.ParseIP(value).To4()
	if ip == nil {
		return 9
	}
	if ip[0] == 192 && ip[1] == 168 {
		return 0
	}
	if ip[0] == 10 {
		return 1
	}
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return 2
	}
	return 3
}

func transferSourceURL(session Session, requestHost string) string {
	host := ""
	for _, address := range session.Addresses {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Host == "" {
			continue
		}
		if host == "" {
			host = parsed.Host
		}
		if strings.EqualFold(parsed.Host, requestHost) {
			host = parsed.Host
			break
		}
	}
	return "http://" + host + "/v1/transfers/" + session.ID + "/source"
}

func contentType(filename string) string {
	value := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	if value == "" {
		return "application/octet-stream"
	}
	return value
}

func safeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune("\\/:*?\"<>|", r) {
			return '_'
		}
		return r
	}, name)
	if name == "" {
		return "tavern-shelf-resource"
	}
	return name
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
