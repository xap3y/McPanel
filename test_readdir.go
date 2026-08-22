package main
import (
	"fmt"
	"os"
)
func main() {
	_, err := os.ReadDir("/Users/sky/Documents/mcs/folia/1.21.11")
	fmt.Println("Error:", err)
}
