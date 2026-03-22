package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Role int

const (
	RoleViewer Role = iota
	RoleDeveloper
	RoleOperator
	RoleAdmin
)

type Action string

const (
	ActionViewStatus    Action = "view:status"
	ActionViewLogs      Action = "view:logs"
	ActionViewMetrics   Action = "view:metrics"
	ActionViewDoctor    Action = "view:doctor"
	ActionViewWorkspace Action = "view:workspace"

	ActionOpenLab Action = "suite:lab"
	ActionShell   Action = "suite:shell"
	ActionExec    Action = "suite:exec"

	ActionInstall  Action = "suite:install"
	ActionRemove   Action = "suite:remove"
	ActionStart    Action = "suite:start"
	ActionStop     Action = "suite:stop"
	ActionUpdate   Action = "suite:update"
	ActionRollback Action = "suite:rollback"
	ActionBackup   Action = "workspace:backup"
	ActionClean    Action = "workspace:clean"

	ActionReboot        Action = "system:reboot"
	ActionShutdown      Action = "system:shutdown"
	ActionRestartDocker Action = "system:restart-docker"
	ActionManageUsers   Action = "system:manage-users"
)

type Session struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Role      Role      `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

var rolePermissions = map[Role][]Action{
	RoleViewer: {
		ActionViewStatus, ActionViewLogs, ActionViewMetrics, ActionViewDoctor, ActionViewWorkspace,
	},
	RoleDeveloper: {
		ActionOpenLab, ActionShell, ActionExec,
	},
	RoleOperator: {
		ActionInstall, ActionRemove, ActionStart, ActionStop, ActionUpdate, ActionRollback, ActionBackup, ActionClean,
	},
	RoleAdmin: {
		ActionReboot, ActionShutdown, ActionRestartDocker, ActionManageUsers,
	},
}

func (r Role) String() string {
	switch r {
	case RoleViewer:
		return "viewer"
	case RoleDeveloper:
		return "developer"
	case RoleOperator:
		return "operator"
	case RoleAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

func (r Role) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

func (r *Role) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		parsed, err := ParseRole(text)
		if err != nil {
			return err
		}
		*r = parsed
		return nil
	}

	var numeric int
	if err := json.Unmarshal(data, &numeric); err != nil {
		return err
	}
	*r = Role(numeric)
	return nil
}

func ParseRole(value string) (Role, error) {
	switch value {
	case "viewer":
		return RoleViewer, nil
	case "developer":
		return RoleDeveloper, nil
	case "operator":
		return RoleOperator, nil
	case "admin":
		return RoleAdmin, nil
	default:
		return 0, fmt.Errorf("unknown role %q", value)
	}
}

func Can(role Role, action Action) bool {
	for current := RoleViewer; current <= role; current++ {
		for _, allowed := range rolePermissions[current] {
			if allowed == action {
				return true
			}
		}
	}
	return false
}

func SessionPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".config", "concave", "session.json")
	}
	return filepath.Join(configDir, "concave", "session.json")
}

func LoadSession() (Session, error) {
	path := SessionPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if session.Token == "" || session.ExpiresAt.IsZero() || time.Now().After(session.ExpiresAt) {
		return Session{}, fmt.Errorf("session expired")
	}
	return session, nil
}

func SaveSession(session Session) error {
	path := SessionPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func ClearSession() error {
	err := os.Remove(SessionPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
