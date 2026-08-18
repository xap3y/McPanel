package main

import (
	"fmt"
	"os/exec"
)

func main() {
	cmd := exec.Command("./gradlew", "clean")
	cmd.Dir = "/Users/sky/Documents/mcpanel"
	err := cmd.Start()
	fmt.Println("Error:", err)
}
