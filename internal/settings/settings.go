package settings

import (
	"database/sql"
	"strings"

	"mcpanel/internal/mcserver"
)

func Load(db *sql.DB, serverName string) mcserver.Settings {
	s := mcserver.Settings{JavaPath: "/usr/bin/java", Xms: "512M", Xmx: "2G", ServerJar: "server.jar"}
	var jvmArgs, serverArgs sql.NullString
	row := db.QueryRow(`SELECT java_path, xms, xmx, jvm_args, server_jar, server_args, velocity_enabled
		FROM server_settings WHERE server_name = ?`, serverName)
	row.Scan(&s.JavaPath, &s.Xms, &s.Xmx, &jvmArgs, &s.ServerJar, &serverArgs, &s.VelocityEnabled)
	if jvmArgs.Valid {
		s.JVMArgs = splitLines(jvmArgs.String)
	}
	if serverArgs.Valid {
		s.ServerArgs = splitLines(serverArgs.String)
	} else {
		s.ServerArgs = []string{"nogui"}
	}
	return s
}

func Save(db *sql.DB, serverName string, s mcserver.Settings) error {
	_, err := db.Exec(`
		INSERT INTO server_settings (server_name, java_path, xms, xmx, jvm_args, server_jar, server_args, velocity_enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE java_path=VALUES(java_path), xms=VALUES(xms), xmx=VALUES(xmx),
			jvm_args=VALUES(jvm_args), server_jar=VALUES(server_jar), server_args=VALUES(server_args),
			velocity_enabled=VALUES(velocity_enabled)`,
		serverName, s.JavaPath, s.Xms, s.Xmx, strings.Join(s.JVMArgs, "\n"), s.ServerJar, strings.Join(s.ServerArgs, "\n"), s.VelocityEnabled)
	return err
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}