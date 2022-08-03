package network

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/api/helpers"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"regexp"
	"strings"
	"errors"
)

func ReplaceKnownHostEntry(w http.ResponseWriter, r *http.Request) {
	ipAddress := strings.ToLower(mux.Vars(r)["ip_address"])
	matcher := regexp.MustCompile(`^(?:\d{1,3}\.){3,3}\d{1,3}$`)
	if !matcher.MatchString(ipAddress) {
		err := errors.New("IP address " + ipAddress + " is invalid")
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}
	fileName, err := getKnownHostsFileName()
	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}
	err = ensureFileExists(fileName)
	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}
	err = removeHostEntry(ipAddress, fileName)
	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}
	err = addHostEntry(ipAddress, fileName)
	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func getKnownHostsFileName() (string, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	filepath := (dirname + "/.ssh/known_hosts")
	return filepath, nil
}

func ensureFileExists(fileName string) (error) {
	_, err := os.Stat(fileName)
	if errors.Is(err, os.ErrNotExist) {
		_, err = os.Create(fileName)
	}
	return err
}

func removeHostEntry(ipAddress string, knownHostsFileName string) (error) {
	_, err := runCommand("sed", "-i", "/^" + ipAddress + "\\s/d", knownHostsFileName)
	return err
}

func addHostEntry(ipAddress string, knownHostsFileName string) (error) {
	_, err := runCommand("/bin/sh", "-c", "ssh-keyscan -t rsa " + ipAddress + " >> " + knownHostsFileName)
	return err
}
