package main
import (
	"fmt"
	"net/http"
	"io"
)
func main() {
	resp, err := http.Get("http://127.0.0.1:8090/api/servers/folia%2F1.21.11/files")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Println("Status:", resp.StatusCode)
	fmt.Println("Body:", string(b))
}
