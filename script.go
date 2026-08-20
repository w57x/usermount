package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func createUser(username, password string) error {
	scriptPath := AppConfig.ScriptPath

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Printf("[MOCK] Would run script %s for user %s\n", scriptPath, username)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Env = append(os.Environ(),
		"USERMOUNT_USERNAME="+username,
		"USERMOUNT_PASSWORD="+password,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("script execution timed out after 30s")
		}
		return fmt.Errorf("script failed: %s, output: %s", err, string(out))
	}

	return nil
}

func deleteUserSystem(username string) error {
	scriptPath := AppConfig.DeleteScriptPath

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Printf("[MOCK] Would run delete script %s for user %s\n", scriptPath, username)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Env = append(os.Environ(),
		"USERMOUNT_USERNAME="+username,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("teardown script execution timed out after 30s")
		}
		return fmt.Errorf("teardown script failed: %s, output: %s", err, string(out))
	}

	return nil
}

func updateUserPasswordSystem(username, newPassword string) error {
	scriptPath := AppConfig.UpdatePasswordScriptPath

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Printf("[MOCK] Would run update password script %s for user %s\n", scriptPath, username)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Env = append(os.Environ(),
		"USERMOUNT_USERNAME="+username,
		"USERMOUNT_PASSWORD="+newPassword,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("password update script execution timed out after 30s")
		}
		return fmt.Errorf("password update script failed: %s, output: %s", err, string(out))
	}

	return nil
}
