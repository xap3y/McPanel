package main
import (
	"fmt"
	"net/url"
	"text/template"
)
func main() {
	fm := template.FuncMap{"pathEscape": url.PathEscape}
	t, err := template.New("").Funcs(fm).Parse(`{{pathEscape "a/b"}}`)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	t.Execute(nil, nil)
}
