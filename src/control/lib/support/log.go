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
	daosNodeLogFolder  = "DaosNodeLog"      // Copy the dmg command output specific to the storage.
	daosAgentNodeLog   = "DaosAgentNodeLog" // Copy the daos_agent command output specific to the node.
	systemInfo         = "SysInfo"          // Copy the system related information
	serverLogs         = "ServerLogs"       // Copy the server/conrol and helper logs
	clientLogs         = "ClientLogs"       // Copy the server/conrol and helper logs
	daosConfig         = "ServerConfig"     // Copy the server config
	customLogs         = "CustomeLogs"      // Copy the Custome logs
)

type Params struct {
	Config       string
	Stop         bool
	Hostlist     string
	TargetFolder string
	CustomLogs   string
	JsonOutput   bool
	LogFunction  string 
	LogCmd     	 string
	Options 	 string
}

type copy struct {
	Cmd     string
	Options string
}

func checkEngineState(log logging.Logger) bool {
	_, err := exec.Command("bash", "-c", "pidof daos_engine").Output()
	if err != nil {
		log.Info("daos_engine is not running on server")
		return false
	}

	return true
}

func getRunningConf(log logging.Logger) (string, error) {
	running_config := ""
	if checkEngineState(log) == true {
		cmd := "ps -eo args | grep daos_engine | head -n 1 | grep -oP '(?<=-d )[^ ]*'"
		stdout, err := exec.Command("bash", "-c", cmd).Output()
		if err != nil {
			return "", err
		}
		running_config = filepath.Join(strings.TrimSpace(string(stdout)), config.ConfigOut)
	}

	return running_config, nil
}

func getServerConf(log logging.Logger, opts ...Params) (string, error) {
	cfgPath, err := getRunningConf(log)

	if err != nil {
		return "", err
	}

	if cfgPath == "" {
		cfgPath = filepath.Join(config.DefaultServer().SocketDir, config.ConfigOut)
	}

	log.Debugf(" -- Server Config File is %s", cfgPath)	
	return cfgPath, nil
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

func cpOutputToFile(target string, log logging.Logger, cp ...copy) (string, error) {
	// Run command and copy output to the file
	// executing as subshell enables pipes in cmd string
	runCmd := strings.Join([]string{cp[0].Cmd, cp[0].Options}, " ")
	out, err := exec.Command("sh", "-c", runCmd).CombinedOutput()
	if err != nil {
		log.Errorf("FAILED -- Error running command -- %s -- %s", runCmd, out)
		return "", errors.Wrapf(err, "Error running command %s with %s", runCmd, out)
	}

	log.Debugf("SUCCESS -- %s > %s ", runCmd, target)
	cmd := strings.ReplaceAll(cp[0].Cmd, " -", "_")
	cmd = strings.ReplaceAll(cmd, " ", "_")
	if err := ioutil.WriteFile(filepath.Join(target, cmd), out, 0644); err != nil {
		log.Errorf("FAILED -- To Write command -- %s -- %s", cmd, err)
		return "", errors.Wrapf(err, "failed to write %s", filepath.Join(target, cmd))
	}

	return string(out), nil
}

func ArchiveLogs(log logging.Logger, opts ...Params) error {
	var buf bytes.Buffer
	err := common.FolderCompress(opts[0].TargetFolder, &buf)
	if err != nil && opts[0].Stop == true {
		return err
	}

	// write to the the .tar.gzip
	tarFileName := fmt.Sprintf("%s.tar.gz", opts[0].TargetFolder)
	log.Debugf("Archiving the log folder %s", tarFileName)
	fileToWrite, err := os.OpenFile(tarFileName, os.O_CREATE|os.O_RDWR, os.FileMode(0755))
	if err != nil && opts[0].Stop == true {
		return err
	}
	defer fileToWrite.Close()

	_, err = io.Copy(fileToWrite, &buf)
	if err != nil && opts[0].Stop == true {
		return err
	}

	return nil
}

func CollectDmgSysteminfo(log logging.Logger, opts ...Params) error {
	// log.Debug("Collecting Dmg Ouput")
	targetDmgLog := filepath.Join(opts[0].TargetFolder, dmgSystemLogFolder)
	err := createFolder(targetDmgLog, log)
	if err != nil {
		return err
	}

	dmg := copy{}
	for _, dmg.Cmd = range control.DmgLogCollectCmd {
		dmg.Options = strings.Join([]string{"-o", opts[0].Config}, " ")

		if opts[0].JsonOutput {
			dmg.Options = strings.Join([]string{dmg.Options, "-j"}, " ")
		}

		_, err = cpOutputToFile(targetDmgLog, log, dmg)
		if err != nil && opts[0].Stop == true {
			return err
		}
	}

	return nil
}

func createHostFolder(dst string, log logging.Logger) (string, error) {
	// Create the individual folder on each server
	hn, err := exec.Command("hostname", "-s").Output()
	if err != nil {
		return "", errors.Wrapf(err, "Error running hostname -s command %s", hn)
	}
	out := strings.Split(string(hn), "\n")

	targetLocation := filepath.Join(dst, out[0])
	err = createFolder(targetLocation, log)
	if err != nil {
		return "", err
	}

	return targetLocation, nil
}

func getSysNameFromQuery(configPath string, log logging.Logger) []string {
	var hostName []string

	dName, err := exec.Command("sh", "-c", "domainname").Output()
	if err != nil {
		err = errors.Wrapf(err, "Error running command domainname with %s", dName)
	}
	domainName := strings.Split(string(dName), "\n")

	cmd := strings.Join([]string{"dmg", "system", "query", "-v", "-o", configPath}, " ")
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		err = errors.Wrapf(err, "Error running command %s with %s", cmd, out)
	}
	temp := strings.Split(string(out), "\n")

	for _, hn := range temp[2 : len(temp)-2] {
		hn = strings.ReplaceAll(strings.Fields(hn)[3][1:], domainName[0], "")
		hn = strings.TrimSuffix(hn, ".")
		hostName = append(hostName, hn)
	}

	return hostName
}

func CollectCustomLogs(targetLocation string, log logging.Logger, opts ...Params) error {
	log.Infof("Log will be collected from custome location %s", opts[0].CustomLogs)

	customeLogFolder := filepath.Join(targetLocation, customLogs)
	err := createFolder(customeLogFolder, log)
	if err != nil && opts[0].Stop == true {
		return err
	}

	err = common.CpDir(opts[0].CustomLogs, customeLogFolder)
	if err != nil && opts[0].Stop == true {
		return err
	}

	return nil
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
		dmg := copy{}
		dmg.Cmd = control.DmgListDeviceCmd
		dmg.Options = strings.Join([]string{"-o", opts[0].Config, "-l", hostName}, " ")
		targetDmgLog := filepath.Join(opts[0].TargetFolder, hostName, daosNodeLogFolder)

		// Create the Folder if log location is not shared FS.
		err := createFolder(targetDmgLog, log)
		if err != nil && opts[0].Stop == true {
			return err
		}

		output, err = cpOutputToFile(targetDmgLog, log, dmg)
		if err != nil && opts[0].Stop == true {
			return err
		}

		// Copy each device health from each server
		for _, v1 := range strings.Split(output, "\n") {
			if strings.Contains(v1, "UUID") {
				device := strings.Fields(v1)[0][5:]
				health := copy{}
				health.Cmd = strings.Join([]string{control.DmgDeviceHealthCmd, "-u", device}, " ")
				health.Options = strings.Join([]string{"-l", hostName, "-o", opts[0].Config}, " ")
				_, err = cpOutputToFile(targetDmgLog, log, health)
				if err != nil && opts[0].Stop == true {
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

	// Collect daos_agent command output
	agentNodeLocation := filepath.Join(targetLocation, daosAgentNodeLog)
	err = createFolder(agentNodeLocation, log)
	if err != nil && opts[0].Stop == true {
		return err
	}

	agent := copy{}
	for _, agent.Cmd = range control.DasoAgnetInfoCmd {
		_, err = cpOutputToFile(agentNodeLocation, log, agent)
		if err != nil && opts[0].Stop == true {
			return err
		}
	}

	// Collect client side log
	clientLogFile := os.Getenv("D_LOG_FILE")
	if clientLogFile != "" {
		clientLogLocation := filepath.Join(targetLocation, clientLogs)
		err = createFolder(clientLogLocation, log)
		if err != nil && opts[0].Stop == true {
			return err
		}
		matches, _ := filepath.Glob(clientLogFile + "*")
		for _, logfile := range matches {
			err := cpFile(logfile, clientLogLocation, log)
			if err != nil && opts[0].Stop == true {
				return err
			}
		}
	}

	// Copy the custome log location
	if opts[0].CustomLogs != "" {
		err := CollectCustomLogs(targetLocation, log, opts...)
		if err != nil && opts[0].Stop == true {
			return err
		}
	}

	return nil
}

func CollectSystemLog(log logging.Logger, opts ...Params) error {
	
	targetLocation, err := createHostFolder(opts[0].TargetFolder, log)
	if err != nil {
		return err
	}

	// Collect system related information
	targetSysinfo := filepath.Join(targetLocation, systemInfo)
	err = createFolder(targetSysinfo, log)
	if err != nil {
		return err
	}

	system := copy{}
	system.Cmd = opts[0].LogCmd
	_, err = cpOutputToFile(targetSysinfo, log, system)
	if err != nil {
		return err
	}

	return nil
}

func CopyServerConfig(log logging.Logger, opts ...Params) error {
	cfgPath, err := getServerConf(log,  opts...)

	serverConfig := config.DefaultServer()
	serverConfig.SetPath(cfgPath)
	serverConfig.Load()
	// Create the individual folder on each server
	targetLocation, err := createHostFolder(opts[0].TargetFolder, log)
	if err != nil {
		return err
	}

	// Copy server config file
	targetConfig := filepath.Join(targetLocation, daosConfig)
	err = createFolder(targetConfig, log)
	if err != nil {
		return err
	}

	err = cpFile(cfgPath, targetConfig, log)
	if err != nil {
		return err
	}

	// Rename the file if it's hidden
	result := IsHidden(filepath.Base(cfgPath))
	if result == true {
		hiddenConf := filepath.Join(targetConfig, filepath.Base(cfgPath))
		nonhiddenConf := filepath.Join(targetConfig, filepath.Base(cfgPath)[1:])
		os.Rename(hiddenConf, nonhiddenConf)
	}

	return nil
}

func CollectServerLog(log logging.Logger, opts ...Params) error {
	// Get the running daos_engine state and config from running process
	cfgPath, _ := getServerConf(log)
	serverConfig := config.DefaultServer()
	stopOnFailure := false

	// Use the provided config in case of engines are down.
	if opts[0].Config != "" {
		cfgPath = opts[0].Config
	}

	if opts[0].Stop == true {
		stopOnFailure = true
	}

	serverConfig.SetPath(cfgPath)
	serverConfig.Load()
	log.Debugf(" -- Server Config File is %s", cfgPath)

	// Create the individual folder on each server
	targetLocation, err := createHostFolder(opts[0].TargetFolder, log)
	if err != nil && stopOnFailure == true {
		return err
	}

	// Copy the custome log location
	if opts[0].CustomLogs != "" {
		err := CollectCustomLogs(targetLocation, log, opts...)
		if err != nil && stopOnFailure == true {
			return err
		}
	}

	// Copy all the log files for each engine
	targetServerLogs := filepath.Join(targetLocation, serverLogs)
	err = createFolder(targetServerLogs, log)
	if err != nil && stopOnFailure == true {
		return err
	}
	for i := range serverConfig.Engines {
		matches, _ := filepath.Glob(serverConfig.Engines[i].LogFile + "*")
		for _, logfile := range matches {
			err := cpFile(logfile, targetServerLogs, log)
			if err != nil && stopOnFailure == true {
				return err
			}
		}
	}

	// Copy DAOS Control log file
	err = cpFile(serverConfig.ControlLogFile, targetServerLogs, log)
	if err != nil && stopOnFailure == true {
		return err
	}

	// Copy DAOS Helper log file
	err = cpFile(serverConfig.HelperLogFile, targetServerLogs, log)
	if err != nil && stopOnFailure == true {
		return err
	}

	// Create the dmg specific folder for each server
	daosNodeLocation := filepath.Join(targetLocation, daosNodeLogFolder)
	err = createFolder(daosNodeLocation, log)
	if err != nil && stopOnFailure == true {
		return err
	}

	// Copy daos_metrics log if server is still running
	daos := copy{}
	if checkEngineState(log) == true {
		for i := range serverConfig.Engines {
			engineId := fmt.Sprintf("%d", i)
			daos.Cmd = strings.Join([]string{"daos_metrics", "-S", engineId}, " ")

			_, err = cpOutputToFile(daosNodeLocation, log, daos)
			if err != nil && stopOnFailure == true {
				return err
			}
		}
	}

	// Collect dump-topology output for each server
	hwlog := logging.NewCommandLineLogger()
	hwProv := hwprov.DefaultTopologyProvider(hwlog)
	topo, err := hwProv.GetTopology(context.Background())
	if err != nil && stopOnFailure == true {
		return err
	}
	f, err := os.Create(filepath.Join(daosNodeLocation, "daos_server_dump-topology"))
	if err != nil && stopOnFailure == true {
		return err
	}
	defer f.Close()
	hardware.PrintTopology(topo, f)
	
	return nil
}

func CollectSupportLog (log logging.Logger, opts ...Params) error {
	switch  opts[0].LogFunction {
	case "CopyServerConfig":
		return CopyServerConfig(log , opts ...)
	case "CollectSystemLog":
		return CollectSystemLog(log , opts ...)
	}

	return nil
}