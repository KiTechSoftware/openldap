package configure

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

var root = "/var/backups/ldappy/"

func Create(label string) (string, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	dir := filepath.Join(root, fmt.Sprintf("%s_%s", ts, label))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	_ = exec.Command("slapcat", "-n", "0", "-l", filepath.Join(dir, "config.ldif")).Run()
	_ = exec.Command("slapcat", "-n", "1", "-l", filepath.Join(dir, "data.ldif")).Run()
	return dir, nil
}

func Restore(dir string) error {
	_ = exec.Command("systemctl", "stop", "slapd").Run()
	_ = os.RemoveAll("/etc/ldap/slapd.d")
	_ = os.MkdirAll("/etc/ldap/slapd.d", 0o700)
	_ = exec.Command("slapadd", "-n", "0", "-F", "/etc/ldap/slapd.d", "-l", filepath.Join(dir, "config.ldif")).Run()
	_ = os.RemoveAll("/var/lib/ldap")
	_ = os.MkdirAll("/var/lib/ldap", 0o700)
	_ = exec.Command("slapadd", "-n", "1", "-l", filepath.Join(dir, "data.ldif")).Run()
	_ = exec.Command("chown", "-R", "openldap:openldap", "/etc/ldap/slapd.d", "/var/lib/ldap").Run()
	return exec.Command("systemctl", "start", "slapd").Run()
}

func List() ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}
