package main

import (
	"reflect"
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
