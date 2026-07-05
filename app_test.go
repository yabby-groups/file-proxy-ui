package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
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
