package main
import (
	"fmt"
	"net/http"
)
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/servers/{name}/files", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Matched: %s\n", r.PathValue("name"))
		w.Write([]byte("Matched!"))
	})
	// Catch-all
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Catch-all: %s\n", r.URL.Path)
		w.WriteHeader(404)
	})
	go http.ListenAndServe(":9099", mux)
	
	req, _ := http.NewRequest("GET", "http://127.0.0.1:9099/api/servers/folia%2F1.21.11/files", nil)
	http.DefaultClient.Do(req)
}
