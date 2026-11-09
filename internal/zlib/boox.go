package zlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BooxConfig points at a Boox (Onyx) e-reader exposing BooxDrop on the local
// network. Library selects whether uploads import into the device library or
// only land in the device's Books folder.
type BooxConfig struct {
	IP      string `json:"ip"`
	Library bool   `json:"library,omitempty"`
}

const booxPort = 8085

// booxUploadTimeout covers slow Wi-Fi transfers of large books.
const booxUploadTimeout = 5 * time.Minute

// BooxDevice describes a BooxDrop device found on the network.
type BooxDevice struct {
	IP    string
	Model string
}

func booxBaseURL(ip string) string {
	return fmt.Sprintf("http://%s:%d", ip, booxPort)
}

var booxHTTPClient = &http.Client{Timeout: 10 * time.Second}

// BooxPing reports whether baseURL answers BooxDrop's /api/ping check.
func BooxPing(baseURL string) bool {
	resp, err := booxHTTPClient.Get(baseURL + "/api/ping")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(body)) == "ok"
}

// BooxDeviceModel fetches the human-readable model name from BooxDrop.
func BooxDeviceModel(baseURL string) string {
	resp, err := booxHTTPClient.Get(baseURL + "/api/device")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var info struct {
		Model string `json:"model"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&info) != nil {
		return ""
	}
	return info.Model
}

// BooxScan probes the local network for devices listening on BooxDrop's
// port and answering its ping check. It prefers the local subnet and falls
// back to common home-router defaults.
func BooxScan() ([]BooxDevice, error) {
	var ips []string
	for _, prefix := range booxNetworkPrefixes() {
		for i := 1; i <= 254; i++ {
			ips = append(ips, fmt.Sprintf("%s.%d", prefix, i))
			if len(ips) >= 762 {
				break
			}
		}
		if len(ips) >= 762 {
			break
		}
	}

	probeTimeout := 800 * time.Millisecond
	var mu sync.Mutex
	var open []string
	var wg sync.WaitGroup
	sem := make(chan struct{}, 128)
	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, booxPort), probeTimeout)
			if err != nil {
				return
			}
			conn.Close()
			mu.Lock()
			open = append(open, ip)
			mu.Unlock()
		}(ip)
	}
	wg.Wait()

	devices := make([]BooxDevice, 0, len(open))
	for _, ip := range open {
		baseURL := booxBaseURL(ip)
		if !BooxPing(baseURL) {
			continue
		}
		model := BooxDeviceModel(baseURL)
		if model == "" {
			model = "BOOX Device"
		}
		devices = append(devices, BooxDevice{IP: ip, Model: model})
	}
	return devices, nil
}

// booxNetworkPrefixes returns the local /24 first, then common defaults.
func booxNetworkPrefixes() []string {
	var prefixes []string
	if conn, err := net.DialTimeout("udp", "8.8.8.8:80", 500*time.Millisecond); err == nil {
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			parts := strings.Split(addr.IP.String(), ".")
			if len(parts) == 4 {
				prefixes = append(prefixes, strings.Join(parts[:3], "."))
			}
		}
		conn.Close()
	}
	for _, prefix := range []string{
		"192.168.0", "192.168.1", "192.168.2", "192.168.3",
		"192.168.31", "192.168.50", "192.168.88",
		"10.0.0", "10.0.1", "172.16.0", "172.16.1",
	} {
		found := false
		for _, p := range prefixes {
			if p == prefix {
				found = true
				break
			}
		}
		if !found {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

type booxUploadResponse struct {
	Code       int    `json:"code"`
	Successful bool   `json:"successful"`
	Message    string `json:"message"`
}

// SendToBoox uploads filePath to the device in cfg via BooxDrop and returns
// where the file landed on the device.
func SendToBoox(filePath string, cfg BooxConfig) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	endpoint := "/api/storage/upload"
	if cfg.Library {
		endpoint = "/api/library/upload"
	}
	url := booxBaseURL(cfg.IP) + endpoint

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	client := &http.Client{Timeout: booxUploadTimeout}
	resp, err := client.Post(url, writer.FormDataContentType(), &body)
	if err != nil {
		return "", fmt.Errorf("cannot reach Boox at %s: %w", cfg.IP, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var parsed booxUploadResponse
	_ = json.Unmarshal(data, &parsed)
	if resp.StatusCode != http.StatusOK || !(parsed.Code == 0 || parsed.Successful) {
		return "", fmt.Errorf("Boox upload failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if parsed.Message != "" {
		return parsed.Message, nil
	}
	return filepath.Base(filePath), nil
}
