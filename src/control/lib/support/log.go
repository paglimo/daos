package support

import (
	"fmt"
	"os"
	"path/filepath"
	"os/exec"
	"strings"
	"io"

	"github.com/pkg/errors"

	"github.com/daos-stack/daos/src/control/server/config"
	"github.com/daos-stack/daos/src/control/common"
	"github.com/daos-stack/daos/src/control/lib/control"
)

func getRunningConf() (string, bool) {
	_, err := exec.Command("bash", "-c", "pidof daos_engine").Output()

    if err != nil {
        fmt.Println(" -- SAMIR -- ERROR -- daos_engine is not running on server")
        return "", false
    }	

	cmd := "ps -eo args | grep daos_engine | head -n 1 | grep -oP '(?<=-d )[^ ]*'"
	stdout, err := exec.Command("bash", "-c", cmd).Output()
	running_config := filepath.Join(strings.TrimSpace(string(stdout)), config.ConfigOut)

	return running_config, true
}

func getServerConf() string{
	conf, err :=  getRunningConf()

	if err == true {
        return conf
    }

	// Return the default config
	serverConfig := config.DefaultServer()
	default_path := filepath.Join(serverConfig.SocketDir, config.ConfigOut)

	return default_path
}

func cpFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	log_file_name := filepath.Base(src)

	fmt.Printf(" -- SAMIR -- Copy File %s to %s\n", log_file_name, dst)
	out, err := os.Create(filepath.Join(dst, log_file_name))
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

// Check if file or directory that starts with . which is hidden
func IsHidden(filename string) (bool) {
	if filename[0:1] == "." {
		return true
	} else {
		return false
	}
	return false
}

func CopyServerConfig(src, dst string) error {
	err := cpFile(src, dst)
	if err != nil {
		return err
	}

	// Rename the file if it's hidden
	result := IsHidden(filepath.Base(src))
	if result == true{
		hiddenConf := filepath.Join(dst, filepath.Base(src))
		nonhiddenConf := filepath.Join(dst, filepath.Base(src)[1:])
		os.Rename(hiddenConf, nonhiddenConf)
	}

	return nil
}

func createLogfolder(target string) (string,  error) {
	hn, err := os.Hostname()
	if err != nil {
		return "", err
	}

	targetLocation := filepath.Join(target, hn)
	fmt.Println("log_location Folder = ", targetLocation)
	// Create the folder if it's not exist
	if _, err = os.Stat(targetLocation); os.IsNotExist(err) {
		fmt.Println("Log folder does not Exists, so creating folder ", string(targetLocation))

		if err := os.MkdirAll(targetLocation, 0700); err != nil && !os.IsExist(err) {
			return "", errors.Wrap(err, "failed to create log directory")
		}
    }

	return targetLocation, nil
}

func Collectdmglog(dst string, configPath string) error {
	_, err := createLogfolder(dst)
	if err != nil {
		return err
	}

	dmgLogfile := filepath.Join(dst, "dmg_ouput.log")

	for _, dmgCommand := range control.DmgLogCollectCmd {
		dmgCommand = strings.Join([]string{dmgCommand, "-o",  configPath}, " ")

		// executing as subshell enables pipes in cmd string and append output to the file.
		out, err := exec.Command("sh", "-c", dmgCommand).CombinedOutput()
		if err != nil {
			err = errors.Wrapf(
				err, "Error running %s: %s", dmgCommand, out)
		}

		f, err := common.AppendFile(dmgLogfile)
		if err != nil {
			return err
		}

		output:= strings.Repeat("=", 50) + "\n" + string(dmgCommand) + "\n" + strings.Repeat("-", 30)
		if _, err := f.WriteString(output); err != nil {
			return err
		}

		output = "\n" +string(out) + "\n"
		if _, err := f.WriteString(output); err != nil {
			return err
		}
		defer f.Close()
	}

	return nil
}

func CollectDaosLog(dst string) error {
	targetLocation, err := createLogfolder(dst)
	if err != nil {
		return err
	}

	cfgPath := getServerConf()
	serverConfig := config.DefaultServer()
	serverConfig.SetPath(cfgPath)
	serverConfig.Load()	

	// Copy server config file
	err = CopyServerConfig(cfgPath, dst)
	if err != nil {
		return err
	}

	// Copy DAOS server engine log files
	for i := range serverConfig.Engines {
		// fmt.Printf(" -- SAMIR -- server log_file[%d] ->  %s \n", i, serverConfig.Engines[i].LogFile)
		err := cpFile(serverConfig.Engines[i].LogFile, targetLocation)

		if err != nil {
			return err
		}
	}

	// Copy DAOS Control log file
	err = cpFile(serverConfig.ControlLogFile, targetLocation)
	if err != nil {
		return err
	}

	// Copy DAOS Helper log file
	// fmt.Printf(" -- SAMIR -- helper_log_file ->  %s \n", serverConfig.HelperLogFile)
	err = cpFile(serverConfig.HelperLogFile, targetLocation)
	if err != nil {
		return err
	}

	return nil
}
