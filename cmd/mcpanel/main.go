package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/term"

	"mcpanel/internal/auth"
	"mcpanel/internal/config"
	"mcpanel/internal/db"
	"mcpanel/internal/httpapi"
	"mcpanel/internal/mcserver"
	"mcpanel/internal/project"
)

func main() {
	cfg, err := config.Load("configs/mcpanel.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if len(os.Args) > 1 && os.Args[1] == "createadmin" {
		createAdmin(cfg)
		return
	}

	runServer(cfg)
}

func createAdmin(cfg *config.Config) {
	dbconn, err := db.Open(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer dbconn.Close()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)
	if username == "" {
		log.Fatal("username cannot be empty")
	}

	fmt.Print("password: ")
	pwBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		log.Fatalf("reading password: %v", err)
	}
	if len(pwBytes) < 8 {
		log.Fatal("password should be at least 8 characters")
	}

	hash, err := auth.HashPassword(string(pwBytes))
	if err != nil {
		log.Fatalf("hashing: %v", err)
	}

	_, err = dbconn.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, hash)
	if err != nil {
		log.Fatalf("insert: %v", err)
	}

	fmt.Printf("admin user %q created\n", username)
}

func runServer(cfg *config.Config) {
	dbconn, err := db.Open(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer dbconn.Close()

	authStore := auth.NewStore(dbconn)
	limiter := auth.NewLimiter()

	screenMgr := mcserver.NewManager(cfg.Minecraft.Root, cfg.Runtime.ScreenPrefix, cfg.Runtime.LogDir)
	consoleMgr := mcserver.NewConsoleManager(screenMgr)
	builder := project.NewRunner(dbconn)

	api := &httpapi.API{
		MCRoot:       cfg.Minecraft.Root,
		ProjectsRoot: cfg.Projects.Root,
		DB:           dbconn,
		Auth:         authStore,
		Limiter:      limiter,
		Screen:       screenMgr,
		Console:      consoleMgr,
		Builder:      builder,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Listen, cfg.Server.Port)
	log.Printf("mcpanel listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, httpapi.NewRouter(api)))
}