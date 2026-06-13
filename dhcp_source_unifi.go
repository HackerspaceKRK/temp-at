package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

// UnifiWifiSource talks to a classic self-hosted UniFi Network controller
// (the /api/... endpoints, not UniFi OS). It keeps a cookie-jar session and
// re-logs in transparently when the session expires.
type UnifiWifiSource struct {
	cfg    DhcpWifiSourceConfig
	client *http.Client

	mu       sync.Mutex // serializes login and protects loggedIn
	loggedIn bool
}

func NewUnifiWifiSource(cfg DhcpWifiSourceConfig) *UnifiWifiSource {
	jar, _ := cookiejar.New(nil)
	return &UnifiWifiSource{
		cfg: cfg,
		client: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify},
			},
		},
	}
}

func (u *UnifiWifiSource) Name() string { return u.cfg.Name }

func (u *UnifiWifiSource) baseURL() string {
	return strings.TrimRight(u.cfg.URL, "/")
}

// login authenticates and stores the session cookie in the jar.
func (u *UnifiWifiSource) login(ctx context.Context) error {
	body, _ := json.Marshal(map[string]any{
		"username": u.cfg.Username,
		"password": u.cfg.Password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.baseURL()+"/api/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("unifi login: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unifi login: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// getJSON issues a GET against the controller, decoding the response into out.
// On a 401/403 it re-logs in once and retries.
func (u *UnifiWifiSource) getJSON(ctx context.Context, path string, out any) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !u.loggedIn {
		if err := u.login(ctx); err != nil {
			return err
		}
		u.loggedIn = true
	}

	status, err := u.doGet(ctx, path, out)
	if err == nil {
		return nil
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		// Session expired: re-login once and retry.
		u.loggedIn = false
		if err := u.login(ctx); err != nil {
			return err
		}
		u.loggedIn = true
		if _, err := u.doGet(ctx, path, out); err != nil {
			return err
		}
		return nil
	}
	return err
}

// doGet performs a single GET, returning the HTTP status code (or 0 on a
// transport error) alongside any error.
func (u *UnifiWifiSource) doGet(ctx context.Context, path string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.baseURL()+path, nil)
	if err != nil {
		return 0, err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("unifi GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, fmt.Errorf("unifi GET %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("unifi GET %s: decode: %w", path, err)
	}
	return resp.StatusCode, nil
}

// unifiDevice is one access point / switch from stat/device-basic.
type unifiDevice struct {
	Mac   string `json:"mac"`
	Name  string `json:"name"`
	Model string `json:"model"`
	Type  string `json:"type"`
}

// unifiClient is one associated station from stat/sta.
type unifiClient struct {
	Mac     string `json:"mac"`
	ApMac   string `json:"ap_mac"`
	Essid   string `json:"essid"`
	Rssi    int    `json:"rssi"`
	Signal  int    `json:"signal"`
	IsWired bool   `json:"is_wired"`
}

func (u *UnifiWifiSource) FetchClients(ctx context.Context) (map[string]WifiClientInfo, error) {
	site := u.cfg.Site
	if site == "" {
		site = "default"
	}

	var devResp struct {
		Data []unifiDevice `json:"data"`
	}
	if err := u.getJSON(ctx, "/api/s/"+site+"/stat/device-basic", &devResp); err != nil {
		return nil, err
	}
	apNames := make(map[string]string, len(devResp.Data))
	for _, d := range devResp.Data {
		if d.Type != "uap" {
			continue
		}
		name := firstNonEmpty(d.Name, d.Model, d.Mac)
		apNames[NormalizeMac(d.Mac)] = name
	}

	var staResp struct {
		Data []unifiClient `json:"data"`
	}
	if err := u.getJSON(ctx, "/api/s/"+site+"/stat/sta", &staResp); err != nil {
		return nil, err
	}

	clients := make(map[string]WifiClientInfo, len(staResp.Data))
	for _, c := range staResp.Data {
		if c.IsWired {
			continue
		}
		clients[NormalizeMac(c.Mac)] = WifiClientInfo{
			SourceName: u.cfg.Name,
			ApName:     apNames[NormalizeMac(c.ApMac)],
			Ssid:       c.Essid,
			Rssi:       c.Rssi,
			SignalDbm:  c.Signal,
		}
	}
	return clients, nil
}
