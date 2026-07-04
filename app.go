package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const apiBaseURL = "https://iot.huabot.com"

type App struct {
	ctx context.Context

	mu          sync.Mutex
	token       string
	userName    string
	config      *PeriodicConfig
	certificate *CertificatePaths
	rootDir     string
	cmd         *exec.Cmd
	logs        []string
	lastError   string
}

type APIErrorResponse struct {
	Err   string `json:"err"`
	Error string `json:"error"`
}

type LoginResponse struct {
	Token string         `json:"token"`
	User  map[string]any `json:"user"`
}

type PeriodicConfigResponse struct {
	Config *PeriodicConfig `json:"config"`
}

type PeriodicConfig struct {
	UID          int               `json:"uid"`
	PeriodicPort string            `json:"periodic_port"`
	RSAMode      string            `json:"rsa_mode"`
	ClientName   string            `json:"client_name"`
	ClientToken  string            `json:"client_token"`
	FuncPrefix   string            `json:"func_prefix"`
	Env          map[string]string `json:"env"`
	EnvText      string            `json:"env_text"`
	CreatedAt    int64             `json:"created_at"`
	UpdatedAt    int64             `json:"updated_at"`
}

type CertificatePaths struct {
	ServerPublicPath  string `json:"server_public_path"`
	ClientPrivatePath string `json:"client_private_path"`
}

type AppStatus struct {
	APIBaseURL   string            `json:"api_base_url"`
	LoggedIn     bool              `json:"logged_in"`
	UserName     string            `json:"user_name"`
	Config       *PeriodicConfig   `json:"config"`
	Certificate  *CertificatePaths `json:"certificate"`
	RootDir      string            `json:"root_dir"`
	Running      bool              `json:"running"`
	LastError    string            `json:"last_error"`
	Logs         []string          `json:"logs"`
	BinaryTarget string            `json:"binary_target"`
}

type StartOptions struct {
	Thread      int  `json:"thread"`
	AllowDelete bool `json:"allow_delete"`
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	_, _ = a.StopFileProxy()
}

func (a *App) Login(name string, passwd string) (AppStatus, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return a.Status(), errors.New("username is required")
	}
	if passwd == "" {
		return a.Status(), errors.New("password is required")
	}

	form := url.Values{}
	form.Set("name", name)
	form.Set("passwd", passwd)
	var out LoginResponse
	if err := a.doForm("POST", "/api/signin/", form, "", &out); err != nil {
		return a.Status(), err
	}
	if strings.TrimSpace(out.Token) == "" {
		return a.Status(), errors.New("login response did not include token")
	}

	a.mu.Lock()
	a.token = out.Token
	a.userName = name
	a.config = nil
	a.certificate = nil
	a.lastError = ""
	a.appendLogLocked("logged in as " + name)
	a.mu.Unlock()

	return a.Status(), nil
}

func (a *App) EnsurePeriodicConfig() (AppStatus, error) {
	token := a.currentToken()
	if token == "" {
		return a.Status(), errors.New("login is required")
	}

	var out PeriodicConfigResponse
	if err := a.doJSON("GET", "/api/sandbox/v1/periodic-config", token, nil, &out); err != nil {
		return a.Status(), err
	}
	if out.Config == nil {
		if err := a.doJSON("POST", "/api/sandbox/v1/periodic-config", token, nil, &out); err != nil {
			return a.Status(), err
		}
	}
	if out.Config == nil {
		return a.Status(), errors.New("periodic config response was empty")
	}

	a.mu.Lock()
	a.config = out.Config
	a.lastError = ""
	a.appendLogLocked("periodic config ready")
	a.mu.Unlock()

	return a.Status(), nil
}

func (a *App) PrepareCertificate() (AppStatus, error) {
	token := a.currentToken()
	if token == "" {
		return a.Status(), errors.New("login is required")
	}
	config := a.currentConfig()
	if config == nil {
		return a.Status(), errors.New("periodic config is required")
	}

	body, err := a.doRaw("GET", "/api/sandbox/v1/periodic-config/certificate", token)
	if err != nil {
		return a.Status(), err
	}
	serverPublic, clientPrivate, err := splitCertificateBundle(string(body))
	if err != nil {
		return a.Status(), err
	}

	dir, err := appConfigDir()
	if err != nil {
		return a.Status(), err
	}
	certDir := filepath.Join(dir, "certs")
	if err := os.MkdirAll(certDir, 0o700); err != nil {
		return a.Status(), err
	}

	serverPath := filepath.Join(certDir, fmt.Sprintf("periodic-server-public-%d.pem", config.UID))
	privatePath := filepath.Join(certDir, fmt.Sprintf("periodic-client-private-%d.pem", config.UID))
	if err := os.WriteFile(serverPath, []byte(serverPublic), 0o644); err != nil {
		return a.Status(), err
	}
	if err := os.WriteFile(privatePath, []byte(clientPrivate), 0o600); err != nil {
		return a.Status(), err
	}

	paths := &CertificatePaths{
		ServerPublicPath:  serverPath,
		ClientPrivatePath: privatePath,
	}
	a.mu.Lock()
	a.certificate = paths
	a.lastError = ""
	a.appendLogLocked("certificate files written")
	a.mu.Unlock()

	return a.Status(), nil
}

func (a *App) SelectRootDirectory() (AppStatus, error) {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select file proxy root directory",
	})
	if err != nil {
		return a.Status(), err
	}
	if dir == "" {
		return a.Status(), nil
	}

	a.mu.Lock()
	a.rootDir = dir
	a.lastError = ""
	a.appendLogLocked("root directory selected: " + dir)
	a.mu.Unlock()
	return a.Status(), nil
}

func (a *App) StartFileProxy(options StartOptions) (AppStatus, error) {
	a.mu.Lock()
	if a.cmd != nil && a.cmd.Process != nil {
		a.mu.Unlock()
		return a.Status(), errors.New("file-proxy is already running")
	}
	config := a.config
	cert := a.certificate
	root := a.rootDir
	a.mu.Unlock()

	if strings.TrimSpace(root) == "" {
		return a.Status(), errors.New("root directory is required")
	}

	binaryPath, err := extractBundledFileProxy()
	if err != nil {
		return a.Status(), err
	}
	thread := options.Thread
	if thread <= 0 {
		thread = 10
	}
	args := buildFileProxyArgs(root, thread, options.AllowDelete, config, cert)

	cmd := exec.Command(binaryPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return a.Status(), err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return a.Status(), err
	}
	if err := cmd.Start(); err != nil {
		return a.Status(), err
	}

	a.mu.Lock()
	a.cmd = cmd
	a.lastError = ""
	a.appendLogLocked("file-proxy started")
	a.mu.Unlock()

	go a.captureOutput("stdout", stdout)
	go a.captureOutput("stderr", stderr)
	go a.waitProcess(cmd)

	return a.Status(), nil
}

func buildFileProxyArgs(
	root string,
	thread int,
	allowDelete bool,
	config *PeriodicConfig,
	cert *CertificatePaths,
) []string {
	args := []string{
		"--root", root,
		"--thread", fmt.Sprint(thread),
	}
	if config != nil && cert != nil {
		args = append(args,
			"--host", config.PeriodicPort,
			"--rsa-private-path", cert.ClientPrivatePath,
			"--rsa-public-path", cert.ServerPublicPath,
			"--rsa-mode", config.RSAMode,
			"--client-name", config.ClientName,
			"--client-token", config.ClientToken,
			"--prefix", config.FuncPrefix,
		)
	}
	if allowDelete {
		args = append(args, "--allow-delete")
	}
	return args
}

func (a *App) StopFileProxy() (AppStatus, error) {
	a.mu.Lock()
	cmd := a.cmd
	a.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return a.Status(), nil
	}

	err := cmd.Process.Signal(os.Interrupt)
	if err != nil {
		err = cmd.Process.Kill()
	}
	return a.Status(), err
}

func (a *App) Status() AppStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *App) statusLocked() AppStatus {
	logs := append([]string(nil), a.logs...)
	return AppStatus{
		APIBaseURL:   apiBaseURL,
		LoggedIn:     a.token != "",
		UserName:     a.userName,
		Config:       a.config,
		Certificate:  a.certificate,
		RootDir:      a.rootDir,
		Running:      a.cmd != nil && a.cmd.Process != nil,
		LastError:    a.lastError,
		Logs:         logs,
		BinaryTarget: binaryTarget(),
	}
}

func (a *App) currentToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.token
}

func (a *App) currentConfig() *PeriodicConfig {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config
}

func (a *App) doForm(method string, path string, form url.Values, token string, out any) error {
	req, err := http.NewRequest(method, apiBaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	if token != "" {
		req.Header.Set("X-REQUEST-TOKEN", token)
	}
	return a.doRequest(req, out)
}

func (a *App) doJSON(method string, path string, token string, body io.Reader, out any) error {
	req, err := http.NewRequest(method, apiBaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("X-REQUEST-TOKEN", token)
	}
	return a.doRequest(req, out)
}

func (a *App) doRaw(method string, path string, token string) ([]byte, error) {
	req, err := http.NewRequest(method, apiBaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	if token != "" {
		req.Header.Set("X-REQUEST-TOKEN", token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	data, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}
	if rsp.StatusCode < 200 || rsp.StatusCode >= 300 {
		return nil, decodeAPIError(data, rsp.Status)
	}
	return data, nil
}

func (a *App) doRequest(req *http.Request, out any) error {
	client := &http.Client{Timeout: 30 * time.Second}
	rsp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	data, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}
	if rsp.StatusCode < 200 || rsp.StatusCode >= 300 {
		return decodeAPIError(data, rsp.Status)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func decodeAPIError(data []byte, fallback string) error {
	var apiErr APIErrorResponse
	if err := json.Unmarshal(data, &apiErr); err == nil {
		if apiErr.Err != "" {
			return errors.New(apiErr.Err)
		}
		if apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}
	}
	text := strings.TrimSpace(string(data))
	if text != "" {
		return errors.New(text)
	}
	return errors.New(fallback)
}

func splitCertificateBundle(bundle string) (string, string, error) {
	blocks := make([]string, 0, 2)
	remaining := []byte(bundle)
	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		blocks = append(blocks, string(pem.EncodeToMemory(block)))
		remaining = rest
	}
	if len(blocks) < 2 {
		return "", "", errors.New("certificate bundle must include server public key and client private key")
	}
	return blocks[0], blocks[1], nil
}

func appConfigDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, "MynaFileProxy")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func binaryTarget() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func bundledBinaryName() string {
	if runtime.GOOS == "windows" {
		return "file-proxy.exe"
	}
	return "file-proxy"
}

func extractBundledFileProxy() (string, error) {
	target := binaryTarget()
	name := bundledBinaryName()
	embeddedPath := filepath.ToSlash(filepath.Join("bin", target, name))
	data, err := bundledBinaries.ReadFile(embeddedPath)
	if err != nil {
		return "", fmt.Errorf("bundled file-proxy binary missing for %s: %w", target, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return "", fmt.Errorf("bundled file-proxy binary is empty for %s", target)
	}

	dir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	outPath := filepath.Join(dir, "bin", target, name)
	info, statErr := os.Stat(outPath)
	if statErr == nil && info.Size() == int64(len(data)) {
		return outPath, nil
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", err
	}
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.WriteFile(outPath, data, mode); err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(outPath, 0o755)
	}
	return outPath, nil
}

func (a *App) captureOutput(label string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		a.mu.Lock()
		a.appendLogLocked(label + ": " + scanner.Text())
		a.mu.Unlock()
	}
	if err := scanner.Err(); err != nil {
		a.mu.Lock()
		a.appendLogLocked(label + " read error: " + err.Error())
		a.mu.Unlock()
	}
}

func (a *App) waitProcess(cmd *exec.Cmd) {
	err := cmd.Wait()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cmd == cmd {
		a.cmd = nil
	}
	if err != nil {
		a.lastError = err.Error()
		a.appendLogLocked("file-proxy exited: " + err.Error())
		return
	}
	a.appendLogLocked("file-proxy stopped")
}

func (a *App) appendLogLocked(line string) {
	const maxLogs = 300
	a.logs = append(a.logs, time.Now().Format("15:04:05")+" "+line)
	if len(a.logs) > maxLogs {
		a.logs = a.logs[len(a.logs)-maxLogs:]
	}
}
