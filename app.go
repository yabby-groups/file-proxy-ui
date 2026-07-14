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

	"github.com/pkg/browser"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultAPIBaseURL = "https://iot.huabot.com"
	defaultThread     = 4
	maxThread         = 16
	defaultWebPort    = 8080
	maxWebPort        = 65535
)

type App struct {
	ctx context.Context

	mu           sync.Mutex
	apiBaseURL   string
	token        string
	userName     string
	userInfo     UserInfo
	config       *PeriodicConfig
	certificate  *CertificatePaths
	rootDir      string
	startOptions StartOptions
	cmd          *exec.Cmd
	webCmd       *exec.Cmd
	logs         []string
	lastError    string
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

type UserInfo struct {
	Name      string `json:"name"`
	NickName  string `json:"nick_name"`
	AvatarURL string `json:"avatar_url"`
}

type AppStatus struct {
	APIBaseURL   string            `json:"api_base_url"`
	LoggedIn     bool              `json:"logged_in"`
	UserName     string            `json:"user_name"`
	UserInfo     UserInfo          `json:"user_info"`
	Config       *PeriodicConfig   `json:"config"`
	Certificate  *CertificatePaths `json:"certificate"`
	RootDir      string            `json:"root_dir"`
	StartOptions StartOptions      `json:"start_options"`
	Running      bool              `json:"running"`
	WebRunning   bool              `json:"web_running"`
	WebURL       string            `json:"web_url"`
	LastError    string            `json:"last_error"`
	Logs         []string          `json:"logs"`
	BinaryTarget string            `json:"binary_target"`
}

type StartOptions struct {
	Thread          int  `json:"thread"`
	AllowDelete     bool `json:"allow_delete"`
	Port            int  `json:"port"`
	AutoOpenBrowser bool `json:"auto_open_browser"`
}

type StoredSettings struct {
	APIBaseURL   string       `json:"api_base_url"`
	RootDir      string       `json:"root_dir"`
	StartOptions StartOptions `json:"start_options"`
	Token        string       `json:"token"`
	UserName     string       `json:"user_name"`
	UserInfo     UserInfo     `json:"user_info"`
}

func NewApp() *App {
	return &App{
		apiBaseURL:   defaultAPIBaseURL,
		startOptions: defaultStartOptions(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.loadSettings(); err != nil {
		a.mu.Lock()
		a.lastError = err.Error()
		a.appendLogLocked("load saved settings failed: " + err.Error())
		a.mu.Unlock()
	}
	if _, err := extractBundledWorkers(); err != nil {
		a.mu.Lock()
		a.lastError = err.Error()
		a.appendLogLocked("prepare bundled workers failed: " + err.Error())
		a.mu.Unlock()
	}
	if a.currentToken() != "" {
		go a.prepareSavedLogin()
	}
}

func (a *App) shutdown(ctx context.Context) {
	_, _ = a.stopAll()
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
	a.userInfo = extractUserInfo(name, out.User)
	a.config = nil
	a.certificate = nil
	a.lastError = ""
	a.appendLogLocked("logged in as " + name)
	saveErr := a.saveSettingsLocked()
	a.mu.Unlock()
	if saveErr != nil {
		return a.Status(), saveErr
	}

	return a.ensurePeriodicAssets()
}

func (a *App) Logout() (AppStatus, error) {
	if _, err := a.stopAll(); err != nil {
		return a.Status(), err
	}

	a.mu.Lock()
	a.token = ""
	a.userName = ""
	a.userInfo = UserInfo{}
	a.config = nil
	a.certificate = nil
	a.lastError = ""
	a.appendLogLocked("logged out")
	saveErr := a.saveSettingsLocked()
	a.mu.Unlock()
	if saveErr != nil {
		return a.Status(), saveErr
	}
	return a.Status(), nil
}

func (a *App) SetAPIBaseURL(value string) (AppStatus, error) {
	next, err := normalizeAPIBaseURL(value)
	if err != nil {
		return a.Status(), err
	}

	a.mu.Lock()
	changed := a.apiBaseURL != next
	a.mu.Unlock()
	if changed {
		if _, err := a.stopAll(); err != nil {
			return a.Status(), err
		}
	}

	a.mu.Lock()
	a.apiBaseURL = next
	if changed {
		a.token = ""
		a.userName = ""
		a.userInfo = UserInfo{}
		a.config = nil
		a.certificate = nil
		a.appendLogLocked("api domain changed: " + next)
	} else {
		a.appendLogLocked("api domain saved: " + next)
	}
	a.lastError = ""
	saveErr := a.saveSettingsLocked()
	a.mu.Unlock()
	if saveErr != nil {
		return a.Status(), saveErr
	}
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

func (a *App) ensurePeriodicAssets() (AppStatus, error) {
	if _, err := a.EnsurePeriodicConfig(); err != nil {
		return a.Status(), err
	}
	return a.PrepareCertificate()
}

func (a *App) prepareSavedLogin() {
	if _, err := a.ensurePeriodicAssets(); err != nil {
		a.mu.Lock()
		a.lastError = err.Error()
		a.appendLogLocked("restore saved login failed: " + err.Error())
		a.mu.Unlock()
	}
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
	saveErr := a.saveSettingsLocked()
	a.mu.Unlock()
	if saveErr != nil {
		return a.Status(), saveErr
	}
	return a.Status(), nil
}

func (a *App) StartFileProxy(options StartOptions) (AppStatus, error) {
	options = normalizeStartOptions(options)
	a.mu.Lock()
	if a.cmd != nil && a.cmd.Process != nil {
		a.mu.Unlock()
		return a.Status(), errors.New("file-proxy is already running")
	}
	config := a.config
	cert := a.certificate
	root := a.rootDir
	token := a.token
	a.startOptions = options
	saveErr := a.saveSettingsLocked()
	a.mu.Unlock()
	if saveErr != nil {
		return a.Status(), saveErr
	}

	if strings.TrimSpace(root) == "" {
		return a.Status(), errors.New("root directory is required")
	}
	if message, err := ensureWorkerFileLimit(); err != nil {
		a.mu.Lock()
		a.appendLogLocked("raise file descriptor limit failed: " + err.Error())
		a.mu.Unlock()
	} else if message != "" {
		a.mu.Lock()
		a.appendLogLocked(message)
		a.mu.Unlock()
	}
	if token != "" && (config == nil || cert == nil) {
		if _, err := a.ensurePeriodicAssets(); err != nil {
			return a.Status(), err
		}
		a.mu.Lock()
		config = a.config
		cert = a.certificate
		a.mu.Unlock()
	}

	binaryPath, err := extractBundledFileProxy()
	if err != nil {
		return a.Status(), err
	}
	args := buildFileProxyArgs(root, options.Thread, options.AllowDelete, config, cert)

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = filepath.Dir(binaryPath)
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
	go a.waitProcess("file-proxy", cmd)

	return a.Status(), nil
}

func (a *App) StartFileProxyWeb(options StartOptions) (AppStatus, error) {
	options = normalizeStartOptions(options)
	a.mu.Lock()
	if a.webCmd != nil && a.webCmd.Process != nil {
		a.mu.Unlock()
		return a.Status(), errors.New("file-proxy-web is already running")
	}
	config := a.config
	cert := a.certificate
	token := a.token
	a.startOptions = options
	saveErr := a.saveSettingsLocked()
	a.mu.Unlock()
	if saveErr != nil {
		return a.Status(), saveErr
	}

	if token != "" && (config == nil || cert == nil) {
		if _, err := a.ensurePeriodicAssets(); err != nil {
			return a.Status(), err
		}
		a.mu.Lock()
		config = a.config
		cert = a.certificate
		a.mu.Unlock()
	}

	binaryPath, err := extractBundledFileProxyWeb()
	if err != nil {
		return a.Status(), err
	}
	cmd := exec.Command(binaryPath, buildFileProxyWebArgs(options.Port, config, cert)...)
	cmd.Dir = filepath.Dir(binaryPath)
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
	a.webCmd = cmd
	a.lastError = ""
	a.appendLogLocked("file-proxy-web started at " + webURL(options.Port))
	a.mu.Unlock()
	go a.captureOutput("file-proxy-web stdout", stdout)
	go a.captureOutput("file-proxy-web stderr", stderr)
	go a.waitProcess("file-proxy-web", cmd)

	url := webURL(options.Port)
	if options.AutoOpenBrowser {
		if err := browser.OpenURL(url); err != nil {
			a.mu.Lock()
			a.appendLogLocked("open web address failed: " + err.Error())
			a.mu.Unlock()
		}
	}
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

func buildFileProxyWebArgs(port int, config *PeriodicConfig, cert *CertificatePaths) []string {
	args := []string{
		"--host", "127.0.0.1",
		"--port", fmt.Sprint(port),
	}
	if config != nil && cert != nil {
		args = append(args,
			"--worker-host", config.PeriodicPort,
			"--rsa-private-path", cert.ClientPrivatePath,
			"--rsa-public-path", cert.ServerPublicPath,
			"--rsa-mode", config.RSAMode,
			"--client-name", config.ClientName,
			"--client-token", config.ClientToken,
			"--prefix", config.FuncPrefix,
		)
	}
	return args
}

func (a *App) StopFileProxy() (AppStatus, error) {
	a.mu.Lock()
	cmd := a.cmd
	a.mu.Unlock()
	return a.Status(), stopCommand(cmd)
}

func (a *App) StopFileProxyWeb() (AppStatus, error) {
	a.mu.Lock()
	cmd := a.webCmd
	a.mu.Unlock()
	return a.Status(), stopCommand(cmd)
}

func (a *App) OpenWebURL() (AppStatus, error) {
	status := a.Status()
	if !status.WebRunning {
		return status, errors.New("file-proxy-web is not running")
	}
	if err := browser.OpenURL(status.WebURL); err != nil {
		return a.Status(), err
	}
	return a.Status(), nil
}

func (a *App) stopAll() (AppStatus, error) {
	a.mu.Lock()
	worker := a.cmd
	web := a.webCmd
	a.mu.Unlock()
	if err := stopCommand(web); err != nil {
		return a.Status(), err
	}
	return a.Status(), stopCommand(worker)
}

func stopCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}

func (a *App) Status() AppStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *App) statusLocked() AppStatus {
	logs := append([]string(nil), a.logs...)
	return AppStatus{
		APIBaseURL:   a.apiBaseURL,
		LoggedIn:     a.token != "",
		UserName:     a.userName,
		UserInfo:     a.userInfo,
		Config:       a.config,
		Certificate:  a.certificate,
		RootDir:      a.rootDir,
		StartOptions: a.startOptions,
		Running:      a.cmd != nil && a.cmd.Process != nil,
		WebRunning:   a.webCmd != nil && a.webCmd.Process != nil,
		WebURL:       webURLForRunning(a.webCmd, a.startOptions.Port),
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

func (a *App) currentAPIBaseURL() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.apiBaseURL == "" {
		return defaultAPIBaseURL
	}
	return a.apiBaseURL
}

func (a *App) doForm(method string, path string, form url.Values, token string, out any) error {
	req, err := http.NewRequest(method, a.currentAPIBaseURL()+path, strings.NewReader(form.Encode()))
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
	req, err := http.NewRequest(method, a.currentAPIBaseURL()+path, body)
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
	req, err := http.NewRequest(method, a.currentAPIBaseURL()+path, nil)
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

func extractUserInfo(loginName string, user map[string]any) UserInfo {
	profile, _ := user["profile"].(map[string]any)
	info := UserInfo{
		Name:      firstString(user["name"], loginName),
		NickName:  firstString(user["nick_name"], profile["nick_name"]),
		AvatarURL: firstString(user["avatar_url"], profile["avatar_url"]),
	}
	if info.NickName == "" {
		info.NickName = firstString(profile["full_name"])
	}
	return info
}

func firstString(values ...any) string {
	for _, value := range values {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
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

func settingsPath() (string, error) {
	dir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func defaultStartOptions() StartOptions {
	return StartOptions{Thread: defaultThread, Port: defaultWebPort, AutoOpenBrowser: true}
}

func normalizeAPIBaseURL(value string) (string, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return "", errors.New("api domain is required")
	}
	if !strings.Contains(text, "://") {
		text = "https://" + text
	}
	parsed, err := url.Parse(text)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("api domain must start with http:// or https://")
	}
	if parsed.Host == "" {
		return "", errors.New("api domain host is required")
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func normalizeStartOptions(options StartOptions) StartOptions {
	if options.Thread <= 0 {
		options.Thread = defaultStartOptions().Thread
	}
	if options.Thread > maxThread {
		options.Thread = maxThread
	}
	if options.Port <= 0 || options.Port > maxWebPort {
		options.Port = defaultWebPort
	}
	return options
}

func (a *App) loadSettings() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var settings StoredSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("decode saved settings: %w", err)
	}

	a.mu.Lock()
	if settings.APIBaseURL != "" {
		apiBaseURL, err := normalizeAPIBaseURL(settings.APIBaseURL)
		if err != nil {
			a.mu.Unlock()
			return fmt.Errorf("decode saved api domain: %w", err)
		}
		a.apiBaseURL = apiBaseURL
	}
	a.rootDir = settings.RootDir
	a.startOptions = normalizeStartOptions(settings.StartOptions)
	a.token = strings.TrimSpace(settings.Token)
	a.userName = settings.UserName
	a.userInfo = settings.UserInfo
	if a.userInfo.Name == "" {
		a.userInfo.Name = a.userName
	}
	a.mu.Unlock()
	return nil
}

func (a *App) saveSettingsLocked() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	settings := StoredSettings{
		APIBaseURL:   a.apiBaseURL,
		RootDir:      a.rootDir,
		StartOptions: normalizeStartOptions(a.startOptions),
		Token:        a.token,
		UserName:     a.userName,
		UserInfo:     a.userInfo,
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func appRuntimeDir() (string, error) {
	root, err := os.UserCacheDir()
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

func bundledWebBinaryName() string {
	if runtime.GOOS == "windows" {
		return "file-proxy-web.exe"
	}
	return "file-proxy-web"
}

func removeQuarantineAttribute(path string) {
	if runtime.GOOS != "darwin" {
		return
	}
	_ = exec.Command("xattr", "-d", "com.apple.quarantine", path).Run()
}

func writeBundledFile(embeddedPath string, outPath string, mode os.FileMode) error {
	data, err := bundledBinaries.ReadFile(embeddedPath)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("bundled file is empty: %s", embeddedPath)
	}

	existing, readErr := os.ReadFile(outPath)
	if readErr == nil && bytes.Equal(existing, data) {
		if mode&0o111 != 0 {
			_ = os.Chmod(outPath, 0o755)
		}
		removeQuarantineAttribute(outPath)
		return nil
	}
	if readErr == nil {
		_ = os.Chmod(outPath, 0o600)
		if err := os.Remove(outPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, data, mode); err != nil {
		return err
	}
	if mode&0o111 != 0 {
		_ = os.Chmod(outPath, 0o755)
	}
	removeQuarantineAttribute(outPath)
	return nil
}

func extractBundledDirectoryFiles(embeddedDir string, outDir string, mode os.FileMode, skipName string) error {
	entries, err := bundledBinaries.ReadDir(embeddedDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == skipName {
			continue
		}
		embeddedPath := filepath.ToSlash(filepath.Join(embeddedDir, entry.Name()))
		outPath := filepath.Join(outDir, entry.Name())
		if err := writeBundledFile(embeddedPath, outPath, mode); err != nil {
			return fmt.Errorf("extract bundled support file %s: %w", embeddedPath, err)
		}
	}
	return nil
}

func extractBundledSupportFiles(target string, executablePath string) error {
	switch runtime.GOOS {
	case "darwin":
		embeddedDir := filepath.ToSlash(filepath.Join("bin", target, "lib", "file-proxy"))
		outDir := filepath.Clean(filepath.Join(filepath.Dir(executablePath), "..", "lib", "file-proxy"))
		return extractBundledDirectoryFiles(embeddedDir, outDir, 0o644, "")
	case "windows":
		embeddedDir := filepath.ToSlash(filepath.Join("bin", target))
		return extractBundledDirectoryFiles(embeddedDir, filepath.Dir(executablePath), 0o644, bundledBinaryName())
	default:
		return nil
	}
}

func extractBundledFileProxy() (string, error) {
	return extractBundledBinary(bundledBinaryName())
}

func extractBundledFileProxyWeb() (string, error) {
	return extractBundledBinary(bundledWebBinaryName())
}

func extractBundledWorkers() (string, error) {
	worker, err := extractBundledFileProxy()
	if err != nil {
		return "", err
	}
	if _, err := extractBundledFileProxyWeb(); err != nil {
		return "", err
	}
	return worker, nil
}

func extractBundledBinary(name string) (string, error) {
	target := binaryTarget()
	embeddedPath := filepath.ToSlash(filepath.Join("bin", target, name))

	dir, err := appRuntimeDir()
	if err != nil {
		return "", err
	}
	outPath := filepath.Join(dir, "bin", target, name)
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := writeBundledFile(embeddedPath, outPath, mode); err != nil {
		return "", fmt.Errorf("bundled %s binary missing for %s: %w", name, target, err)
	}
	if err := extractBundledSupportFiles(target, outPath); err != nil {
		return "", err
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

func (a *App) waitProcess(name string, cmd *exec.Cmd) {
	err := cmd.Wait()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cmd == cmd {
		a.cmd = nil
	}
	if a.webCmd == cmd {
		a.webCmd = nil
	}
	if err != nil {
		a.lastError = err.Error()
		a.appendLogLocked(name + " exited: " + err.Error())
		return
	}
	a.appendLogLocked(name + " stopped")
}

func webURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/", port)
}

func webURLForRunning(cmd *exec.Cmd, port int) string {
	if cmd == nil || cmd.Process == nil {
		return ""
	}
	return webURL(port)
}

func (a *App) appendLogLocked(line string) {
	const maxLogs = 300
	a.logs = append(a.logs, time.Now().Format("15:04:05")+" "+line)
	if len(a.logs) > maxLogs {
		a.logs = a.logs[len(a.logs)-maxLogs:]
	}
}
