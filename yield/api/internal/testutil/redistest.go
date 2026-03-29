package testutil

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type TestRedis struct {
	Addr string
	cmd  *exec.Cmd
	t    testing.TB
}

func NewTestRedis(t testing.TB) *TestRedis {
	t.Helper()

	redisBin := findRedisBin(t)
	port := freeRedisPort(t)

	cmd := exec.Command(filepath.Join(redisBin, "redis-server"),
		"--port", fmt.Sprintf("%d", port),
		"--save", "",
		"--appendonly", "no",
		"--daemonize", "no",
		"--loglevel", "warning",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("redis-server start: %v", err)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	r := &TestRedis{
		Addr: addr,
		cmd:  cmd,
		t:    t,
	}

	if err := r.waitReady(); err != nil {
		r.Cleanup()
		t.Fatalf("redis not ready: %v", err)
	}

	return r
}

func (r *TestRedis) Cleanup() {
	if r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Kill()
		r.cmd.Wait()
	}
}

func (r *TestRedis) waitReady() error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", r.Addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("redis not ready after 5s on %s", r.Addr)
}

func findRedisBin(t testing.TB) string {
	t.Helper()

	if v := os.Getenv("REDIS_BIN"); v != "" {
		return v
	}

	searchPaths := []string{
		"/opt/homebrew/opt/redis/bin",
		"/usr/local/opt/redis/bin",
		"/usr/bin",
	}

	nixProfile := os.Getenv("HOME") + "/.nix-profile/bin"
	if _, err := os.Stat(filepath.Join(nixProfile, "redis-server")); err == nil {
		return nixProfile
	}

	for _, p := range searchPaths {
		if _, err := os.Stat(filepath.Join(p, "redis-server")); err == nil {
			return p
		}
	}

	if path, err := exec.LookPath("redis-server"); err == nil {
		return filepath.Dir(path)
	}

	t.Skip("redis-server not found — set REDIS_BIN or install redis")
	return ""
}

func freeRedisPort(t testing.TB) int {
	t.Helper()
	for i := 0; i < 10; i++ {
		port := 16379 + rand.Intn(10000)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	t.Fatal("could not find free port for redis")
	return 0
}
