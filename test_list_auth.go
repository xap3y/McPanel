package main
import (
	"fmt"
	"net/http"
	"net/url"
	"io"
	"strings"
)
func main() {
	resp, err := http.PostForm("http://127.0.0.1:8090/login", url.Values{"username": {"admin"}, "password": {"145060Wer#"}})
	if err != nil { panic(err) }
	cookie := resp.Header.Get("Set-Cookie")
	if cookie != "" { cookie = strings.Split(cookie, ";")[0] }
	resp.Body.Close()
	req, _ := http.NewRequest("GET", "http://127.0.0.1:8090/api/servers/folia%2F1.21.11/files?path=", nil)
	req.Header.Set("Cookie", cookie)
	client := &http.Client{}
	resp2, err := client.Do(req)
	if err != nil { panic(err) }
	defer resp2.Body.Close()
	b, _ := io.ReadAll(resp2.Body)
	fmt.Println("Status:", resp2.StatusCode)
	fmt.Println("Body:", string(b))
}
