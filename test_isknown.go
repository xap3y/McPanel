package main
import (
	"fmt"
	"mcpanel/internal/mcserver"
)
func main() {
	known := mcserver.IsKnownServer("/Users/sky/Documents/mcs", "folia/1.21.11", []string{"folia", "purpur"})
	fmt.Println("IsKnownServer:", known)
}
