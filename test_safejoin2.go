package main
import (
	"fmt"
	"mcpanel/internal/fsutil"
)
func main() {
	target, err := fsutil.SafeJoin("/Users/sky/Documents/mcs/folia/1.21.11", "")
	fmt.Println("Result:", target, "Error:", err)
}
