package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// coreBinaryName is what desktop-build (see Makefile) places next to the
// packaged app binary — a prebuilt cmd/core, so a real user's double
// click never needs a terminal or a second binary running by hand.
const coreBinaryName = "agentic-desk-core"

// coreProcess is cmd/core running as a child of the desktop app. A
// single-user desktop app has no reason to make the user coordinate two
// binaries themselves — that's exactly what caused the recurring "Load
// failed" bug documented in SESSION_HANDOFF.md/DEPLOY.md/TESTING.md
// (the GUI silently defaulting to a port nothing, or the wrong process,
// was listening on). The desktop app now owns core's whole lifecycle.
type coreProcess struct {
	cmd  *exec.Cmd
	addr string // "http://127.0.0.1:<port>", set only on success
	err  error
}

// startCore picks a free loopback port, launches cmd/core against it,
// and blocks until the API answers or it gives up. The child's env is
// the parent's own environment (DATABASE_URL/GEMINI_API_KEY come from
// wherever the desktop app itself was started with them — e.g. a dev
// shell) plus any var missing from that environment but present in the
// persisted config file from secrets.go (the double-click/Finder-launch
// case, where no shell env exists at all). A real exported env var
// always wins over the persisted file — see mergeMissingEnv.
func startCore(ctx context.Context) *coreProcess {
	port, err := freePort()
	if err != nil {
		return &coreProcess{err: fmt.Errorf("find free port for core: %w", err)}
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	argv, dir, err := locateCoreCommand()
	if err != nil {
		return &coreProcess{err: err}
	}

	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	env := mergeMissingEnv(os.Environ(), loadPersistedEnv())
	cmd.Env = append(env, "API_ADDR="+addr)
	cmd.Stdout = io.MultiWriter(os.Stdout, &output)
	cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	if err := cmd.Start(); err != nil {
		return &coreProcess{err: fmt.Errorf("start core (%v): %w", argv, err)}
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	if err := waitForReady(ctx, addr, 15*time.Second, exited); err != nil {
		return &coreProcess{cmd: cmd, err: fmt.Errorf("%w\n%s", err, output.String())}
	}
	return &coreProcess{cmd: cmd, addr: "http://" + addr}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// locateCoreCommand finds how to run cmd/core: a prebuilt binary shipped
// next to the desktop executable (packaged .app — see the Makefile's
// desktop-build target), or `go run ./cmd/core` from the repo root as a
// dev-mode fallback (`wails dev`/`go run ./cmd/desktop`, no packaged
// binary exists yet).
func locateCoreCommand() (argv []string, dir string, err error) {
	if exe, exeErr := os.Executable(); exeErr == nil {
		sibling := filepath.Join(filepath.Dir(exe), coreBinaryName)
		if _, statErr := os.Stat(sibling); statErr == nil {
			return []string{sibling}, filepath.Dir(exe), nil
		}
	}

	root, rootErr := findRepoRoot()
	if rootErr != nil {
		return nil, "", fmt.Errorf("locate cmd/core: no packaged %s next to the app, and %w", coreBinaryName, rootErr)
	}
	return []string{"go", "run", "./cmd/core"}, root, nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above starting dir")
		}
		dir = parent
	}
}

// waitForReady polls addr until something answers HTTP (any status
// counts — this only proves the port is listening, not that every
// dependency like Postgres is healthy) or exited fires first, which
// means core crashed on startup (e.g. missing DATABASE_URL) and there's
// no point waiting out the rest of the timeout.
func waitForReady(ctx context.Context, addr string, timeout time.Duration, exited <-chan error) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			if err == nil {
				err = fmt.Errorf("exited immediately with no error")
			}
			return fmt.Errorf("core process exited before it became ready: %w", err)
		case <-time.After(150 * time.Millisecond):
		}

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/profile", nil)
		if reqErr != nil {
			continue
		}
		if resp, doErr := client.Do(req); doErr == nil {
			resp.Body.Close()
			return nil
		}
	}
	return fmt.Errorf("core did not become ready on %s within %s", addr, timeout)
}

// stop kills the child core process, if one was started. Safe to call
// on a nil or never-started coreProcess.
func (c *coreProcess) stop() {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return
	}
	_ = c.cmd.Process.Kill()
}
