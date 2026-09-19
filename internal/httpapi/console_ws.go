package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"mcpanel/internal/mcserver"
)

var upgrader = websocket.Upgrader{
	// Same-origin only: reject cross-site WebSocket handshakes.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		host := r.Host
		return origin == "http://"+host || origin == "https://"+host
	},
}

type wsMsg struct {
	Type string `json:"type"` // "command"
	Data string `json:"data"`
}

func (a *API) ConsoleWS(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name, a.ServerSubfolders()) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	hub := a.Console.HubFor(name)
	history := hub.Snapshot()
	// A running server starts its console with the most recent 100 lines.
	// The normal (stopped) console history remains 200 lines.
	if running, _ := a.Screen.IsRunning(name); running && len(history) > 100 {
		history = history[len(history)-100:]
	}
	for _, line := range history {
		if conn.WriteJSON(map[string]string{"type": "line", "data": line}) != nil {
			return
		}
	}

	sub := hub.Subscribe()
	defer hub.Unsubscribe(sub)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m wsMsg
			if json.Unmarshal(raw, &m) != nil {
				continue
			}
			if m.Type == "command" && m.Data != "" {
				a.Screen.SendCommand(name, m.Data)
			}
			if m.Type == "ctrlc" {
				a.Screen.SendCtrlC(name)
			}
		}
	}()

	for {
		select {
		case line, ok := <-sub:
			if !ok {
				return
			}
			if conn.WriteJSON(map[string]string{"type": "line", "data": line}) != nil {
				return
			}
		case <-done:
			return
		}
	}
}
