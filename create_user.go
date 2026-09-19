// Command create_user creates a user that can sign in to MCPanel.
//
// Examples:
//
//	go run ./create_user.go -username admin
//	go run ./create_user.go -config /etc/mcpanel/config.yaml -username admin
//
// The password is read from the terminal without echoing. The -password flag
// is available for non-interactive use, but may be visible in shell history
// or process listings.
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode"

	"golang.org/x/term"

	"mcpanel/internal/auth"
	"mcpanel/internal/config"
	"mcpanel/internal/db"
)

var errUsage = errors.New("username must be 1-64 characters and contain no whitespace")

func main() {
	configPath := flag.String("config", "configs/mcpanel.yaml", "path to the MCPanel config file")
	dsnOverride := flag.String("dsn", "", "database DSN (overrides the config file)")
	username := flag.String("username", "", "username to create")
	password := flag.String("password", "", "password (prefer the hidden prompt instead)")
	flag.Parse()

	if *username == "" && flag.NArg() == 1 {
		*username = flag.Arg(0)
	}
	*username = strings.TrimSpace(*username)
	if flag.NArg() > 1 || *username == "" {
		flag.Usage = func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] [username]\n\n", os.Args[0])
			flag.PrintDefaults()
		}
		flag.Usage()
		os.Exit(2)
	}

	if err := validateUsername(*username); err != nil {
		fatal(err)
	}

	if *password == "" {
		var err error
		*password, err = readPassword("Password: ")
		if err != nil {
			fatal(fmt.Errorf("read password: %w", err))
		}
		confirmation, err := readPassword("Confirm password: ")
		if err != nil {
			fatal(fmt.Errorf("read password confirmation: %w", err))
		}
		if *password != confirmation {
			fatal(errors.New("passwords do not match"))
		}
	}
	if len(*password) < 8 {
		fatal(errors.New("password must be at least 8 characters"))
	}

	c, err := config.Load(*configPath)
	if err != nil {
		fatal(fmt.Errorf("load config: %w", err))
	}
	dsn := c.Database.DSN
	if *dsnOverride != "" {
		dsn = *dsnOverride
	}
	if dsn == "" {
		fatal(errors.New("database DSN is empty; set database.dsn or use -dsn"))
	}

	database, err := db.Open(dsn)
	if err != nil {
		fatal(fmt.Errorf("open database: %w", err))
	}
	defer database.Close()

	if err := createUser(database, *username, *password); err != nil {
		fatal(err)
	}
	fmt.Printf("user %q created successfully\n", *username)
}

func createUser(database *sql.DB, username, password string) error {
	username = strings.TrimSpace(username)
	if err := validateUsername(username); err != nil {
		return err
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if _, err := database.Exec(
		`INSERT INTO users (username, password_hash) VALUES (?, ?)`,
		username, hash,
	); err != nil {
		return fmt.Errorf("create user %q: %w", username, err)
	}
	return nil
}

func validateUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" || len([]rune(username)) > 64 {
		return errUsage
	}
	for _, r := range username {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return errUsage
		}
	}
	return nil
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err == nil {
		return string(password), nil
	}
	if !term.IsTerminal(int(syscall.Stdin)) {
		return "", errors.New("stdin is not a terminal; use -password for non-interactive use")
	}
	return "", err
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
