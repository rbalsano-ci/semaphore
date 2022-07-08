package network

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/api/helpers"
	"github.com/gorilla/mux"
	"bytes"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"errors"
)

func GetIpAddress(w http.ResponseWriter, r *http.Request) {
	macAddress := strings.ToLower(mux.Vars(r)["mac_address"])
	cmd := exec.Command("arp", "-ne")

	cmd.Stdin = nil

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()

	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}

	matcher, _ := regexp.Compile(`(i?)((?:\d{1,3}\.){3,3}\d{1,3})\s+ether\s+` + macAddress)
	matches := matcher.FindStringSubmatch(out.String())
	if matches == nil {
		helpers.WriteError(w, errors.New("IP address for MAC address " + macAddress + " not found"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, map[string]string{"ip": matches[2]})
}

func GetMacAddress(w http.ResponseWriter, r *http.Request) {
	ipAddress := strings.ToLower(mux.Vars(r)["ip_address"])
	cmd := exec.Command("arp", "-ne")

	cmd.Stdin = nil

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()

	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}

	matcher, _ := regexp.Compile(`(i?)` + ipAddress + `\s+ether\s+((?:[[:xdigit:]]{2,2}\:){5,5}[[:xdigit:]]{2,2})`)
	matches := matcher.FindStringSubmatch(out.String())
	if matches == nil {
		helpers.WriteError(w, errors.New("MAC address for IP address " + ipAddress + " not found"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, map[string]string{"mac": matches[2]})
}
