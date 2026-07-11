package palworld

import (
	"os/exec"
	"strconv"
	"strings"
)

func processIDsOS(name string) ([]int, error) {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name+".exe", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return nil, err
	}
	var ids []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}
		pidStr := strings.Trim(fields[1], "\"")
		pid, err := strconv.Atoi(pidStr)
		if err == nil {
			ids = append(ids, pid)
		}
	}
	return ids, nil
}
