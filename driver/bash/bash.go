// Package bash implements the Driver interface.
package bash

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/gemnasium/migrate/driver"
	"github.com/gemnasium/migrate/file"
	"github.com/gemnasium/migrate/migrate/direction"
)

type Driver struct {
	sync.Mutex
	versionFile string
	versions    file.Versions
}

func (driver *Driver) writeVersions() error {
	wFile, err := os.Create(driver.versionFile)
	if err != nil {
		return err
	}
	defer wFile.Close()
	driver.Lock()
	defer driver.Unlock()
	sort.Sort(driver.versions)
	for _, version := range driver.versions {
		if _, err := fmt.Fprintf(wFile, "%d\n", version); err != nil {
			return err
		}
	}
	return nil
}

func (driver *Driver) addVersion(v file.Version) {
	driver.Lock()
	driver.versions = append(driver.versions, v)
	driver.Unlock()
}

func (driver *Driver) removeVersion(v file.Version) {
	driver.Lock()
	defer driver.Unlock()
	index := func() int {
		for i, ver := range driver.versions {
			if ver == v {
				return i
			}
		}
		return -1
	}()
	if index < 0 {
		return
	}
	driver.versions = append(driver.versions[:index], driver.versions[index+1:]...)
}

func (driver *Driver) Initialize(url string) error {
	driver.versions = []file.Version{}
	urlParts := strings.Split(url, ":")
	driver.versionFile = urlParts[1]
	// try to open file for reading
	rFile, err := os.Open(driver.versionFile)
	if err != nil {
		return nil
	}
	defer rFile.Close()
	driver.Lock()
	defer driver.Unlock()
	for {
		var version uint64
		_, scanErr := fmt.Fscanf(rFile, "%d\n", &version)
		if scanErr != nil {
			break
		}
		driver.versions = append(driver.versions, file.Version(version))
	}
	sort.Sort(driver.versions)
	return nil
}

func (driver *Driver) Close() error {
	return driver.writeVersions()
}

func (driver *Driver) FilenameExtension() string {
	return "sh"
}

func (driver *Driver) Migrate(f file.File, pipe chan interface{}) {
	scriptPath := path.Join(f.Path, f.FileName)
	cmd := exec.Command("bash", scriptPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	output, err := ioutil.ReadAll(stdout)
	outString := string(output[:])
	cmd.Wait()
	if err != nil {
		log.Fatal(err)
	}
	defer close(pipe)
	pipe <- outString
	if f.Direction == direction.Up {
		driver.addVersion(f.Version)
	} else {
		driver.removeVersion(f.Version)
	}
	return
}

func (driver *Driver) Version() (file.Version, error) {
	return driver.versions[len(driver.versions)-1], nil
}

func (driver *Driver) Versions() (file.Versions, error) {
	return driver.versions, nil
}

func init() {
	driver.RegisterDriver("bash", &Driver{})
}
