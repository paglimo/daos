package support

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"os/exec"
	"strings"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"

	"github.com/daos-stack/daos/src/control/server/config"
	"github.com/daos-stack/daos/src/control/common"
	"github.com/daos-stack/daos/src/control/lib/control"
	"github.com/daos-stack/daos/src/control/logging"
	"github.com/daos-stack/daos/src/control/lib/hardware/hwprov"
	"github.com/daos-stack/daos/src/control/lib/hardware"
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

func cpOutputToFile(cmd string, target string) error {
	// Run command and copy output to the file
	// executing as subshell enables pipes in cmd string
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		err = errors.Wrapf(
			err, "Error running command %s with %s", cmd, out)
	}

	if err := ioutil.WriteFile(filepath.Join(target, cmd), out, 0644); err != nil {
		return errors.Wrapf(err, "failed to write %s", filepath.Join(target, cmd))
	}

	return nil
}

func Collectdmglog(dst string, configPath string) error {
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

func CollectServerLog(dst string) error {
	targetLocation, err := createLogfolder(dst)
	if err != nil {
		return err
	}

	// Get the server config
	cfgPath := getServerConf()
	serverConfig := config.DefaultServer()
	serverConfig.SetPath(cfgPath)
	serverConfig.Load()	

	// Copy server config file
	targetConfig := filepath.Join(dst, "Configs")
	if err := os.Mkdir(targetConfig, 0700); err != nil && !os.IsExist(err) {
		return errors.Wrapf(err, "failed to create %s directory", targetConfig)
	}
	err = CopyServerConfig(cfgPath, targetConfig)
	if err != nil {
		return err
	}

	// Copy DAOS server engine log files
	for i := range serverConfig.Engines {
		// Find the matching file incase of log file is based on PID or it has backup
		matches, _ := filepath.Glob(serverConfig.Engines[i].LogFile + "*")
		for _, logfile := range matches {
			err := cpFile(logfile, targetLocation)
			if err != nil {
				return err
			}
		}
	}

	// Copy DAOS Control log file
	err = cpFile(serverConfig.ControlLogFile, targetLocation)
	if err != nil {
		return err
	}

	// Copy DAOS Helper log file
	err = cpFile(serverConfig.HelperLogFile, targetLocation)
	if err != nil {
		return err
	}

	// Copy daos_metrics log
	for i := range serverConfig.Engines {
		engineId := fmt.Sprintf("%d", i)
		cmd := strings.Join([]string{"daos_metrics", "-S",  engineId}, " ")

		// executing as subshell enables pipes in cmd string
		out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
		if err != nil {
			err = errors.Wrapf(
				err, "Error running daos_metrics -S %d : %s", i, out)
		}

		engineIdLog := fmt.Sprintf("daos_metrics_srv_id_%d.log", i)
		if err := ioutil.WriteFile(filepath.Join(targetLocation, engineIdLog), out, 0644); err != nil {
			return errors.Wrapf(err, "failed to write %s", filepath.Join(targetLocation, engineIdLog))
		}
	}

	// Collect dump-topology output
	log := logging.NewCommandLineLogger()
	hwProv := hwprov.DefaultTopologyProvider(log)
	topo, err := hwProv.GetTopology(context.Background())
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(targetLocation, "dmg_dump-topology.log"))
    if err != nil {
        return err
    }
    defer f.Close()
	hardware.PrintTopology(topo, f)


	for _, sysCommand := range control.SysInfoCmd {
		targetSysinfo := filepath.Join(targetLocation, "sysinfo")
		if err := os.Mkdir(targetSysinfo, 0700); err != nil && !os.IsExist(err) {
			return errors.Wrapf(err, "failed to create Sysinfo directory %s", targetSysinfo)
		}
		err = cpOutputToFile(sysCommand, targetSysinfo)
		if err != nil {
			return err
		}
	}

	return nil

}
