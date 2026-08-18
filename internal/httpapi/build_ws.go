// internal/httpapi/build_ws.go
package httpapi

import "net/http"

func (a *API) BuildWS(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	lb, ok := a.Builder.LiveBuild(name)
	if !ok {
		conn.WriteJSON(map[string]string{"type": "status", "data": "idle"})
		return
	}

	// Notify client that a build is in progress
	conn.WriteJSON(map[string]string{"type": "status", "data": "running"})

	for _, line := range lb.Snapshot() {
		if conn.WriteJSON(map[string]string{"type": "line", "data": line}) != nil {
			return
		}
	}

	sub := lb.Subscribe()
	defer lb.Unsubscribe(sub)

	for {
		select {
		case line := <-sub:
			if conn.WriteJSON(map[string]string{"type": "line", "data": line}) != nil {
				return
			}
		case <-lb.Done():
			conn.WriteJSON(map[string]string{"type": "status", "data": "finished"})
			return
		}
	}
}