package settings

import (
	"database/sql"
	"strings"
)

type GlobalSettings struct {
	Subfolders []string
}

func LoadGlobal(db *sql.DB) GlobalSettings {
	var g GlobalSettings
	var val sql.NullString
	err := db.QueryRow(`SELECT setting_value FROM global_settings WHERE setting_key = 'subfolders'`).Scan(&val)
	if err == nil && val.Valid && val.String != "" {
		for _, s := range strings.Split(val.String, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				g.Subfolders = append(g.Subfolders, s)
			}
		}
	}
	return g
}

func SaveGlobal(db *sql.DB, g GlobalSettings) error {
	val := strings.Join(g.Subfolders, ",")
	_, err := db.Exec(`
		INSERT INTO global_settings (setting_key, setting_value)
		VALUES ('subfolders', ?)
		ON DUPLICATE KEY UPDATE setting_value=?
	`, val, val)
	return err
}
