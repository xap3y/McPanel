package mcserver

import (
	"bufio"
	"os"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

const ringSize = 1000

type Hub struct {
	mu        sync.Mutex
	ring      []string
	pos       int
	full      bool
	listeners map[chan string]struct{}
}

func newHub() *Hub {
	return &Hub{ring: make([]string, ringSize), listeners: make(map[chan string]struct{})}
}

func (h *Hub) push(line string) {
	h.mu.Lock()
	h.ring[h.pos] = line
	h.pos = (h.pos + 1) % ringSize
	if h.pos == 0 {
		h.full = true
	}
	listeners := make([]chan string, 0, len(h.listeners))
	for c := range h.listeners {
		listeners = append(listeners, c)
	}
	h.mu.Unlock()

	for _, c := range listeners {
		select {
		case c <- line:
		default: // slow client, drop line rather than block the tailer
		}
	}
}

func (h *Hub) Snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.full {
		out := make([]string, h.pos)
		copy(out, h.ring[:h.pos])
		return out
	}
	out := make([]string, ringSize)
	copy(out, h.ring[h.pos:])
	copy(out[ringSize-h.pos:], h.ring[:h.pos])
	return out
}

func (h *Hub) Subscribe() chan string {
	c := make(chan string, 256)
	h.mu.Lock()
	h.listeners[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *Hub) Unsubscribe(c chan string) {
	h.mu.Lock()
	delete(h.listeners, c)
	h.mu.Unlock()
	close(c)
}

// ConsoleManager owns one Hub + one fsnotify watcher per server that
// has an open console (lazily started, event-driven - no polling).
type ConsoleManager struct {
	mu   sync.Mutex
	hubs map[string]*Hub
	m    *Manager
}

func NewConsoleManager(m *Manager) *ConsoleManager {
	return &ConsoleManager{hubs: make(map[string]*Hub), m: m}
}

func (cm *ConsoleManager) HubFor(server string) *Hub {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if h, ok := cm.hubs[server]; ok {
		return h
	}
	h := newHub()
	cm.hubs[server] = h
	go cm.tail(server, h)
	return h
}

func (cm *ConsoleManager) tail(server string, h *Hub) {
	path := cm.m.LogPath(server)

	f, err := os.Open(path)
	if err != nil {
		// log file doesn't exist until the server has started at least
		// once; wait for the watcher on the directory to pick it up.
		f = nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()
	watcher.Add(cm.m.logDir)

	var reader *bufio.Reader
	if f != nil {
		stat, err := f.Stat()
		if err == nil {
			size := stat.Size()
			const maxTail = 64 * 1024
			if size > maxTail {
				f.Seek(size-maxTail, 0)
				// Read until the next newline to ensure we start at a complete line
				buf := make([]byte, 1)
				for {
					if n, err := f.Read(buf); err != nil || n == 0 || buf[0] == '\n' {
						break
					}
				}
			}
		}
		reader = bufio.NewReader(f)
	}

	var partialLine string
	readAvailable := func() {
		if reader == nil {
			return
		}
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				partialLine += line
				break
			}
			fullLine := partialLine + line
			partialLine = ""

			if clean, ok := normalizeConsoleLine(fullLine); ok {
				h.push(clean)
			}
		}
	}

	if reader != nil {
		readAvailable()
	}

	for event := range watcher.Events {
		if event.Name != path {
			continue
		}
		if reader == nil {
			// file just got created
			if nf, err := os.Open(path); err == nil {
				f = nf
				reader = bufio.NewReader(f)
			}
		}
		readAvailable()
	}
}

// normalizeConsoleLine performs the small amount of terminal emulation needed
// for screen logs. Content after a carriage return overwrites the prompt;
// cursor-control sequences are discarded, while SGR colors survive.
func normalizeConsoleLine(line string) (string, bool) {
	line = strings.TrimRight(line, "\r\n")
	if i := strings.LastIndexByte(line, '\r'); i >= 0 {
		line = line[i+1:]
	}
	if strings.ContainsRune(line, '\b') {
		return "", false
	}
	line = keepSGR(line)
	plain := strings.TrimSpace(removeSGR(line))
	if plain == "" || plain == ">" || strings.HasPrefix(plain, "> ") {
		return "", false
	}
	return line, true
}

// keepSGR removes cursor/mode sequences but preserves ANSI Select Graphic
// Rendition (...m), including 256-color and 24-bit color parameters.
func keepSGR(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] != 0x1b {
			if s[i] >= 0x20 || s[i] == '\t' {
				out = append(out, s[i])
			}
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				if s[j] == 'm' {
					out = append(out, s[i:j+1]...)
				}
				i = j + 1
				continue
			}
		}
		if i+1 < len(s) {
			i += 2
		} else {
			i++
		}
	}
	return string(out)
}

func removeSGR(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}

func stripANSI(s string) string {
	return keepSGR(s)
}
