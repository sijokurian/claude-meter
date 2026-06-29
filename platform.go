package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func notify(title, subtitle, message string) {
	if runtime.GOOS == "darwin" {
		if path, err := exec.LookPath("terminal-notifier"); err == nil {
			exec.Command(path,
				"-title", title,
				"-subtitle", subtitle,
				"-message", message,
			).Run()
			return
		}
		script := fmt.Sprintf(
			`display notification %q with title %q subtitle %q`,
			message, title, subtitle,
		)
		exec.Command("osascript", "-e", script).Run()
	} else {
		cmd := []string{"-t", "5000", title, subtitle + "\n" + message}
		exec.Command("notify-send", cmd...).Run()
	}
}

func askInput(title, prompt, dflt string) (string, bool) {
	if runtime.GOOS == "darwin" {
		script := fmt.Sprintf(
			`display dialog %q with title %q default answer %q buttons {"Cancel", "OK"} default button "OK"`,
			prompt, title, dflt,
		)
		out, err := exec.Command("osascript", "-e", script).Output()
		if err != nil {
			return "", false
		}
		for _, part := range strings.Split(strings.TrimSpace(string(out)), ", ") {
			if strings.HasPrefix(part, "text returned:") {
				return part[len("text returned:"):], true
			}
		}
		return "", false
	}

	out, err := exec.Command("zenity", "--entry",
		"--title", title,
		"--text", prompt,
		"--entry-text", dflt,
	).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func showAlert(title, message string) {
	if runtime.GOOS == "darwin" {
		script := fmt.Sprintf(
			`display dialog %q with title %q buttons {"OK"} default button "OK"`,
			message, title,
		)
		exec.Command("osascript", "-e", script).Run()
	} else {
		exec.Command("zenity", "--info",
			"--title", title,
			"--text", message,
		).Run()
	}
}
