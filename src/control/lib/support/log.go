package support

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/daos-stack/daos/src/control/common"
	"github.com/daos-stack/daos/src/control/lib/control"
	"github.com/daos-stack/daos/src/control/lib/hardware"
	"github.com/daos-stack/daos/src/control/lib/hardware/hwprov"
	"github.com/daos-stack/daos/src/control/logging"
	"github.com/daos-stack/daos/src/control/server/config"
)

// Folder structure to copy logs and configs
const (
	dmgSystemLogFolder = "DmgSystemLog"     // Copy the dmg command output for DAOS system
	dmgNodeLogFolder   = "DmgNodeLog"       // Copy the dmg command output specific to the storage.
	daosAgentNodeLog   = "daosAgentNodeLog" // Copy the daos_agent command output specific to the node.
	systemInfo         = "SysInfo"          // Copy the system related information
	serverLogs         = "ServerLogs"       // Copy the server/conrol and helper logs
	daosConfig         = "ServerConfig"     // Copy the server config
)

type Params struct {
	Config       string
	Continue     bool
	Hostlist     string
	TargetFolder string
}

func getRunningConf(log logging.Logger) (string, bool) {
	_, err := exec.Command("bash", "-c", "pidof daos_engine").Output()
	if err != nil {
		log.Info("daos_engine is not running on server")
		return "", false
	}

	cmd := "ps -eo args | grep daos_engine | head -n 1 | grep -oP '(?<=-d )[^ ]*'"
	stdout, err := exec.Command("bash", "-c", cmd).Output()
	running_config := filepath.Join(strings.TrimSpace(string(stdout)), config.ConfigOut)

	return running_config, true
}

func getServerConf(log logging.Logger) (string, bool) {
	conf, err := getRunningConf(log)

	if err == true {
		return conf, err
	}

	// Return the default config
	serverConfig := config.DefaultServer()
	default_path := filepath.Join(serverConfig.SocketDir, config.ConfigOut)

	return default_path, err
}

func cpFile(src, dst string, log logging.Logger) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	log_file_name := filepath.Base(src)

	log.Debugf(" -- Copy File %s to %s\n", log_file_name, dst)

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
func IsHidden(filename string) bool {
	if filename[0:1] == "." {
		return true
	} else {
		return false
	}
	return false
}

func CopyServerConfig(src, dst string, log logging.Logger) error {
	err := cpFile(src, dst, log)
	if err != nil {
		return err
	}

	// Rename the file if it's hidden
	result := IsHidden(filepath.Base(src))
	if result == true {
		hiddenConf := filepath.Join(dst, filepath.Base(src))
		nonhiddenConf := filepath.Join(dst, filepath.Base(src)[1:])
		os.Rename(hiddenConf, nonhiddenConf)
	}

	return nil
}

func createFolder(target string, log logging.Logger) error {
	// Create the folder if it's not exist
	if _, err := os.Stat(target); os.IsNotExist(err) {
		log.Debugf("Log folder is not Exists, so creating folder %s", target)

		if err := os.MkdirAll(target, 0700); err != nil && !os.IsExist(err) {
			return errors.Wrapf(err, "failed to create log directory %s", target)
		}
	}

	return nil
}

func cpOutputToFile(cmd string, target string, log logging.Logger) (string, error) {
	// Run command and copy output to the file
	// executing as subshell enables pipes in cmd string
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Errorf("FAILED -- Error running command -- %s -- %s", cmd, out)
		return "", errors.Wrapf(err, "Error running command %s with %s", cmd, out)
	}

	log.Debugf("SUCCESS -- %s > %s ", cmd, target)
	cmd = strings.ReplaceAll(cmd, "/", "_")
	if err := ioutil.WriteFile(filepath.Join(target, cmd), out, 0644); err != nil {
		log.Errorf("FAILED -- To Write command -- %s -- %s", cmd, err)
		return "", errors.Wrapf(err, "failed to write %s", filepath.Join(target, cmd))
	}

	return string(out), nil
}

func ArchiveLogs(log logging.Logger, opts ...Params) error {
	var buf bytes.Buffer
	err := common.FolderCompress(opts[0].TargetFolder, &buf)
	if err != nil && opts[0].Continue == false {
		return err
	}

	// write to the the .tar.gzip
	tarFileName := fmt.Sprintf("%s.tar.gz", opts[0].TargetFolder)
	log.Debugf("Archiving the log folder %s", tarFileName)
	fileToWrite, err := os.OpenFile(tarFileName, os.O_CREATE|os.O_RDWR, os.FileMode(0755))
	if err != nil && opts[0].Continue == false {
		return err
	}
	_, err = io.Copy(fileToWrite, &buf)
	if err != nil && opts[0].Continue == false {
		return err
	}

	return nil
}

func CollectDmgSysteminfo(log logging.Logger, opts ...Params) error {
	targetDmgLog := filepath.Join(opts[0].TargetFolder, dmgSystemLogFolder)
	err := createFolder(targetDmgLog, log)
	if err != nil {
		return err
	}

	for _, dmgCommand := range control.DmgLogCollectCmd {
		dmgCommand = strings.Join([]string{dmgCommand, "-o", opts[0].Config}, " ")
		_, err = cpOutputToFile(dmgCommand, targetDmgLog, log)
		if err != nil {
			return err
		}
	}

	return nil
}

func createHostFolder(dst string, log logging.Logger) (string, error) {
	// Create the individual folder on each server
	hn, err := os.Hostname()
	if err != nil {
		return "", err
	}
	targetLocation := filepath.Join(dst, hn)
	err = createFolder(targetLocation, log)
	if err != nil {
		return "", err
	}

	return targetLocation, nil
}

func getSysNameFromQuery(configPath string, log logging.Logger) []string {
	var hostName []string

	cmd := strings.Join([]string{"dmg", "system", "query", "-v", "-o", configPath}, " ")
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		err = errors.Wrapf(err, "Error running command %s with %s", cmd, out)
	}
	temp := strings.Split(string(out), "\n")

	for _, v := range temp[2 : len(temp)-2] {
		hostName = append(hostName, strings.Fields(v)[3][1:])
	}

	return hostName
}

func CollectDmgNodeinfo(log logging.Logger, opts ...Params) error {
	// Get the Hostlist
	var hostNames []string
	var output string

	if len(opts[0].Hostlist) > 0 {
		hostNames = strings.Fields(opts[0].Hostlist)
	} else {
		hostNames = getSysNameFromQuery(opts[0].Config, log)
	}

	for _, hostName := range hostNames {
		// Copy all the devices information for each server
		dmgCommand := strings.Join([]string{control.DmgListDeviceCmd, "-o", opts[0].Config, "-l", hostName}, " ")
		targetDmgLog := filepath.Join(opts[0].TargetFolder, hostName, dmgNodeLogFolder)

		// Create the Folder if log location is not shared FS.
		err := createFolder(targetDmgLog, log)
		if err != nil && opts[0].Continue == false {
			return err
		}

		output, err = cpOutputToFile(dmgCommand, targetDmgLog, log)
		if err != nil && opts[0].Continue == false {
			return err
		}

		// Copy each device health from each server
		for _, v1 := range strings.Split(output, "\n") {
			if strings.Contains(v1, "UUID") {
				device := strings.Fields(v1)[0][5:]
				deviceHealthcmd := strings.Join([]string{
					control.DmgDeviceHealthCmd, "-u", device, "-l", hostName, "-o", opts[0].Config}, " ")
				output, err = cpOutputToFile(deviceHealthcmd, targetDmgLog, log)
				if err != nil && opts[0].Continue == false {
					return err
				}
			}
		}
	}

	return nil
}

func CollectClientLog(log logging.Logger, opts ...Params) error {
	targetLocation, err := createHostFolder(opts[0].TargetFolder, log)
	if err != nil {
		return err
	}

	// Collect daos_agent logs
	agentNodeLocation := filepath.Join(targetLocation, daosAgentNodeLog)
	err = createFolder(agentNodeLocation, log)
	if err != nil && opts[0].Continue == false {
		return err
	}
	for _, agentCommand := range control.DasoAgnetInfoCmd {
		_, err = cpOutputToFile(agentCommand, agentNodeLocation, log)
		if err != nil && opts[0].Continue == false {
			return err
		}
	}

	return nil
}

func CollectServerLog(log logging.Logger, opts ...Params) error {
	// Get the running daos_engine state and config from running process
	cfgPath, serverRunning := getServerConf(log)
	serverConfig := config.DefaultServer()
	continuCollect := false

	// Use the provided config in case of engines are down.
	if opts[0].Config != "" {
		cfgPath = opts[0].Config
	}

	if opts[0].Continue == true {
		continuCollect = true
	}

	serverConfig.SetPath(cfgPath)
	serverConfig.Load()
	log.Debugf(" -- Server Config File is %s", cfgPath)

	// Create the individual folder on each server
	targetLocation, err := createHostFolder(opts[0].TargetFolder, log)
	if err != nil && continuCollect == false {
		return err
	}

	// Copy server config file
	targetConfig := filepath.Join(targetLocation, daosConfig)
	err = createFolder(targetConfig, log)
	if err != nil && continuCollect == false {
		return err
	}
	err = CopyServerConfig(cfgPath, targetConfig, log)
	if err != nil && continuCollect == false {
		return err
	}

	// Copy all the log files for each engine
	targetServerLogs := filepath.Join(targetLocation, serverLogs)
	err = createFolder(targetServerLogs, log)
	if err != nil && continuCollect == false {
		return err
	}
	for i := range serverConfig.Engines {
		matches, _ := filepath.Glob(serverConfig.Engines[i].LogFile + "*")
		for _, logfile := range matches {
			err := cpFile(logfile, targetServerLogs, log)
			if err != nil && continuCollect == false {
				return err
			}
		}
	}

	// Copy DAOS Control log file
	err = cpFile(serverConfig.ControlLogFile, targetServerLogs, log)
	if err != nil && continuCollect == false {
		return err
	}

	// Copy DAOS Helper log file
	err = cpFile(serverConfig.HelperLogFile, targetServerLogs, log)
	if err != nil && continuCollect == false {
		return err
	}

	// Create the dmg specific folder for each server
	dmgNodeLocation := filepath.Join(targetLocation, dmgNodeLogFolder)
	err = createFolder(dmgNodeLocation, log)
	if err != nil && continuCollect == false {
		return err
	}

	// Copy daos_metrics log if server is still running
	if serverRunning == true {
		for i := range serverConfig.Engines {
			engineId := fmt.Sprintf("%d", i)
			daoscmd := strings.Join([]string{"daos_metrics", "-S", engineId}, " ")

			_, err = cpOutputToFile(daoscmd, dmgNodeLocation, log)
			if err != nil && continuCollect == false {
				return err
			}
		}
	}

	// Collect dump-topology output for each server
	hwlog := logging.NewCommandLineLogger()
	hwProv := hwprov.DefaultTopologyProvider(hwlog)
	topo, err := hwProv.GetTopology(context.Background())
	if err != nil && continuCollect == false {
		return err
	}
	f, err := os.Create(filepath.Join(dmgNodeLocation, "daos_server dump-topology"))
	if err != nil && continuCollect == false {
		return err
	}
	defer f.Close()
	hardware.PrintTopology(topo, f)

	// Collect system related information
	targetSysinfo := filepath.Join(targetLocation, systemInfo)
	err = createFolder(targetSysinfo, log)
	if err != nil && continuCollect == false {
		return err
	}
	for _, sysCommand := range control.SysInfoCmd {
		_, err = cpOutputToFile(sysCommand, targetSysinfo, log)
		if err != nil && continuCollect == false {
			return err
		}
	}

	return nil

}
