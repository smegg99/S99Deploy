// internal/site/script_test.go

package site

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// A hostile Host reaches the served script as bytes root's shell will expand.
func TestServedScriptQuotesNoHostItWasNotGiven(t *testing.T) {
	// net/http passes $ ( ) and ' through, and the script assigns url="...".
	// The proof is the script itself: it is run, and must create nothing.
	router, _ := serving(t, "ELFBYTES")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: router}
	go server.Serve(listener)
	defer server.Close()

	// A bare name: net/http rejects a Host carrying a slash on its own, and
	// this test is about what the site does with the ones it lets through.
	dir := t.TempDir()
	marker := filepath.Join(dir, "PWNED")
	const host = "x$(touch$IFS'PWNED')y"

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conn,
		"GET /install.sh HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host); err != nil {
		t.Fatal(err)
	}
	answer, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(answer), "400 Bad Request") {
		t.Errorf("status line = %q, want 400", strings.SplitN(string(answer), "\r\n", 2)[0])
	}
	if strings.Contains(string(answer), "$(") {
		t.Errorf("the answer carries the host verbatim:\n%s", answer)
	}

	body := strings.SplitN(string(answer), "\r\n\r\n", 2)[1]
	cmd := exec.Command("sh")
	cmd.Stdin = strings.NewReader(body)
	cmd.Dir = dir
	// The real PATH, so the injected touch would resolve if it ever ran.
	cmd.Env = append(os.Environ(), "TMPDIR="+dir)
	_, _ = cmd.CombinedOutput()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("the served answer executed the host: %v", err)
	}
}
