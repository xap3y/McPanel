package main
import (
	"fmt"
	"mcpanel/internal/fsutil"
	"os"
)
func main() {
	err := os.MkdirAll("/tmp/mcs/folia/1.21.11", 0755)
	if err != nil { panic(err) }
	
	target, err := fsutil.SafeJoin("/tmp/mcs/folia/1.21.11", "")
	fmt.Println("Result:", target, "Error:", err)
}
