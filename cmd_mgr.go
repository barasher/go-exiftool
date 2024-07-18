package exiftool

import "fmt"

func (e *Exiftool) GetPidInfo() (int, string, error) {
	if e == nil {
		return 0, "", fmt.Errorf("exiftool not initialized")
	}
	if e.cmd == nil {
		return 0, "", fmt.Errorf("exiftool cmd not initialized")
	}

	if e.cmd.Process == nil {
		return 0, "", fmt.Errorf("exiftool process not initialized")
	}

	return e.cmd.Process.Pid, e.cmd.String(), nil
}
