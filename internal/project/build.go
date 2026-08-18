package project

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
)

type BuildStatus string

const (
	StatusRunning  BuildStatus = "RUNNING"
	StatusSuccess  BuildStatus = "SUCCESS"
	StatusFailed   BuildStatus = "FAILED"
	StatusCanceled BuildStatus = "CANCELED"
)

type Build struct {
	mu        sync.Mutex
	lines     []string
	listeners map[chan string]struct{}
	cmd       *exec.Cmd
	Status    BuildStatus
	done      chan struct{}
}

func newBuild() *Build {
	return &Build{listeners: make(map[chan string]struct{}), Status: StatusRunning, done: make(chan struct{})}
}

func (b *Build) Done() <-chan struct{} { return b.done }

func (b *Build) push(line string) {
	b.mu.Lock()
	b.lines = append(b.lines, line)
	if len(b.lines) > 2000 {
		b.lines = b.lines[len(b.lines)-2000:]
	}
	listeners := make([]chan string, 0, len(b.listeners))
	for c := range b.listeners {
		listeners = append(listeners, c)
	}
	b.mu.Unlock()
	for _, c := range listeners {
		select {
		case c <- line:
		default:
		}
	}
}

func (b *Build) Snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}

func (b *Build) Subscribe() chan string {
	c := make(chan string, 256)
	b.mu.Lock()
	b.listeners[c] = struct{}{}
	b.mu.Unlock()
	return c
}

func (b *Build) Unsubscribe(c chan string) {
	b.mu.Lock()
	delete(b.listeners, c)
	b.mu.Unlock()
	close(c)
}

type Runner struct {
	db     *sql.DB
	mu     sync.Mutex
	active map[string]*Build // one build per project at a time
}

func NewRunner(db *sql.DB) *Runner { return &Runner{db: db, active: make(map[string]*Build)} }

func (r *Runner) LiveBuild(projectName string) (*Build, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.active[projectName]
	return b, ok
}

func (r *Runner) Start(projectDir, projectName string, profile BuildProfile) (*Build, error) {
	r.mu.Lock()
	if _, ok := r.active[projectName]; ok {
		r.mu.Unlock()
		return nil, fmt.Errorf("a build is already running for %q", projectName)
	}
	b := newBuild()
	r.active[projectName] = b
	r.mu.Unlock()

	res, err := r.db.Exec(`INSERT INTO build_history (project_name, profile_name, status) VALUES (?, ?, ?)`,
		projectName, profile.ProfileName, StatusRunning)
	var buildID int64
	if err == nil {
		buildID, _ = res.LastInsertId()
	}

	exe := profile.Executable
	if filepath.Base(exe) != exe && !filepath.IsAbs(exe) {
		// If it's a relative path like "./gradlew" or "bin/build", resolve it against projectDir
		exe = filepath.Join(projectDir, exe)
	}

	cmd := exec.Command(exe, profile.Args...) // argv slice - never a shell string
	cmd.Dir = projectDir

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	b.mu.Lock()
	b.cmd = cmd
	b.mu.Unlock()

	if err := cmd.Start(); err != nil {
		r.finish(projectName, buildID, StatusFailed, -1, "")
		r.mu.Lock()
		delete(r.active, projectName)
		r.mu.Unlock()
		close(b.done)
		return nil, err
	}

	go streamPipe(stdout, b.push)
	go streamPipe(stderr, b.push)

	go func() {
		waitErr := cmd.Wait()
		status, exitCode := StatusSuccess, 0
		if waitErr != nil {
			status = StatusFailed
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		artifact, _ := DetectArtifact(projectDir, "*.jar")
		r.finish(projectName, buildID, status, exitCode, artifact)

		b.mu.Lock()
		b.Status = status
		b.mu.Unlock()
		close(b.done)

		r.mu.Lock()
		delete(r.active, projectName)
		r.mu.Unlock()
	}()

	return b, nil
}

func (r *Runner) Cancel(projectName string) error {
	r.mu.Lock()
	b, ok := r.active[projectName]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("no build running for %q", projectName)
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Kill()
	}
	return nil
}

func (r *Runner) finish(projectName string, buildID int64, status BuildStatus, exitCode int, artifact string) {
	if buildID == 0 {
		return
	}
	r.db.Exec(`UPDATE build_history SET status=?, finished_at=NOW(), exit_code=?, artifact_path=? WHERE id=?`,
		status, exitCode, artifact, buildID)
}

func streamPipe(r io.Reader, push func(string)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		push(scanner.Text())
	}
}