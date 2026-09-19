package settings

import (
	"database/sql"
	"strings"
)

type GlobalSettings struct {
	Subfolders           []string
	DefaultBuildJavaHome string
	RootProxyServer      string
}

const DefaultBuildJavaHome = "/usr/lib/jvm/java-21-openjdk"

func LoadGlobal(db *sql.DB) GlobalSettings {
	g := GlobalSettings{DefaultBuildJavaHome: DefaultBuildJavaHome}
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
	if err := db.QueryRow(`SELECT setting_value FROM global_settings WHERE setting_key = 'default_build_java_home'`).Scan(&val); err == nil && val.Valid && val.String != "" {
		g.DefaultBuildJavaHome = val.String
	}
	if err := db.QueryRow(`SELECT setting_value FROM global_settings WHERE setting_key = 'root_proxy_server'`).Scan(&val); err == nil && val.Valid {
		g.RootProxyServer = val.String
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
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO global_settings (setting_key, setting_value)
		VALUES ('default_build_java_home', ?)
		ON DUPLICATE KEY UPDATE setting_value=?
	`, g.DefaultBuildJavaHome, g.DefaultBuildJavaHome)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO global_settings (setting_key, setting_value)
		VALUES ('root_proxy_server', ?)
		ON DUPLICATE KEY UPDATE setting_value=?
	`, g.RootProxyServer, g.RootProxyServer)
	return err
}
