//go:build windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// killPortOwner chiude il processo (una versione precedente dell'app) che occupa la porta locale dell'app.
func killPortOwner(addr string) {
	cmd := exec.Command("netstat", "-ano", "-p", "TCP")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || f[0] != "TCP" || f[1] != addr || !strings.HasPrefix(f[2], "0.0.0.0:") {
			continue
		}
		pid, err := strconv.Atoi(f[len(f)-1])
		if err != nil || pid <= 4 || pid == os.Getpid() {
			continue
		}
		k := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F")
		k.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		k.Run()
	}
}
