package main

import (
	"html/template"
	"os"
)

func main() {
	t := template.Must(template.New("test").Parse("<script>window.NAME = {{.Name}};</script>"))
	t.Execute(os.Stdout, map[string]string{"Name": "MyProject"})
}
