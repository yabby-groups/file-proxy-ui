package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLoginForwardsTOTPCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/signin/" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse login form: %v", err)
		}
		if got := r.Form.Get("name"); got != "alice" {
			t.Errorf("name = %q, want alice", got)
		}
		if got := r.Form.Get("passwd"); got != "secret" {
			t.Errorf("passwd = %q, want secret", got)
		}
		if got := r.Form.Get("totp_code"); got != "123456" {
			t.Errorf("totp_code = %q, want 123456", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"err":"totp invalid"}`))
	}))
	defer server.Close()

	app := NewApp()
	app.apiBaseURL = server.URL
	_, err := app.Login(" alice ", "secret", " 123456 ")
	if err == nil || err.Error() != "totp invalid" {
		t.Fatalf("Login error = %v, want totp invalid", err)
	}
}

func TestSplitCertificateBundle(t *testing.T) {
	bundle := strings.Join([]string{
		"-----BEGIN RSA PUBLIC KEY-----",
		"AQID",
		"-----END RSA PUBLIC KEY-----",
		"-----BEGIN RSA PRIVATE KEY-----",
		"BAUG",
		"-----END RSA PRIVATE KEY-----",
	}, "\n")

	server, client, err := splitCertificateBundle(bundle)
	if err != nil {
		t.Fatalf("splitCertificateBundle returned error: %v", err)
	}
	if !strings.Contains(server, "RSA PUBLIC KEY") {
		t.Fatalf("server block was not public key: %q", server)
	}
	if !strings.Contains(client, "RSA PRIVATE KEY") {
		t.Fatalf("client block was not private key: %q", client)
	}
}

func TestSplitCertificateBundleRejectsSingleBlock(t *testing.T) {
	_, _, err := splitCertificateBundle("-----BEGIN RSA PUBLIC KEY-----\npublic\n-----END RSA PUBLIC KEY-----")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBundledBinaryName(t *testing.T) {
	if bundledBinaryName() == "" {
		t.Fatal("expected binary name")
	}
}

func TestWriteBundledFileOverwritesSameSizeDifferentContent(t *testing.T) {
	embeddedPath := filepath.ToSlash(filepath.Join("bin", "darwin-arm64", "file-proxy"))
	data, err := bundledBinaries.ReadFile(embeddedPath)
	if err != nil {
		t.Skipf("bundled test binary unavailable: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("bundled test binary is empty")
	}

	outPath := filepath.Join(t.TempDir(), "file-proxy")
	stale := append([]byte(nil), data...)
	stale[0] ^= 0xff
	if err := os.WriteFile(outPath, stale, 0o755); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	if err := writeBundledFile(embeddedPath, outPath, 0o755); err != nil {
		t.Fatalf("writeBundledFile returned error: %v", err)
	}
	written, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if !reflect.DeepEqual(written, data) {
		t.Fatal("expected stale same-size file to be overwritten")
	}
}

func TestExtractBundledDirectoryFilesCopiesWindowsSiblings(t *testing.T) {
	embeddedDir := filepath.ToSlash(filepath.Join("bin", "windows-amd64"))
	entries, err := bundledBinaries.ReadDir(embeddedDir)
	if err != nil {
		t.Skipf("bundled windows test files unavailable: %v", err)
	}

	hasSupportFile := false
	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() != "file-proxy.exe" {
			hasSupportFile = true
			break
		}
	}
	if !hasSupportFile {
		t.Skip("bundled windows support files unavailable")
	}

	outDir := t.TempDir()
	if err := extractBundledDirectoryFiles(embeddedDir, outDir, 0o644, "file-proxy.exe"); err != nil {
		t.Fatalf("extractBundledDirectoryFiles returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "file-proxy.exe")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expected main file-proxy.exe to be skipped")
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "file-proxy.exe" {
			continue
		}
		info, err := os.Stat(filepath.Join(outDir, entry.Name()))
		if err != nil {
			t.Fatalf("expected support file %s to be copied: %v", entry.Name(), err)
		}
		if info.Size() == 0 {
			t.Fatalf("expected support file %s to be non-empty", entry.Name())
		}
	}
}

func TestStartupExtractsBundledFileProxy(t *testing.T) {
	configRoot := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configRoot)
		t.Setenv("LocalAppData", configRoot)
	} else {
		t.Setenv("HOME", configRoot)
		t.Setenv("XDG_CONFIG_HOME", configRoot)
		t.Setenv("XDG_CACHE_HOME", filepath.Join(configRoot, "cache"))
	}

	embeddedPath := filepath.ToSlash(filepath.Join("bin", binaryTarget(), bundledBinaryName()))
	expected, err := bundledBinaries.ReadFile(embeddedPath)
	if err != nil {
		t.Skipf("bundled test binary unavailable for %s: %v", binaryTarget(), err)
	}

	app := NewApp()
	app.startup(context.Background())
	status := app.Status()
	if status.LastError != "" {
		t.Fatalf("startup returned error: %s", status.LastError)
	}

	dir, err := appRuntimeDir()
	if err != nil {
		t.Fatalf("appRuntimeDir returned error: %v", err)
	}
	actual, err := os.ReadFile(filepath.Join(dir, "bin", binaryTarget(), bundledBinaryName()))
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("expected startup to extract bundled file-proxy")
	}
}

func TestBuildFileProxyArgsAllowsMissingPeriodicConfig(t *testing.T) {
	args := buildFileProxyArgs("/tmp/root", 12, true, nil, nil)
	expected := []string{
		"--root", "/tmp/root",
		"--thread", "12",
		"--allow-delete",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("args mismatch:\nwant %#v\ngot  %#v", expected, args)
	}
}

func TestBuildFileProxyArgsIncludesPeriodicConfigWhenPrepared(t *testing.T) {
	args := buildFileProxyArgs(
		"/tmp/root",
		4,
		false,
		&PeriodicConfig{
			PeriodicPort: "tcp://periodic:5000",
			RSAMode:      "AES",
			ClientName:   "client",
			ClientToken:  "token",
			FuncPrefix:   "prefix_",
		},
		&CertificatePaths{
			ServerPublicPath:  "/certs/server.pem",
			ClientPrivatePath: "/certs/client.pem",
		},
	)
	expected := []string{
		"--root", "/tmp/root",
		"--thread", "4",
		"--host", "tcp://periodic:5000",
		"--rsa-private-path", "/certs/client.pem",
		"--rsa-public-path", "/certs/server.pem",
		"--rsa-mode", "AES",
		"--client-name", "client",
		"--client-token", "token",
		"--prefix", "prefix_",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("args mismatch:\nwant %#v\ngot  %#v", expected, args)
	}
}

func TestBuildFileProxyWebArgsIncludesPeriodicConfig(t *testing.T) {
	args := buildFileProxyWebArgs(
		9123,
		&PeriodicConfig{
			PeriodicPort: "tcp://periodic:5000",
			RSAMode:      "AES",
			ClientName:   "client",
			ClientToken:  "token",
			FuncPrefix:   "prefix_",
		},
		&CertificatePaths{
			ServerPublicPath:  "/certs/server.pem",
			ClientPrivatePath: "/certs/client.pem",
		},
	)
	expected := []string{
		"--host", "127.0.0.1",
		"--port", "9123",
		"--worker-host", "tcp://periodic:5000",
		"--rsa-private-path", "/certs/client.pem",
		"--rsa-public-path", "/certs/server.pem",
		"--rsa-mode", "AES",
		"--client-name", "client",
		"--client-token", "token",
		"--prefix", "prefix_",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("args mismatch:\nwant %#v\ngot  %#v", expected, args)
	}
}

func TestBuildFileProxyWebArgsAllowsMissingPeriodicConfig(t *testing.T) {
	args := buildFileProxyWebArgs(8080, nil, nil)
	expected := []string{"--host", "127.0.0.1", "--port", "8080"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("args mismatch:\nwant %#v\ngot %#v", expected, args)
	}
}

func TestNormalizeStartOptionsBoundsThread(t *testing.T) {
	if got := normalizeStartOptions(StartOptions{}).Thread; got != defaultThread {
		t.Fatalf("default thread mismatch: %d", got)
	}
	if got := normalizeStartOptions(StartOptions{Thread: maxThread + 100}).Thread; got != maxThread {
		t.Fatalf("max thread mismatch: %d", got)
	}
	if got := normalizeStartOptions(StartOptions{Thread: 2}).Thread; got != 2 {
		t.Fatalf("expected explicit thread to be preserved, got %d", got)
	}
}

func TestNormalizeStartOptionsDefaultsAndBoundsPort(t *testing.T) {
	defaults := defaultStartOptions()
	if defaults.Port != defaultWebPort || !defaults.AutoOpenBrowser {
		t.Fatalf("unexpected new-app defaults: %#v", defaults)
	}
	if got := normalizeStartOptions(StartOptions{}).AutoOpenBrowser; got {
		t.Fatal("expected explicit false auto-open setting to be preserved")
	}
	if got := normalizeStartOptions(StartOptions{Port: maxWebPort + 1}).Port; got != defaultWebPort {
		t.Fatalf("invalid port should use default, got %d", got)
	}
	if got := normalizeStartOptions(StartOptions{Port: 9123}).Port; got != 9123 {
		t.Fatalf("expected explicit port to be preserved, got %d", got)
	}
}

func TestSettingsPersistRootDirectoryAndStartOptions(t *testing.T) {
	configRoot := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configRoot)
		t.Setenv("LocalAppData", configRoot)
	} else {
		t.Setenv("HOME", configRoot)
		t.Setenv("XDG_CONFIG_HOME", configRoot)
		t.Setenv("XDG_CACHE_HOME", filepath.Join(configRoot, "cache"))
	}

	app := NewApp()
	app.mu.Lock()
	app.token = "saved-token"
	app.userName = "saved-user"
	app.userInfo = UserInfo{
		Name:      "saved-user",
		NickName:  "Saved Nick",
		AvatarURL: "https://example.test/avatar.png",
	}
	app.apiBaseURL = "https://api.example.test"
	app.rootDir = "/tmp/file-proxy-root"
	app.startOptions = StartOptions{Thread: 32, AllowDelete: true, Port: 9123, AutoOpenBrowser: false}
	if err := app.saveSettingsLocked(); err != nil {
		app.mu.Unlock()
		t.Fatalf("saveSettingsLocked returned error: %v", err)
	}
	app.mu.Unlock()

	next := NewApp()
	if err := next.loadSettings(); err != nil {
		t.Fatalf("loadSettings returned error: %v", err)
	}
	status := next.Status()
	if status.APIBaseURL != "https://api.example.test" {
		t.Fatalf("api base url mismatch: %q", status.APIBaseURL)
	}
	if status.RootDir != "/tmp/file-proxy-root" {
		t.Fatalf("root dir mismatch: %q", status.RootDir)
	}
	if status.StartOptions.Thread != maxThread {
		t.Fatalf("thread mismatch: %d", status.StartOptions.Thread)
	}
	if !status.StartOptions.AllowDelete {
		t.Fatal("expected allow delete to be restored")
	}
	if status.StartOptions.Port != 9123 || status.StartOptions.AutoOpenBrowser {
		t.Fatalf("web start options mismatch: %#v", status.StartOptions)
	}
	if !status.LoggedIn {
		t.Fatal("expected login state to be restored")
	}
	if status.UserName != "saved-user" {
		t.Fatalf("user name mismatch: %q", status.UserName)
	}
	if status.UserInfo.NickName != "Saved Nick" {
		t.Fatalf("nick name mismatch: %q", status.UserInfo.NickName)
	}
	if status.UserInfo.AvatarURL != "https://example.test/avatar.png" {
		t.Fatalf("avatar url mismatch: %q", status.UserInfo.AvatarURL)
	}
}

func TestSetAPIBaseURLPersistsAndClearsLoginState(t *testing.T) {
	configRoot := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configRoot)
		t.Setenv("LocalAppData", configRoot)
	} else {
		t.Setenv("HOME", configRoot)
		t.Setenv("XDG_CONFIG_HOME", configRoot)
		t.Setenv("XDG_CACHE_HOME", filepath.Join(configRoot, "cache"))
	}

	app := NewApp()
	app.mu.Lock()
	app.token = "saved-token"
	app.userName = "saved-user"
	app.userInfo = UserInfo{Name: "saved-user", NickName: "Saved Nick"}
	app.config = &PeriodicConfig{ClientName: "client"}
	app.certificate = &CertificatePaths{ServerPublicPath: "server.pem", ClientPrivatePath: "client.pem"}
	app.mu.Unlock()

	status, err := app.SetAPIBaseURL("api.example.test/")
	if err != nil {
		t.Fatalf("SetAPIBaseURL returned error: %v", err)
	}
	if status.APIBaseURL != "https://api.example.test" {
		t.Fatalf("api base url mismatch: %q", status.APIBaseURL)
	}
	if status.LoggedIn {
		t.Fatal("expected changing domain to clear login state")
	}
	if status.Config != nil || status.Certificate != nil {
		t.Fatalf("expected changing domain to clear periodic assets: %#v %#v", status.Config, status.Certificate)
	}

	next := NewApp()
	if err := next.loadSettings(); err != nil {
		t.Fatalf("loadSettings returned error: %v", err)
	}
	if got := next.Status().APIBaseURL; got != "https://api.example.test" {
		t.Fatalf("saved api base url mismatch: %q", got)
	}
}

func TestNormalizeAPIBaseURL(t *testing.T) {
	cases := map[string]string{
		"iot.huabot.com":             "https://iot.huabot.com",
		"https://iot.huabot.com/":    "https://iot.huabot.com",
		"http://localhost:8080/api/": "http://localhost:8080",
	}
	for input, expected := range cases {
		actual, err := normalizeAPIBaseURL(input)
		if err != nil {
			t.Fatalf("normalizeAPIBaseURL(%q) returned error: %v", input, err)
		}
		if actual != expected {
			t.Fatalf("normalizeAPIBaseURL(%q): want %q, got %q", input, expected, actual)
		}
	}
	if _, err := normalizeAPIBaseURL("ftp://example.test"); err == nil {
		t.Fatal("expected invalid scheme error")
	}
}

func TestLogoutClearsSavedLoginState(t *testing.T) {
	configRoot := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configRoot)
		t.Setenv("LocalAppData", configRoot)
	} else {
		t.Setenv("HOME", configRoot)
		t.Setenv("XDG_CONFIG_HOME", configRoot)
		t.Setenv("XDG_CACHE_HOME", filepath.Join(configRoot, "cache"))
	}

	app := NewApp()
	app.mu.Lock()
	app.token = "saved-token"
	app.userName = "saved-user"
	app.userInfo = UserInfo{Name: "saved-user", NickName: "Saved Nick", AvatarURL: "https://example.test/avatar.png"}
	app.rootDir = "/tmp/file-proxy-root"
	app.startOptions = StartOptions{Thread: 16, AllowDelete: true, Port: 8090, AutoOpenBrowser: false}
	if err := app.saveSettingsLocked(); err != nil {
		app.mu.Unlock()
		t.Fatalf("saveSettingsLocked returned error: %v", err)
	}
	app.mu.Unlock()

	if _, err := app.Logout(); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}

	next := NewApp()
	if err := next.loadSettings(); err != nil {
		t.Fatalf("loadSettings returned error: %v", err)
	}
	status := next.Status()
	if status.LoggedIn {
		t.Fatal("expected saved login state to be cleared")
	}
	if status.UserName != "" {
		t.Fatalf("expected user name to be cleared, got %q", status.UserName)
	}
	if status.UserInfo != (UserInfo{}) {
		t.Fatalf("expected user info to be cleared, got %#v", status.UserInfo)
	}
	if status.RootDir != "/tmp/file-proxy-root" {
		t.Fatalf("expected root dir to be preserved, got %q", status.RootDir)
	}
	if status.StartOptions.Thread != 16 || !status.StartOptions.AllowDelete || status.StartOptions.Port != 8090 || status.StartOptions.AutoOpenBrowser {
		t.Fatalf("expected start options to be preserved, got %#v", status.StartOptions)
	}
}

func TestLogoutStopsRunningWorker(t *testing.T) {
	configRoot := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configRoot)
		t.Setenv("LocalAppData", configRoot)
	} else {
		t.Setenv("HOME", configRoot)
		t.Setenv("XDG_CONFIG_HOME", configRoot)
		t.Setenv("XDG_CACHE_HOME", filepath.Join(configRoot, "cache"))
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	app := NewApp()
	app.mu.Lock()
	app.cmd = cmd
	app.token = "saved-token"
	app.userName = "saved-user"
	app.mu.Unlock()
	go app.waitProcess("file-proxy", cmd)

	if _, err := app.Logout(); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !app.Status().Running {
			status := app.Status()
			if status.LoggedIn {
				t.Fatal("expected logout to clear login state")
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("expected logout to stop running worker")
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestExtractUserInfoUsesProfileFields(t *testing.T) {
	info := extractUserInfo("login-name", map[string]any{
		"name": "api-name",
		"profile": map[string]any{
			"nick_name":  "Nick",
			"avatar_url": "https://example.test/avatar.png",
		},
	})

	if info.Name != "api-name" {
		t.Fatalf("name mismatch: %q", info.Name)
	}
	if info.NickName != "Nick" {
		t.Fatalf("nick name mismatch: %q", info.NickName)
	}
	if info.AvatarURL != "https://example.test/avatar.png" {
		t.Fatalf("avatar url mismatch: %q", info.AvatarURL)
	}
}
