CREATE TABLE IF NOT EXISTS users (
  id INT AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(64) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS sessions (
  token CHAR(64) PRIMARY KEY,
  user_id INT NOT NULL,
  csrf_token CHAR(64) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS server_settings (
  server_name VARCHAR(128) PRIMARY KEY,
  java_path VARCHAR(512) NOT NULL DEFAULT '/usr/bin/java',
  xms VARCHAR(16) NOT NULL DEFAULT '512M',
  xmx VARCHAR(16) NOT NULL DEFAULT '2G',
  jvm_args TEXT,
  server_jar VARCHAR(256) NOT NULL DEFAULT 'server.jar',
  server_args TEXT,
  velocity_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  auto_restart_on_crash BOOLEAN NOT NULL DEFAULT FALSE,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS build_profiles (
  id INT AUTO_INCREMENT PRIMARY KEY,
  project_name VARCHAR(128) NOT NULL,
  profile_name VARCHAR(128) NOT NULL,
  executable VARCHAR(256) NOT NULL DEFAULT './gradlew',
  args TEXT,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  UNIQUE KEY uniq_project_profile (project_name, profile_name)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS project_settings (
  project_name VARCHAR(128) PRIMARY KEY,
  artifact_glob VARCHAR(256) NOT NULL DEFAULT '*.jar',
  deployment_filename VARCHAR(256),
  deploy_targets TEXT,
  build_args TEXT,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;


CREATE TABLE IF NOT EXISTS build_history (
  id INT AUTO_INCREMENT PRIMARY KEY,
  project_name VARCHAR(128) NOT NULL,
  profile_name VARCHAR(128) NOT NULL,
  status VARCHAR(16) NOT NULL,
  started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  finished_at TIMESTAMP NULL,
  exit_code INT NULL,
  artifact_path VARCHAR(512) NULL,
  INDEX idx_project (project_name, started_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS deploy_history (
  id INT AUTO_INCREMENT PRIMARY KEY,
  project_name VARCHAR(128) NOT NULL,
  server_name VARCHAR(128) NOT NULL,
  build_id INT NULL,
  status VARCHAR(16) NOT NULL,
  restarted BOOLEAN NOT NULL DEFAULT FALSE,
  deployed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  error_message TEXT NULL,
  FOREIGN KEY (build_id) REFERENCES build_history(id) ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS custom_projects (
  name VARCHAR(128) PRIMARY KEY,
  dir VARCHAR(1024) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS global_settings (
  setting_key VARCHAR(128) PRIMARY KEY,
  setting_value TEXT
) ENGINE=InnoDB;
