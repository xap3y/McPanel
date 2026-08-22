package mcserver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	root         string
	screenPrefix string
	logDir       string

	mu        sync.Mutex
	startedAt map[string]time.Time // server name → time screen session was started by us
}

func NewManager(root, screenPrefix, logDir string) *Manager {
	if abs, err := filepath.Abs(logDir); err == nil {
		logDir = abs
	}
	os.MkdirAll(logDir, 0o755)
	return &Manager{
		root:         root,
		screenPrefix: screenPrefix,
		logDir:       logDir,
		startedAt:    make(map[string]time.Time),
	}
}

func (m *Manager) SessionName(server string) string {
	return m.screenPrefix + strings.ReplaceAll(server, "/", "_")
}

func (m *Manager) LogPath(server string) string {
	return filepath.Join(m.logDir, strings.ReplaceAll(server, "/", "_")+".log")
}

func (m *Manager) screenRCPath(server string) string {
	return filepath.Join(m.logDir, strings.ReplaceAll(server, "/", "_")+".screenrc")
}

// StartedAt returns the time we started the given server (zero if unknown / started outside panel).
func (m *Manager) StartedAt(server string) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startedAt[server]
}

// IsRunning parses `screen -ls` output. screen's exit code is unreliable
// (nonzero when there ARE sessions in some versions), so we parse stdout.
func (m *Manager) IsRunning(server string) (bool, int) {
	out, _ := exec.Command("screen", "-ls").Output()
	name := m.SessionName(server)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// format: "12345.mcpanel-1.21.11\t(Detached)"
		if !strings.Contains(line, "."+name) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		idPart := strings.SplitN(fields[0], ".", 2)
		if len(idPart) != 2 || idPart[1] != name {
			continue
		}
		var pid int
		fmt.Sscanf(idPart[0], "%d", &pid)
		return true, pid
	}
	return false, 0
}

// Start launches the server jar inside a detached screen session with
// logging enabled. serverDir must already be validated by the caller.
func (m *Manager) Start(server, serverDir string, s Settings) error {
	if running, _ := m.IsRunning(server); running {
		return fmt.Errorf("server %q is already running", server)
	}

	// Write a temporary .screenrc for this server to set the logfile and 256 colors
	// Server names may contain a configured group prefix (for example
	// "folia/1.21.11"). Keep runtime files flat, just as LogPath does;
	// otherwise the missing log subdirectory prevents screen from starting.
	rcPath := m.screenRCPath(server)
	if err := os.WriteFile(rcPath, []byte(fmt.Sprintf("logfile %s\nlogfile flush 0\nterm screen-256color\n", m.LogPath(server))), 0o644); err != nil {
		return fmt.Errorf("write screen configuration: %w", err)
	}

	jar, err := ResolveServerJar(serverDir, s.ServerJar)
	if err != nil {
		return err
	}

	args := []string{"-c", rcPath, "-L", "-dmS", m.SessionName(server)}

	wrapperScript := `printf "\rTERM_PROGRAM=$TERM_PROGRAM TERM=$TERM COLORS=$(tput colors)\n"; exec "$@"`
	args = append(args, "sh", "-c", wrapperScript, "sh")

	args = append(args, s.JavaPath)
	args = append(args, "-Xms"+s.Xms, "-Xmx"+s.Xmx)
	args = append(args, s.JVMArgs...)
	args = append(args, "-jar", jar)
	args = append(args, s.ServerArgs...)

	cmd := exec.Command("screen", args...)
	cmd.Dir = serverDir

	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "TERM_PROGRAM=") && !strings.HasPrefix(e, "TMUX=") && !strings.HasPrefix(e, "TMUX_PANE=") {
			env = append(env, e)
		}
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return err
	}
	// screen successfully creating a session does not mean Java stayed alive.
	time.Sleep(300 * time.Millisecond)
	if running, _ := m.IsRunning(server); !running {
		return fmt.Errorf("server exited immediately; check %s", m.LogPath(server))
	}

	m.mu.Lock()
	m.startedAt[server] = time.Now()
	m.mu.Unlock()
	return nil
}

// ResolveServerJar uses the configured JAR when it exists. The historical
// default (server.jar) is also treated as an auto-detect request, because
// Paper and Velocity downloads are normally versioned filenames.
func ResolveServerJar(serverDir, configured string) (string, error) {
	if configured == "" {
		configured = "server.jar"
	}
	path := configured
	if !filepath.IsAbs(path) {
		path = filepath.Join(serverDir, path)
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return configured, nil
	}
	if configured != "server.jar" {
		return "", fmt.Errorf("configured server JAR %q does not exist", configured)
	}

	entries, err := os.ReadDir(serverDir)
	if err != nil {
		return "", err
	}
	var jars []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
			jars = append(jars, entry.Name())
		}
	}
	if len(jars) == 0 {
		return "", fmt.Errorf("no server JAR found in %s", serverDir)
	}
	return jars[0], nil
}

// SendCommand writes to the screen session via `stuff`. The command text
// is passed as a single argv element - never through a shell - so this
// is safe regardless of what the operator types into the console.
func (m *Manager) SendCommand(server, command string) error {
	name := m.SessionName(server)
	cmd := exec.Command("screen", "-S", name, "-p", "0", "-X", "stuff", command+"\r")
	return cmd.Run()
}

// SendCtrlC sends a Ctrl-C (SIGINT) to the foreground process in the screen session.
func (m *Manager) SendCtrlC(server string) error {
	name := m.SessionName(server)
	// screen `stuff` with \003 sends a literal ^C to the pty
	cmd := exec.Command("screen", "-S", name, "-p", "0", "-X", "stuff", "\x03")
	return cmd.Run()
}

// Stop sends the graceful "stop" console command and waits up to
// timeout for the session to disappear, otherwise Kill()s it.
func (m *Manager) Stop(server string, timeout time.Duration) error {
	running, _ := m.IsRunning(server)
	if !running {
		return nil
	}
	if err := m.SendCommand(server, "stop"); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if running, _ := m.IsRunning(server); !running {
			m.mu.Lock()
			delete(m.startedAt, server)
			m.mu.Unlock()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return m.Kill(server)
}

// Kill terminates the whole screen session (and its child process tree).
func (m *Manager) Kill(server string) error {
	name := m.SessionName(server)
	m.mu.Lock()
	delete(m.startedAt, server)
	m.mu.Unlock()
	return exec.Command("screen", "-S", name, "-X", "quit").Run()
}

func (m *Manager) Restart(server, serverDir string, s Settings, timeout time.Duration) error {
	if err := m.Stop(server, timeout); err != nil {
		return err
	}
	return m.Start(server, serverDir, s)
}
