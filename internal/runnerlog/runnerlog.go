package runnerlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CommandRun represents a command execution record
type CommandRun struct {
	BackupID    string    `json:"backup_id"`
	DBType      string    `json:"db_type"`
	Job         string    `json:"job"`
	CommandType string    `json:"command_type"`
	Command     string    `json:"command"`
	ExitCode    int       `json:"exit_code"`
	Stdout      string    `json:"stdout,omitempty"`
	Stderr      string    `json:"stderr,omitempty"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	DurationMs  int64     `json:"duration_ms"`
}

// Logger handles command run logging
type Logger struct {
	logFile string
}

// NewLogger creates a new logger
func NewLogger(dataDir string) *Logger {
	logDir := filepath.Join(dataDir, "logs")
	os.MkdirAll(logDir, 0750)

	return &Logger{
		logFile: filepath.Join(logDir, "command_runs.jsonl"),
	}
}

// Log logs a command run
func (l *Logger) Log(run CommandRun) error {
	f, err := os.OpenFile(l.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(run)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

// Query queries command runs
func (l *Logger) Query(backupID string, limit int) ([]CommandRun, error) {
	f, err := os.Open(l.logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var runs []CommandRun
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var run CommandRun
		if err := decoder.Decode(&run); err != nil {
			continue
		}
		if backupID == "" || run.BackupID == backupID {
			runs = append(runs, run)
		}
	}

	// Return last N records
	if limit > 0 && len(runs) > limit {
		runs = runs[len(runs)-limit:]
	}

	return runs, nil
}
