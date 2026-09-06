// internal/site/script_test.go

package site

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptVerifiesBeforeInstalling(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "matching download", true: "corrupt download"}[corrupt], func(t *testing.T) {
			router, _ := serving(t, "ELFBYTES")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if corrupt && r.URL.Path == "/s99deploy" {
					_, _ = io.WriteString(w, "TRUNCATED")
					return
				}
				router.ServeHTTP(w, r)
			}))
			defer server.Close()
			response, err := http.Get(server.URL + "/install.sh")
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}

			dir := t.TempDir()
			destination := filepath.Join(dir, "installed")
			stub := "#!/bin/sh\nset -eu\n[ \"$1\" = -m ]\n[ \"$2\" = 0755 ]\n[ \"$4\" = /usr/local/bin/s99deploy ]\ncp \"$3\" \"$S99DEPLOY_TEST_DEST\"\n"
			if err := os.WriteFile(filepath.Join(dir, "install"), []byte(stub), 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("sh")
			cmd.Stdin = strings.NewReader(string(body))
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "S99DEPLOY_TEST_DEST="+destination, "TMPDIR="+dir)
			output, err := cmd.CombinedOutput()
			if corrupt {
				if err == nil || !strings.Contains(string(output), "checksum mismatch") {
					t.Fatalf("error=%v output=%s", err, output)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatalf("corrupt download was installed: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("script failed: %v\n%s", err, output)
				}
				installed, err := os.ReadFile(destination)
				if err != nil || string(installed) != "ELFBYTES" {
					t.Fatalf("installed=%q error=%v", installed, err)
				}
			}
			leftovers, err := filepath.Glob(filepath.Join(dir, "tmp.*"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("temporary downloads survived: %v, %v", leftovers, err)
			}
		})
	}
}
