package main

import (
	"fmt"
	"os"
	"os/exec"
)

func createUser(username, password string) error {
	scriptPath := AppConfig.ScriptPath

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Printf("[MOCK] Would run script %s for user %s\n", scriptPath, username)
		return nil
	}

	cmd := exec.Command(scriptPath, username, password)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("script failed: %s, output: %s", err, string(out))
	}

	return nil
}
