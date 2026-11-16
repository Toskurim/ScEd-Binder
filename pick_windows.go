//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
)

// pickFolder apre la finestra standard di Windows "Sfoglia cartelle" tramite PowerShell.
func pickFolder(title string) (string, error) {
	script := `Add-Type -AssemblyName System.Windows.Forms; ` +
		`$d = New-Object System.Windows.Forms.FolderBrowserDialog; ` +
		`$d.Description = '` + strings.ReplaceAll(title, "'", "''") + `'; $d.ShowNewFolderButton = $false; ` +
		`$o = New-Object System.Windows.Forms.Form; $o.TopMost = $true; ` +
		`if ($d.ShowDialog($o) -eq 'OK') { [Console]::Out.Write($d.SelectedPath) }`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
