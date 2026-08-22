package main
import (
	"fmt"
	"net/http"
	"io"
)
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/servers/{name}/files", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Matched! %s", r.PathValue("name"))
	})
	go http.ListenAndServe(":9099", mux)
	
	resp, err := http.Get("http://127.0.0.1:9099/api/servers/folia%2F1.21.11/files")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Println("Status:", resp.StatusCode)
	fmt.Println("Body:", string(b))
}
