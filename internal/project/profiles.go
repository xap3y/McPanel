package project

import (
	"database/sql"
	"strings"
)

type BuildProfile struct {
	ID          int
	ProjectName string
	ProfileName string
	Executable  string
	Args        []string
	Command     string
	IsDefault   bool
}

func ListProfiles(db *sql.DB, projectName string) ([]BuildProfile, error) {
	rows, err := db.Query(`SELECT id, profile_name, executable, args, is_default FROM build_profiles WHERE project_name = ?`, projectName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BuildProfile
	for rows.Next() {
		var p BuildProfile
		var argsStr string
		if err := rows.Scan(&p.ID, &p.ProfileName, &p.Executable, &argsStr, &p.IsDefault); err != nil {
			return nil, err
		}
		p.ProjectName = projectName
		p.Args = splitLines(argsStr)
		if p.Executable == "/bin/sh" && len(p.Args) == 2 && p.Args[0] == "-c" {
			p.Command = p.Args[1]
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		// sensible defaults until the user configures their own
		out = append(out,
			BuildProfile{ProjectName: projectName, ProfileName: "Default", Executable: "./gradlew", Args: []string{"build"}, IsDefault: true},
			BuildProfile{ProjectName: projectName, ProfileName: "Clean Build", Executable: "./gradlew", Args: []string{"clean", "build"}},
		)
	}
	return out, nil
}

func SaveProfile(db *sql.DB, p BuildProfile) error {
	if p.Command != "" {
		p.Executable = "/bin/sh"
		p.Args = []string{"-c", p.Command}
	}
	if p.IsDefault {
		db.Exec(`UPDATE build_profiles SET is_default = FALSE WHERE project_name = ?`, p.ProjectName)
	}
	_, err := db.Exec(`
		INSERT INTO build_profiles (project_name, profile_name, executable, args, is_default)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE executable=VALUES(executable), args=VALUES(args), is_default=VALUES(is_default)`,
		p.ProjectName, p.ProfileName, p.Executable, joinLines(p.Args), p.IsDefault)
	return err
}

func DeleteProfile(db *sql.DB, projectName, profileName string) error {
	_, err := db.Exec(`DELETE FROM build_profiles WHERE project_name = ? AND profile_name = ?`, projectName, profileName)
	return err
}

type ProjectSettings struct {
	ArtifactGlob       string   `json:"ArtifactGlob"`
	DeploymentFilename string   `json:"DeploymentFilename"` // empty = keep original artifact filename
	DeployTargets      []string `json:"DeployTargets"`
}

func LoadProjectSettings(db *sql.DB, projectName string) ProjectSettings {
	var glob, destFilename, targets sql.NullString
	err := db.QueryRow(`SELECT artifact_glob, deployment_filename, deploy_targets FROM project_settings WHERE project_name = ?`, projectName).
		Scan(&glob, &destFilename, &targets)
	ps := ProjectSettings{ArtifactGlob: "*.jar"}
	if err == nil {
		if glob.Valid {
			ps.ArtifactGlob = glob.String
		}
		if destFilename.Valid {
			ps.DeploymentFilename = destFilename.String
		}
		if targets.Valid && targets.String != "" {
			ps.DeployTargets = strings.Split(targets.String, ",")
		}
	}
	return ps
}

func SaveProjectSettings(db *sql.DB, projectName string, ps ProjectSettings) error {
	var destNull interface{}
	if ps.DeploymentFilename != "" {
		destNull = ps.DeploymentFilename
	}
	_, err := db.Exec(`
		INSERT INTO project_settings (project_name, artifact_glob, deployment_filename, deploy_targets)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE artifact_glob=VALUES(artifact_glob), deployment_filename=VALUES(deployment_filename), deploy_targets=VALUES(deploy_targets)`,
		projectName, ps.ArtifactGlob, destNull, strings.Join(ps.DeployTargets, ","))
	return err
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func joinLines(lines []string) string { return strings.Join(lines, "\n") }
