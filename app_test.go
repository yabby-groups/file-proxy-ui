package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

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
	app.startOptions = StartOptions{Thread: 32, AllowDelete: true}
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
	app.startOptions = StartOptions{Thread: 16, AllowDelete: true}
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
	if status.StartOptions.Thread != 16 || !status.StartOptions.AllowDelete {
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
	go app.waitProcess(cmd)

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
