package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zvensmoluya/tavern-shelf/internal/card"
	"github.com/zvensmoluya/tavern-shelf/internal/importer"
)

// ImportURL downloads source bytes without decoding or re-encoding images.
// Site URLs are resolved here; the normal importer owns identification and storage.
func (a *App) ImportURL(ctx context.Context, input string) (importer.Result, error) {
	client := newImportClient()
	defer client.CloseIdleConnections()
	return a.importURL(ctx, input, client)
}

func (a *App) importURL(ctx context.Context, input string, client *http.Client) (importer.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	target, err := resolveImportURL(input)
	if err != nil {
		return importer.Result{}, err
	}
	if target.metadata {
		raw, _, err := fetchImport(ctx, client, target.url)
		if err != nil {
			return importer.Result{}, err
		}
		var metadata struct {
			Node struct {
				ID    json.Number `json:"id"`
				Image string      `json:"max_res_url"`
			} `json:"node"`
		}
		if json.Unmarshal(raw, &metadata) != nil {
			return importer.Result{}, errors.New("来源网站返回的下载信息无法读取")
		}
		if !target.lorebook && metadata.Node.Image != "" {
			// Prefer the site's original PNG when it actually contains a card.
			image, _, imageErr := fetchImport(ctx, client, metadata.Node.Image)
			if imageErr == nil {
				if _, parseErr := card.ParsePNG(bytes.NewReader(image)); parseErr == nil {
					return a.ImportUpload(ctx, target.name+".png", bytes.NewReader(image))
				}
			}
		}
		if _, err := metadata.Node.ID.Int64(); err != nil {
			return importer.Result{}, errors.New("来源网站没有提供可下载的角色卡或世界书")
		}
		file := "tavern_raw.json"
		if target.lorebook {
			file = "sillytavern_raw.json"
		}
		target.url = "https://api.chub.ai/api/v4/projects/" + metadata.Node.ID.String() + "/repository/files/raw%252F" + file + "/raw"
		target.name += ".json"
	}
	raw, filename, err := fetchImport(ctx, client, target.url)
	if err != nil {
		return importer.Result{}, err
	}
	if target.name != "" {
		filename = target.name
	}
	if target.pygmalion {
		var envelope struct {
			Character json.RawMessage `json:"character"`
		}
		if json.Unmarshal(raw, &envelope) != nil || len(envelope.Character) == 0 {
			return importer.Result{}, errors.New("来源网站没有返回角色卡")
		}
		raw = envelope.Character // Preserve the exported card object, including unknown fields.
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.HasPrefix(trimmed, []byte("<")) {
		return importer.Result{}, errors.New("链接返回的是网页或验证页面，请复制附件下载链接")
	}
	if !card.Supported(filename) {
		if bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
			filename += ".png"
		} else {
			filename += ".json"
		}
	}
	result, err := a.ImportUpload(ctx, filename, bytes.NewReader(raw))
	if err != nil {
		return result, fmt.Errorf("文件已下载，但未能收录：%w", err)
	}
	return result, nil
}

type importTarget struct {
	url, name                     string
	metadata, lorebook, pygmalion bool
}

var importID = regexp.MustCompile(`^[\pL\pN_.-]+$`)
var uuidPattern = regexp.MustCompile(`(?i)^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

func resolveImportURL(input string) (importTarget, error) {
	input = strings.TrimSpace(input)
	if len(input) > 16<<10 {
		return importTarget{}, errors.New("链接过长")
	}
	if !strings.Contains(input, "://") {
		if uuidPattern.MatchString(input) {
			return importTarget{url: "https://server.pygmalion.chat/api/export/character/" + input + "/v2", name: input + ".json", pygmalion: true}, nil
		}
		if strings.HasPrefix(input, "AICC/") {
			input = "https://aicharactercards.com/" + strings.TrimPrefix(input, "AICC/")
		} else {
			input = "https://chub.ai/" + input
		}
	}
	u, err := url.Parse(input)
	if err != nil || validateImportURL(u) != nil {
		return importTarget{}, errors.New("请输入完整的 HTTP(S) 附件链接或支持的社区角色卡地址")
	}
	target := importTarget{url: u.String()}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	validParts := func(parts []string) bool {
		for _, p := range parts {
			if !importID.MatchString(p) || p == "." || p == ".." {
				return false
			}
		}
		return true
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "media.discordapp.net":
		if strings.HasPrefix(u.Path, "/attachments/") || strings.HasPrefix(u.Path, "/ephemeral-attachments/") {
			// The media proxy may resize/re-encode the preview and drop card metadata.
			// Request the attachment itself, preserving the signed query components.
			u.Host = "cdn.discordapp.com"
			query := make([]string, 0)
			for _, part := range strings.Split(u.RawQuery, "&") {
				key, _, _ := strings.Cut(part, "=")
				key, _ = url.QueryUnescape(key)
				switch key {
				case "width", "height", "format", "quality", "lossless", "animated", "fit":
					continue
				}
				if part != "" {
					query = append(query, part)
				}
			}
			u.RawQuery = strings.Join(query, "&")
			target.url = u.String()
		}
	case "chub.ai", "www.chub.ai", "venus.chub.ai", "characterhub.org", "www.characterhub.org":
		kind := "characters"
		if len(parts) == 3 && (parts[0] == "characters" || parts[0] == "lorebooks") {
			kind = parts[0]
			parts = parts[1:]
		}
		if len(parts) != 2 || !validParts(parts) {
			return importTarget{}, errors.New("请复制 Chub 角色卡或世界书详情链接")
		}
		target = importTarget{url: "https://api.chub.ai/api/" + kind + "/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "?full=true", name: parts[1], metadata: true, lorebook: kind == "lorebooks"}
	case "realm.risuai.net":
		if len(parts) == 2 && parts[0] == "character" && validParts(parts[1:]) {
			target = importTarget{url: "https://realm.risuai.net/api/v1/download/png-v3/" + url.PathEscape(parts[1]) + "?non_commercial=true", name: parts[1] + ".png"}
		}
	case "aicharactercards.com", "www.aicharactercards.com":
		if len(parts) >= 2 && !strings.HasPrefix(u.Path, "/wp-json/") && validParts(parts[len(parts)-2:]) {
			tail := parts[len(parts)-2:]
			target = importTarget{url: "https://aicharactercards.com/wp-json/pngapi/v1/image/" + url.PathEscape(tail[0]) + "/" + url.PathEscape(tail[1]), name: tail[1] + ".png"}
		}
	case "pygmalion.chat", "www.pygmalion.chat":
		for _, p := range parts {
			if uuidPattern.MatchString(p) {
				target = importTarget{url: "https://server.pygmalion.chat/api/export/character/" + p + "/v2", name: p + ".json", pygmalion: true}
				break
			}
		}
	}
	return target, nil
}

func validateImportURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return errors.New("只支持无需登录的 HTTP(S) 下载链接")
	}
	return nil
}

func publicImportIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !netip.MustParsePrefix("100.64.0.0/10").Contains(ip)
}

func newImportClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Direct connections pin validated DNS answers. An explicitly configured
	// HTTP(S)_PROXY owns remote DNS and routing, including fake-IP environments.
	proxyAddresses := make(map[string]bool)
	var proxyMu sync.Mutex
	transport.Proxy = func(req *http.Request) (*url.URL, error) {
		if err := validateRemoteImportHost(req.URL); err != nil {
			return nil, err
		}
		proxy, err := http.ProxyFromEnvironment(req)
		if err != nil || proxy == nil {
			return proxy, err
		}
		port := proxy.Port()
		if port == "" {
			port = "80"
			if proxy.Scheme == "https" {
				port = "443"
			}
		}
		proxyMu.Lock()
		proxyAddresses[net.JoinHostPort(proxy.Hostname(), port)] = true
		proxyMu.Unlock()
		return proxy, nil
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		proxyMu.Lock()
		isProxy := proxyAddresses[address]
		proxyMu.Unlock()
		if isProxy {
			return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, address)
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !publicImportIP(ip) {
				return nil, errors.New("链接不能指向本机或内网地址")
			}
		}
		dialer := net.Dialer{Timeout: 15 * time.Second}
		for _, ip := range ips {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			err = dialErr
		}
		if err == nil {
			err = errors.New("域名没有可用地址")
		}
		return nil, err
	}
	transport.ResponseHeaderTimeout = 20 * time.Second
	return &http.Client{Transport: transport, Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("下载链接重定向次数过多")
		}
		if err := validateImportURL(req.URL); err != nil {
			return err
		}
		return validateRemoteImportHost(req.URL)
	}}
}

func validateRemoteImportHost(u *url.URL) error {
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || !strings.Contains(host, ".") && !strings.Contains(host, ":") {
		return errors.New("链接不能指向本机或内网地址")
	}
	if ip, err := netip.ParseAddr(host); err == nil && !publicImportIP(ip) {
		return errors.New("链接不能指向本机或内网地址")
	}
	return nil
}

func fetchImport(ctx context.Context, client *http.Client, address string) ([]byte, string, error) {
	u, err := url.Parse(address)
	if err != nil || validateImportURL(u) != nil {
		return nil, "", errors.New("来源网站提供了无效下载链接")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", errors.New("无法创建下载请求")
	}
	req.Header.Set("User-Agent", "TavernShelf/1.0")
	req.Header.Set("Accept", "image/png, application/json, application/octet-stream;q=0.9, */*;q=0.5")
	response, err := client.Do(req)
	if err != nil {
		// url.Error embeds the signed URL; never send it back in errors or logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, "", fmt.Errorf("下载失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("下载失败（HTTP %d），链接可能已过期、需要登录或被来源网站拦截；请重新复制附件链接", response.StatusCode)
	}
	if response.ContentLength > MaxUploadSize {
		return nil, "", ErrUploadTooLarge
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxUploadSize+1))
	if err != nil {
		return nil, "", errors.New("下载中断，请重试")
	}
	if int64(len(raw)) > MaxUploadSize {
		return nil, "", ErrUploadTooLarge
	}
	filename := path.Base(response.Request.URL.Path)
	if _, params, e := mime.ParseMediaType(response.Header.Get("Content-Disposition")); e == nil && params["filename"] != "" {
		filename = path.Base(strings.ReplaceAll(params["filename"], "\\", "/"))
	}
	filename = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, filename)
	filename = strings.Trim(filename, " .")
	if filename == "" {
		filename = "download"
	}
	if len(filename) > 180 {
		filename = "download"
	}
	return raw, filename, nil
}
