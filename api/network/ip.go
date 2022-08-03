package network

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/api/helpers"
	"golang.org/x/exp/slices"
	"net/http"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type Route struct {
  Dst string
  Gateway string
  Dev string
  Metric int
  Flags []string
}

type AddressInfo struct {
  Family string
  Local string
  Prefixlen int
  Broadcast string
  Scope string
  Label string
  Valid_life_time int
  Preferred_life_time int
}

type NetworkInterface struct {
  Ifindex int
  Ifname string
  Flags []string
  Mtu int
  Qdisc string
  Operstate string
  Group string
  Txqlen int

  Addr_info []AddressInfo
}

type HostNetworkInfo struct {
  Mac string
  Ip string
  Vendor string
}

func GetPrimaryIpAddress(w http.ResponseWriter, r *http.Request) {
	externalInterfaceNames, err := getDefaultRouteInterfaceNames()
	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}

	var ipAddress string
	networkInterfaces, err := getNetworkInterfaces()
	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}
	for _, networkInterface := range networkInterfaces {
		if (!slices.Contains(externalInterfaceNames, networkInterface.Ifname)) {
			ipAddress = networkInterface.Addr_info[0].Local
		}
	}
	helpers.WriteJSON(w, http.StatusOK, ipAddress)
}

func GetLocalHosts(w http.ResponseWriter, r *http.Request) {
	externalInterfaceNames, err := getDefaultRouteInterfaceNames()
	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}

	var localInterfaces []NetworkInterface
	networkInterfaces, err := getNetworkInterfaces()
	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}
	for _, networkInterface := range networkInterfaces {
		if (!slices.Contains(externalInterfaceNames, networkInterface.Ifname)) {
			log.Debug("Found local network interface: " + networkInterface.Ifname)
			localInterfaces = append(localInterfaces, networkInterface)
		}
	}

	if (len(localInterfaces) == 0) {
		err := errors.New("Apparently no local subnets on this computer")
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}

	if (len(localInterfaces) > 1) {
		err := errors.New("More than one possible local subnet on this computer")
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}

	privateSubnet := localInterfaces[0].Addr_info[0]
	sshHosts, err := getSshHosts(privateSubnet.Local + "/" + strconv.Itoa(privateSubnet.Prefixlen))
	if err != nil {
		log.Error(err)
		helpers.WriteError(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, sshHosts)
}

func getDefaultRouteInterfaceNames() ([]string, error) {
	var defaultInterfaces []string

	output, err := runCommand("ip", "-j", "route", "list", "default")

	if (err != nil) {
		return defaultInterfaces, err
	}

	var routes []Route
	json.Unmarshal([]byte(output), &routes)

	interfaces := make(map[string]bool)
	for _, route := range routes {
		dev := route.Dev
		if _, value := interfaces[dev]; !value {
			interfaces[route.Dev] = true
			defaultInterfaces = append(defaultInterfaces, route.Dev)
		}
	}

	return defaultInterfaces, nil
}

func getNetworkInterfaces() ([]NetworkInterface, error) {
	var interfaces []NetworkInterface
	output, err := runCommand("ip", "-j", "-family", "inet", "addr", "show")

	if (err != nil) {
		return interfaces, err
	}

	log.Debug("Interfaces (command output):\n" + output)
	json.Unmarshal([]byte(output), &interfaces)

	var networkInterfaces []NetworkInterface
	matcher, _ := regexp.Compile(`^(lo|tun|docker)`)
	for _, potentialInterface := range interfaces {
		if (!matcher.MatchString(potentialInterface.Ifname) &&
		    !slices.Contains(potentialInterface.Flags, "NO-CARRIER")) {
			networkInterfaces = append(networkInterfaces, potentialInterface)
		}
	}

	return networkInterfaces, nil
}

func getSshHosts(subnet string) ([]HostNetworkInfo, error) {
	hostReports := []HostNetworkInfo{}

	output, err := runCommand("sudo", "nmap", "-Pn", "-p22", "--open", subnet)
	if (err != nil) {
		return hostReports, err
	}

	matcher := regexp.MustCompile(`(?s)((?:[[:digit:]]{1,3}\.){3,3}[[:digit:]]{1,3}).*(?:MAC Address: ((?:[[:xdigit:]]{2,2}:){5,5}[[:xdigit:]]{2,2})\s+\((.*)\))`)

	for _, scanReport := range regexp.MustCompile("Nmap scan report for ").Split(output, -1) {
		log.Debug(scanReport)
		matched := matcher.FindStringSubmatch(scanReport)
		if matched != nil {
			host := HostNetworkInfo{
				Ip: matched[1],
				Mac: strings.ToLower(matched[2]),
				Vendor: matched[3],
			}
			hostReports = append(hostReports, host)
		}
	}
	return hostReports, nil
}
