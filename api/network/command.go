package network

import (
	log "github.com/Sirupsen/logrus"
	"bytes"
	"os/exec"
	"strings"
)

func runCommand(command string, args ...string) (string, error) {
	log.Debug("Command is: " + command + " " + strings.Join(args, " "))
	cmd := exec.Command(command, args...)

	cmd.Stdin = nil

	var out bytes.Buffer
	var err_out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &err_out
	err := cmd.Run()

	if err != nil {
		log.Error(err)
		log.Error(err_out.String())
		return "", err
	}

	return out.String(), nil
}
