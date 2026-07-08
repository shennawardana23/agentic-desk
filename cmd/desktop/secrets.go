package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// persistedEnvPath is where a real end user's double-clicked packaged
// app can find GEMINI_API_KEY without ever opening a terminal. Finder
// launches a process with none of the invoking shell's env vars, so
// os.Environ() alone (what startCore passed to the child in
// corelauncher.go before this file existed) is always empty for a
// double-clicked .app — that's the exact bug this fixes. `make
// configure-key KEY=...` (see Makefile) writes this file for the user.
func persistedEnvPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agentic-desk", ".env"), nil
}

// loadPersistedEnv reads persistedEnvPath as simple KEY=VALUE lines (one
// per line, blank lines and lines starting with '#' ignored). The file
// is optional — a missing file returns an empty map and no error, since
// a dev running `go run ./cmd/desktop` with GEMINI_API_KEY already
// exported has no need for it.
func loadPersistedEnv() map[string]string {
	values := map[string]string{}

	path, err := persistedEnvPath()
	if err != nil {
		return values
	}
	f, err := os.Open(path)
	if err != nil {
		return values
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}

// mergeMissingEnv returns base with any key from extra appended as
// "KEY=VALUE", but only for keys base doesn't already set — a real,
// already-exported env var always wins over the persisted file. This is
// what keeps `make run`/an exported GEMINI_API_KEY for local debugging
// completely unaffected by this fallback.
func mergeMissingEnv(base []string, extra map[string]string) []string {
	set := make(map[string]bool, len(base))
	for _, kv := range base {
		if k, _, ok := strings.Cut(kv, "="); ok {
			set[k] = true
		}
	}
	merged := base
	for k, v := range extra {
		if !set[k] {
			merged = append(merged, k+"="+v)
		}
	}
	return merged
}
