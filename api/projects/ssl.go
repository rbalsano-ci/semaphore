package projects

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/api/helpers"
	"github.com/gorilla/mux"
	/* "github.com/ansible-semaphore/semaphore/db" */
	"bytes"
	/* "github.com/gorilla/context" */
	"net/http"
	"os"
	"os/exec"
	"regexp"
	/* "net/url" */
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"errors"
)

const PkiDirectory = "/tmp/provisioning/pki"

type FileSummary struct {
  KeyName       string
  KeyModulus    string
  CertName      string
  CertSubject   string
  CertModulus   string
  Valid	 bool
  InvalidReason string
}

func GetAllPki(w http.ResponseWriter, r *http.Request) {
	err := ensureDirectoryExists(PkiDirectory)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	macAddresses, err := getPkiDirectories(PkiDirectory)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, macAddresses)
}

func GetPki(w http.ResponseWriter, r *http.Request) {
	err := ensureDirectoryExists(PkiDirectory)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	macAddress := strings.ToLower(mux.Vars(r)["mac_address"])

	summary, err := getFileSummary(PkiDirectory + "/" + macAddress)
	if errors.Is(err, fs.ErrNotExist) {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, summary)
}

func UpdatePki(w http.ResponseWriter, r *http.Request) {
	macAddress := strings.ToLower(mux.Vars(r)["mac_address"])
	deviceDirectory := PkiDirectory + "/" + macAddress
	err := ensureDirectoryExists(deviceDirectory)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	for _, param := range []string{"key-file", "crt-file"} {
		_, fileHeader, err := r.FormFile(param)
		if errors.Is(err, http.ErrMissingFile) {
			continue
		} else if err != nil {
			helpers.WriteError(w, err)
			return
		}
		file, err := fileHeader.Open()
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		buf := bytes.NewBuffer(nil)
		io.Copy(buf, file)
		file.Close()
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		err = os.WriteFile(deviceDirectory + "/" + fileHeader.Filename, buf.Bytes(), 0644)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func DeletePki(w http.ResponseWriter, r *http.Request) {
	macAddress := strings.ToLower(mux.Vars(r)["mac_address"])
	deviceDirectory := PkiDirectory + "/" + macAddress
	err := ensureDirectoryExists(deviceDirectory)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	files, err := os.ReadDir(deviceDirectory)
	for _, file := range files {
		filename := file.Name()
		err = os.Remove(deviceDirectory + "/" + filename)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
	}
	err = os.Remove(deviceDirectory)
	if err != nil {
	       helpers.WriteError(w, err)
	       return
	}
	w.WriteHeader(http.StatusNoContent)
}

func DeletePkiFile(w http.ResponseWriter, r *http.Request) {
	macAddress := strings.ToLower(mux.Vars(r)["mac_address"])
	deviceDirectory := PkiDirectory + "/" + macAddress
	err := ensureDirectoryExists(deviceDirectory)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	fileName := mux.Vars(r)["file_name"]
	err = os.Remove(deviceDirectory + "/" + fileName)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	files, err := os.ReadDir(deviceDirectory)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	if len(files) == 0 {
		err = os.Remove(deviceDirectory)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func ensureDirectoryExists(directory string) error {
	return os.MkdirAll(directory, 0700)
}

func getFileSummary(directory string) (FileSummary, error) {
	_, err := os.Stat(directory)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Error(err)
		}
		return FileSummary{}, err
	}
	files, err := os.ReadDir(directory)
	if err != nil {
		log.Error(err)
		return FileSummary{}, err
	}
	var summary FileSummary
	summary.InvalidReason = ""
	if len(files) == 0 {
		return FileSummary{}, os.ErrNotExist
	} else if len(files) != 2 {
		var fileNames []string
		for _, file := range files {
			fileNames = append(fileNames, file.Name())
		}
		summary.InvalidReason = "Expect exactly two files, but got " + strings.Join(fileNames,",") + "."
	}
	for _, file := range files {
		filename := file.Name()
		path := directory + "/" + filename
		modulus, err := getModulus(path)
		if err != nil {
			log.Error(err)
			return FileSummary{}, err
		}
		ext := filepath.Ext(path)
		if ext == ".key" {
			summary.KeyName = filename
			summary.KeyModulus = modulus
		} else if ext == ".crt" {
			subject, err := getSubject(path)
			if err != nil {
				log.Error(err)
				return FileSummary{}, err
			}
			summary.CertName = filename
			summary.CertModulus = modulus
			summary.CertSubject = subject
		}
	}
	if (len(summary.KeyModulus) == 0) ||
	   (len(summary.CertModulus) == 0) ||
	   (summary.KeyModulus != summary.CertModulus) {
		if summary.InvalidReason == "" {
			summary.InvalidReason = "Modulus missing or mismatch between .key and .crt files."
		}
	}
	summary.Valid = (summary.InvalidReason == "")

	return summary, nil
}

func getPkiDirectories(directory string) ([]string, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		log.Error(err)
		return []string{}, err
	}
	var directories []string
	for _, file := range files {
		if file.IsDir() {
			directories = append(directories, file.Name())
		}
	}

	return directories, nil
}

func getSubject(filename string) (string, error) {
	output, err := runCommand("/usr/bin/openssl", "x509", "-noout", "-subject", "-in", filename)
	if err != nil {
		log.Error(err)
		return "", err
	}
	matcher, _ := regexp.Compile(`subject=CN\s*=\s*(.*)`)
	matches := matcher.FindStringSubmatch(output)
	if matches == nil {
		err = errors.New("Unable to find common name in subject")
		log.Error(err)
		return "", err
	}
	return matches[1], nil
}

func runCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Stdin = nil

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()

	if err != nil {
		log.Error(err)
		return "", err
	}

	return out.String(), nil
}

func getModulus(filename string) (string, error) {
	var sslType string
	ext := filepath.Ext(filename)
	if ext == ".key" {
		sslType = "rsa"
	} else if ext == ".crt" {
		sslType = "x509"
	} else {
		err := errors.New("Extension of '" + ext + "' is not '.key' or '.crt'")
		log.Error(err)
		return "", err
	}

	cmd1 := exec.Command("/usr/bin/openssl", sslType, "-noout", "-modulus", "-in", filename)
	cmd2 := exec.Command("/usr/bin/openssl", "md5")

	r, w := io.Pipe()
	cmd2.Stdin = r
	cmd1.Stdout = w

	var output bytes.Buffer
	cmd2.Stdout = &output

	var error1 bytes.Buffer
	var error2 bytes.Buffer
	cmd1.Stderr = &error1
	cmd2.Stderr = &error2

	cmd1.Start()
	cmd2.Start()
	cmd1.Wait()
	w.Close()
	cmd2.Wait()
	if error1.String() != "" {
		err := errors.New(error1.String())
		log.Error(err)
		return "", err
	}
	if error2.String() != "" {
		err := errors.New(error2.String())
		log.Error(err)
		return "", err
	}

	return strings.TrimSuffix(output.String(), "\n"), nil
}
