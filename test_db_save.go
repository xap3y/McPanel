package main
import (
	"database/sql"
	"fmt"
	"mcpanel/internal/settings"
	_ "github.com/go-sql-driver/mysql"
)
func main() {
	db, err := sql.Open("mysql", "xap3y:145060Wer#@tcp(127.0.0.1:3306)/mcpanel")
	if err != nil { panic(err) }
	
	g := settings.GlobalSettings{Subfolders: []string{"folia", "purpur"}}
	err = settings.SaveGlobal(db, g)
	fmt.Printf("Save error: %v\n", err)
	
	g2 := settings.LoadGlobal(db)
	fmt.Printf("Subfolders: %q\n", g2.Subfolders)
}
