package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The Docker resources the benchmark uses.
const (
	// benchNetwork connects run containers to PostgreSQL.
	benchNetwork = "goforge-bench"
	// pgContainer serves the orders task's databases: one per run, created empty.
	pgContainer = "goforge-bench-postgres"
	pgImage     = "postgres:18-alpine"
	// imageRepo is the image agents and scoring run in (see Dockerfile).
	imageRepo = "goforge-bench"
)

// docker runs a docker command and returns its trimmed combined output.
func docker(args ...string) (string, error) {
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// ensureNetwork creates the benchmark network if it doesn't exist.
func ensureNetwork() error {
	if _, err := docker("network", "inspect", benchNetwork); err == nil {
		return nil
	}
	_, err := docker("network", "create", benchNetwork)
	return err
}

// ensurePostgres starts the PostgreSQL container on the benchmark network if
// needed, and returns the host address it is published on (for scoring on the
// host, e.g. the references).
func ensurePostgres() (string, error) {
	if err := ensureNetwork(); err != nil {
		return "", err
	}
	if running, err := docker("inspect", "-f", "{{.State.Running}}", pgContainer); err != nil || running != "true" {
		_, _ = docker("rm", "-f", pgContainer)
		if _, err := docker("run", "-d", "--name", pgContainer, "--network", benchNetwork, "-p", "127.0.0.1::5432",
			"-e", "POSTGRES_USER=bench", "-e", "POSTGRES_PASSWORD=bench", pgImage); err != nil {
			return "", err
		}
	}
	// A container started by an earlier version of bench may not be on the network
	networks, err := docker("inspect", "-f", "{{json .NetworkSettings.Networks}}", pgContainer)
	if err != nil {
		return "", err
	}
	if !strings.Contains(networks, `"`+benchNetwork+`"`) {
		if _, err := docker("network", "connect", benchNetwork, pgContainer); err != nil {
			return "", err
		}
	}
	// Probe over TCP: during initialization PostgreSQL runs a socket-only server
	deadline := time.Now().Add(90 * time.Second)
	for exec.Command("docker", "exec", pgContainer, "pg_isready", "-U", "bench", "-h", "127.0.0.1").Run() != nil {
		if time.Now().After(deadline) {
			return "", errors.New("PostgreSQL did not become ready within 90s")
		}
		time.Sleep(500 * time.Millisecond)
	}
	out, err := docker("port", pgContainer, "5432/tcp")
	if err != nil {
		return "", err
	}
	return strings.Split(out, "\n")[0], nil
}

// createDatabase creates an empty database and returns its name and a
// function that drops it.
func createDatabase() (string, func(), error) {
	name := fmt.Sprintf("bench_%d", time.Now().UnixNano())
	if _, err := docker("exec", pgContainer, "psql", "-U", "bench", "-d", "postgres", "-c", "CREATE DATABASE "+name); err != nil {
		return "", func() {}, err
	}
	drop := func() {
		_, _ = docker("exec", pgContainer, "psql", "-U", "bench", "-d", "postgres", "-c", "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	}
	return name, drop, nil
}

// containerDatabaseURL is the URL of a database for a run container.
func containerDatabaseURL(name string) string {
	return fmt.Sprintf("postgres://bench:bench@%s:5432/%s?sslmode=disable", pgContainer, name)
}

// hostDatabaseURL is the URL of a database for a process on the host.
func hostDatabaseURL(hostPort, name string) string {
	return fmt.Sprintf("postgres://bench:bench@%s/%s?sslmode=disable", hostPort, name)
}

// ensureImage builds the benchmark image for the versions if it doesn't exist,
// and returns its tag and ID.
func ensureImage(claudeVersion, goforgeVersion string) (string, string, error) {
	tag := fmt.Sprintf("%s:claude-%s-goforge-%s", imageRepo, claudeVersion, goforgeVersion)
	if _, err := docker("image", "inspect", tag); err != nil {
		fmt.Printf("building the benchmark image %s…\n", tag)
		if _, err := docker("build", "--build-arg", "CLAUDE_CODE_VERSION="+claudeVersion,
			"--build-arg", "GOFORGE_VERSION="+goforgeVersion, "-t", tag, "."); err != nil {
			return "", "", err
		}
	}
	id, err := docker("image", "inspect", "-f", "{{.Id}}", tag)
	return tag, id, err
}

// buildLinuxBench builds bench for Linux into dir, to score inside the image.
func buildLinuxBench(dir string) (string, error) {
	path := filepath.Join(dir, "bench-linux")
	cmd := exec.Command("go", "build", "-o", path, "./cmd/bench")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+runtime.GOARCH)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building bench for Linux: %v: %s", err, out)
	}
	return path, nil
}

// hostUser returns "uid:gid", so files a container writes belong to the user.
func hostUser() string {
	return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
}
