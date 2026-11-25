package netease

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

// fakeKeychain is a keychain for the tests, or none at all (missing), as a
// Linux desktop without a Secret Service.
type fakeKeychain struct {
	missing bool
	secrets map[string][]byte
}

var errNoStore = fmt.Errorf("no Secret Service: %w", errors.ErrUnsupported)

func (k *fakeKeychain) Get(key string) ([]byte, error) {
	if k.missing {
		return nil, errNoStore
	}
	b, ok := k.secrets[key]
	if !ok {
		return nil, mygo.ErrSecretNotFound
	}
	return b, nil
}

func (k *fakeKeychain) Set(key string, value []byte) error {
	if k.missing {
		return errNoStore
	}
	k.secrets[key] = value
	return nil
}

func (k *fakeKeychain) Delete(key string) error {
	if k.missing {
		return errNoStore
	}
	delete(k.secrets, key)
	return nil
}

func useFakeKeychain(t *testing.T, k *fakeKeychain) string {
	t.Helper()
	dir := t.TempDir()
	oldUse, oldSecrets, oldDir := useSessionFile, sessionSecrets, userDataDir
	useSessionFile = func() bool { return false }
	sessionSecrets = k
	userDataDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { useSessionFile, sessionSecrets, userDataDir = oldUse, oldSecrets, oldDir })
	return filepath.Join(dir, "session")
}

// Without a keychain, the session is kept in the private file, and read
// back from it at the next launch, instead of being lost.
func TestSessionWithoutKeychainKeepsToFile(t *testing.T) {
	path := useFakeKeychain(t, &fakeKeychain{missing: true})
	if err := writeSession([]byte("MUSIC_U=x")); err != nil {
		t.Fatalf("writeSession: %v", err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "MUSIC_U=x" {
		t.Fatalf("session file = %q, %v", b, err)
	}
	if b, err := readSession(); err != nil || string(b) != "MUSIC_U=x" {
		t.Fatalf("readSession = %q, %v", b, err)
	}
	deleteSession()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file left after sign-out: %v", err)
	}
}

// With a keychain, the session goes there and no file is written, nor read.
func TestSessionWithKeychainKeepsOutOfFile(t *testing.T) {
	k := &fakeKeychain{secrets: map[string][]byte{}}
	path := useFakeKeychain(t, k)
	if err := os.WriteFile(path, []byte("MUSIC_U=stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := readSession(); !errors.Is(err, mygo.ErrSecretNotFound) {
		t.Fatalf("readSession with an empty keychain = %q, %v; want ErrSecretNotFound", b, err)
	}
	if err := writeSession([]byte("MUSIC_U=y")); err != nil {
		t.Fatalf("writeSession: %v", err)
	}
	if string(k.secrets[sessionSecret]) != "MUSIC_U=y" {
		t.Fatalf("keychain holds %q", k.secrets[sessionSecret])
	}
	if b, _ := os.ReadFile(path); string(b) != "MUSIC_U=stale" {
		t.Fatalf("session file was touched: %q", b)
	}
	deleteSession()
	if _, ok := k.secrets[sessionSecret]; ok {
		t.Fatal("session left in the keychain after sign-out")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("sign-out with a keychain removed the file: %v", err)
	}
}
