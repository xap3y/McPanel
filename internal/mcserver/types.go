package mcserver

import "time"

type Status string

const (
	StatusOnline  Status = "ONLINE"
	StatusOffline Status = "OFFLINE"
	StatusCrashed Status = "CRASHED"
)

type Settings struct {
	JavaPath        string
	Xms             string
	Xmx             string
	JVMArgs         []string
	ServerJar       string
	ServerArgs      []string
	VelocityEnabled bool
	ServerPort      int
}

type Server struct {
	Name      string
	Dir       string
	Status    Status
	PID       int
	StartedAt time.Time
	RAMBytes  uint64
	CPUPct    float64
	Settings  Settings
}
