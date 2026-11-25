//go:build darwin

package netease

import (
	"os"
	"testing"
)

func TestPreferSessionFileForAdHocBinary(t *testing.T) {
	if !preferSessionFile() {
		t.Fatal("expected ad-hoc signed test binary to prefer private session file")
	}
	if !useSessionFile() {
		t.Fatal("expected useSessionFile to be true for ad-hoc signed test binary")
	}
}

func TestPathHasCertificateSignature(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if pathHasCertificateSignature(exe) {
		t.Errorf("ad-hoc signed test binary %s reported certificate signature", exe)
	}
	if !pathHasCertificateSignature("/usr/bin/true") {
		t.Error("Apple-signed /usr/bin/true reported no certificate signature")
	}
}
