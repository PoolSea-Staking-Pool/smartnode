package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alessio/shellescape"
	"github.com/rivo/tview"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"

	"github.com/dustin/go-humanize"
	cliconfig "github.com/rocket-pool/smartnode/rocketpool-cli/service/config"
	"github.com/rocket-pool/smartnode/shared"
	"github.com/rocket-pool/smartnode/shared/services/config"
	"github.com/rocket-pool/smartnode/shared/services/rocketpool"
	cliutils "github.com/rocket-pool/smartnode/shared/utils/cli"
	"github.com/shirou/gopsutil/v3/disk"
)

// Settings
const (
	ExporterContainerSuffix         string = "_exporter"
	ValidatorContainerSuffix        string = "_validator"
	BeaconContainerSuffix           string = "_eth2"
	ExecutionContainerSuffix        string = "_eth1"
	NodeContainerSuffix             string = "_node"
	ApiContainerSuffix              string = "_api"
	PruneProvisionerContainerSuffix string = "_prune_provisioner"
	clientDataVolumeName            string = "/ethclient"
	dataFolderVolumeName            string = "/.rocketpool/data"

	PruneFreeSpaceRequired uint64 = 50 * 1024 * 1024 * 1024
	dockerImageRegex       string = ".*/(?P<image>.*):.*"
	colorReset             string = "\033[0m"
	colorBold              string = "\033[1m"
	colorRed               string = "\033[31m"
	colorYellow            string = "\033[33m"
	colorGreen             string = "\033[32m"
	colorLightBlue         string = "\033[36m"
	clearLine              string = "\033[2K"
)

// Install the Rocket Pool service
func installService(c *cli.Context) error {

	if c.String("network") != "" {
		fmt.Printf("%sNOTE: The --network flag is deprecated. You no longer need to specify it.%s\n\n", colorLightBlue, colorReset)
	}

	// Prompt for confirmation
	if !(c.Bool("yes") || cliutils.Confirm(fmt.Sprintf(
		"The Rocket Pool service will be installed --Version: %s\n\n%sIf you're upgrading, your existing configuration will be backed up and preserved.\nAll of your and modifications will be migrated automatically.%s\nAre you sure you want to continue?",
		c.String("version"), colorGreen, colorReset,
	))) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Install service
	err = rp.InstallService(c.Bool("verbose"), c.Bool("no-deps"), c.String("network"), c.String("version"), c.String("path"))
	if err != nil {
		return err
	}

	// Print success message & return
	fmt.Println("")
	fmt.Println("The Rocket Pool service was successfully installed!")

	printPatchNotes(c)

	// Load the config, which will upgrade it
	cfg, isNew, err := rp.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading new configuration: %w", err)
	}

	isMigration := false
	if isNew {
		// Look for a legacy config to migrate
		migratedConfig, err := rp.LoadLegacyConfigFromBackup()
		if err != nil {
			return err
		}
		if migratedConfig != nil {
			cfg = migratedConfig
			isMigration = true
		}
	}

	// Save the upgraded config
	err = rp.SaveConfig(cfg)
	if err != nil {
		return fmt.Errorf("error saving upgraded configuration: %w", err)
	}

	// Print the docker permissions notice
	if isNew && !isMigration {
		fmt.Printf("%sNOTE:\nSince this is your first time installing Rocket Pool, please start a new shell session by logging out and back in or restarting the machine.\n", colorYellow)
		fmt.Printf("This is necessary for your user account to have permissions to use Docker.%s", colorReset)
	}

	return nil

}

// Print the latest patch notes for this release
// TODO: get this from an external source and don't hardcode it into the CLI
func printPatchNotes(c *cli.Context) {

	fmt.Print(`
______           _        _    ______           _ 
| ___ \         | |      | |   | ___ \         | |
| |_/ /___   ___| | _____| |_  | |_/ /__   ___ | |
|    // _ \ / __| |/ / _ \ __| |  __/ _ \ / _ \| |
| |\ \ (_) | (__|   <  __/ |_  | | | (_) | (_) | |
\_| \_\___/ \___|_|\_\___|\__| \_|  \___/ \___/|_|

`)
	fmt.Printf("%s=== Smartnode v%s ===%s\n\n", colorGreen, shared.RocketPoolVersion, colorReset)
	fmt.Printf("Changes you should be aware of before starting:\n\n")

	fmt.Printf("%s=== New Configuration System ===%s\n", colorGreen, colorReset)
	fmt.Println("The Smartnode's configuration system has undergone some massive changes based on community feedback! Here are the updates:\n")
	fmt.Println("- The text-based `rocketpool service config` interview process, along with the old `config.yml` file, have been replaced with a shiny, easy-to-use new UI. You can now simply browse through and change any of the settings.`\n")
	fmt.Println("- All of your settings will now persist across Smartnode updates - you don't need to redo the changes anymore after updating!\n")
	fmt.Println("- First-class support for Hybrid mode (externally-managed clients)! No more need to mess with the Docker files.\n")
	fmt.Println("- Advanced users who customize their Docker compose files can now do so with special files in the `override` folder - these will replace any settings in the original Docker compose files, and will persist across updates so you only need to create them once.\n")

	fmt.Printf("%s=== Restoring from Backup ===%s\n", colorGreen, colorReset)
	fmt.Println("All of your previous configuration files and settings have been backed up. Please see <link I have yet to write> for a walkthrough of how to restore them if you need to revert to the previous version.")
}

// Install the Rocket Pool update tracker for the metrics dashboard
func installUpdateTracker(c *cli.Context) error {

	// Prompt for confirmation
	if !(c.Bool("yes") || cliutils.Confirm(
		"This will add the ability to display any available Operating System updates or new Rocket Pool versions on the metrics dashboard. "+
			"Are you sure you want to install the update tracker?")) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Get the container prefix
	prefix, err := getContainerPrefix(rp)
	if err != nil {
		return fmt.Errorf("Error getting validator container prefix: %w", err)
	}

	// Install service
	err = rp.InstallUpdateTracker(c.Bool("verbose"), c.String("version"))
	if err != nil {
		return err
	}

	// Print success message & return
	colorReset := "\033[0m"
	colorYellow := "\033[33m"
	fmt.Println("")
	fmt.Println("The Rocket Pool update tracker service was successfully installed!")
	fmt.Println("")
	fmt.Printf("%sNOTE:\nPlease run 'docker restart %s%s' to enable update tracking on the metrics dashboard.%s\n", colorYellow, prefix, ExporterContainerSuffix, colorReset)
	fmt.Println("")
	return nil

}

// View the Rocket Pool service status
func serviceStatus(c *cli.Context) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Print what network we're on
	err = cliutils.PrintNetwork(rp)
	if err != nil {
		return err
	}

	// Print service status
	return rp.PrintServiceStatus(getComposeFiles(c))

}

// Configure the service
func configureService(c *cli.Context) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Load the config, checking to see if it's new (hasn't been installed before)
	cfg, isNew, err := rp.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading user settings: %w", err)
	}

	// Check to see if there is an existing backup config from a previous version after an update
	var oldCfg *config.RocketPoolConfig
	isUpdate := rp.IsFirstRun()
	if isUpdate {
		oldCfg, err = rp.LoadBackupConfig()
		if err != nil {
			return fmt.Errorf("error loading user settings: %w", err)
		}
	}

	// Check to see if this is a migration from a legacy config
	isMigration := false
	if isNew {
		// Look for a legacy config to migrate
		migratedConfig, err := rp.LoadLegacyConfigFromBackup()
		if err != nil {
			return err
		}
		if migratedConfig != nil {
			oldCfg = migratedConfig
			isMigration = true
			cfg = oldCfg.CreateCopy()

			// Update the config
			err = cfg.UpdateDefaults()
			if err != nil {
				return fmt.Errorf("error updating legacy configuration to with new settings: %w", err)
			}
		}
	}

	// Save the config and exit in headless mode
	if c.Bool("headless") {
		return rp.SaveConfig(cfg)
	}

	app := tview.NewApplication()
	md := cliconfig.NewMainDisplay(app, oldCfg, cfg, isNew, isMigration, isUpdate)
	err = app.Run()
	if err != nil {
		return err
	}

	// Deal with saving the config and printing the changes
	if md.ShouldSave {
		// Remove the upgrade flag
		if oldCfg != nil {
			err = rp.RemoveUpgradeFlagFile()
			if err != nil {
				return fmt.Errorf("error removing upgrade flag file prior to saving: %w", err)
			}
		}

		// Save the config
		rp.SaveConfig(md.Config)
		fmt.Println("Your changes have been saved!")

		// Deal with network changes
		prefix := fmt.Sprint(md.PreviousConfig.Smartnode.ProjectName.Value)
		if md.ChangeNetworks {
			err = changeNetworks(c, rp, fmt.Sprintf("%s%s", prefix, ApiContainerSuffix))
			if err != nil {
				fmt.Printf("%s%s%s\nThe Smartnode could not automatically change networks for you, so you will have to run the steps manually. Please follow the steps laid out in the Node Operator's guide (https://docs.rocketpool.net/guides/node/mainnet.html).\n", colorRed, err.Error(), colorReset)
			}
			return nil
		}

		if len(md.ContainersToRestart) > 0 {
			fmt.Println("The following containers must be restarted for the changes to take effect:")
			for _, container := range md.ContainersToRestart {
				fmt.Printf("\t%s_%s\n", prefix, container)
			}

			fmt.Println()
			for _, container := range md.ContainersToRestart {
				fullName := fmt.Sprintf("%s_%s", prefix, container)
				fmt.Printf("Stopping %s... ", fullName)
				rp.StopContainer(fullName)
				fmt.Print("done!\n")
			}

			fmt.Println()
			fmt.Println("Applying changes and restarting containers...")
			return startService(c)
		}
	} else {
		fmt.Println("Your changes have not been saved. Your Smartnode configuration is the same as it was before.")
		return nil
	}

	return err
}

// Handle a network change by terminating the service, deleting everything, and starting over
func changeNetworks(c *cli.Context, rp *rocketpool.Client, apiContainerName string) error {

	// Stop all of the containers
	fmt.Print("Stopping containers... ")
	err := rp.PauseService(getComposeFiles(c))
	if err != nil {
		return fmt.Errorf("error stopping service: %w", err)
	}
	fmt.Println("done")

	// Restart the API container
	fmt.Print("Starting API container... ")
	output, err := rp.StartContainer(apiContainerName)
	if err != nil {
		return fmt.Errorf("error starting API container: %w", err)
	}
	if output != apiContainerName {
		return fmt.Errorf("starting API container had unexpected output: %s", output)
	}
	fmt.Println("done")

	// Get the path of the user's data folder
	fmt.Print("Retrieving data folder path... ")
	volumePath, err := rp.GetClientVolumeSource(apiContainerName, dataFolderVolumeName)
	if err != nil {
		return fmt.Errorf("error getting data folder path: %w", err)
	}
	fmt.Printf("done, data folder = %s\n", volumePath)

	// Delete the data folder
	fmt.Print("Removing data folder... ")
	_, err = rp.TerminateDataFolder()
	if err != nil {
		return err
	}
	fmt.Println("done")

	// Terminate the current setup
	fmt.Print("Removing old installation... ")
	err = rp.StopService(getComposeFiles(c))
	if err != nil {
		return fmt.Errorf("error terminating old installation: %w", err)
	}
	fmt.Println("done")

	// Create new validator folder
	fmt.Print("Recreating data folder... ")
	err = os.MkdirAll(filepath.Join(volumePath, "validators"), 0775)
	if err != nil {
		return fmt.Errorf("error recreating data folder: %w", err)
	}

	// Start the service
	fmt.Print("Starting Rocket Pool... ")
	err = rp.StartService(getComposeFiles(c))
	if err != nil {
		return fmt.Errorf("error starting service: %w", err)
	}
	fmt.Println("done")

	return nil

}

// Start the Rocket Pool service
func startService(c *cli.Context) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Update the Prometheus template with the assigned ports
	cfg, isNew, err := rp.LoadConfig()
	if err != nil {
		return fmt.Errorf("Error loading user settings: %w", err)
	}

	isMigration := false
	if isNew {
		// Look for a legacy config to migrate
		migratedConfig, err := rp.LoadLegacyConfigFromBackup()
		if err != nil {
			return err
		}
		if migratedConfig != nil {
			cfg = migratedConfig
			isMigration = true
		}
	}

	if isMigration {
		return fmt.Errorf("You must upgrade your configuration before starting the Smartnode. Please run `rocketpool service config` to confirm your settings were migrated correctly, and enjoy the new configuration UI!")
	} else if isNew {
		return fmt.Errorf("No configuration detected. Please run `rocketpool service config` to set up your Smartnode before running it.")
	}

	metricsEnabled := cfg.EnableMetrics.Value.(bool)
	if metricsEnabled {
		err := rp.UpdatePrometheusConfiguration(cfg.GenerateEnvironmentVariables())
		if err != nil {
			return err
		}
	}

	if !c.Bool("ignore-slash-timer") {
		// Do the client swap check
		err := checkForValidatorChange(rp, cfg)
		if err != nil {
			fmt.Printf("%sWarning: couldn't verify that the validator container can be safely restarted:\n\t%s\n", colorYellow, err.Error())
			fmt.Println("If you are changing to a different ETH2 client, it may resubmit an attestation you have already submitted.")
			fmt.Println("This will slash your validator!")
			fmt.Println("To prevent slashing, you must wait 15 minutes from the time you stopped the clients before starting them again.\n")
			fmt.Println("**If you did NOT change clients, you can safely ignore this warning.**\n")
			if !cliutils.Confirm(fmt.Sprintf("Press y when you understand the above warning, have waited, and are ready to start Rocket Pool:%s", colorReset)) {
				fmt.Println("Cancelled.")
				return nil
			}
		}
	} else {
		fmt.Printf("%sIgnoring anti-slashing safety delay.%s\n", colorYellow, colorReset)
	}

	// Start service
	return rp.StartService(getComposeFiles(c))

}

func checkForValidatorChange(rp *rocketpool.Client, cfg *config.RocketPoolConfig) error {

	// Get the container prefix
	prefix, err := getContainerPrefix(rp)
	if err != nil {
		return fmt.Errorf("Error getting validator container prefix: %w", err)
	}

	// Get the current validator client
	currentValidatorImageString, err := rp.GetDockerImage(prefix + ValidatorContainerSuffix)
	if err != nil {
		return fmt.Errorf("Error getting current validator image: %w", err)
	}

	currentValidatorName, err := getDockerImageName(currentValidatorImageString)
	if err != nil {
		return fmt.Errorf("Error getting current validator image name: %w", err)
	}

	// Get the new validator client according to the settings file
	selectedConsensusClientConfig, err := cfg.GetSelectedConsensusClientConfig()
	if err != nil {
		return fmt.Errorf("Error getting selected consensus client config: %w", err)
	}
	pendingValidatorName, err := getDockerImageName(selectedConsensusClientConfig.GetValidatorImage())
	if err != nil {
		return fmt.Errorf("Error getting pending validator image name: %w", err)
	}

	// Compare the clients and warn if necessary
	if currentValidatorName == pendingValidatorName {
		fmt.Printf("Validator client [%s] was previously used - no slashing prevention delay necessary.\n", currentValidatorName)
	} else if currentValidatorName == "" {
		fmt.Println("This is the first time starting Rocket Pool - no slashing prevention delay necessary.")
	} else {

		// Get the time that the container responsible for validator duties exited
		validatorDutyContainerName, err := getContainerNameForValidatorDuties(currentValidatorName, rp)
		if err != nil {
			return fmt.Errorf("Error getting validator container name: %w", err)
		}
		validatorFinishTime, err := rp.GetDockerContainerShutdownTime(validatorDutyContainerName)
		if err != nil {
			return fmt.Errorf("Error getting validator shutdown time: %w", err)
		}

		// If it hasn't exited yet, shut it down
		zeroTime := time.Time{}
		status, err := rp.GetDockerStatus(validatorDutyContainerName)
		if err != nil {
			return fmt.Errorf("Error getting container [%s] status: %w", validatorDutyContainerName, err)
		}
		if validatorFinishTime == zeroTime || status == "running" {
			fmt.Printf("%sValidator is currently running, stopping it...%s\n", colorYellow, colorReset)
			response, err := rp.StopContainer(validatorDutyContainerName)
			validatorFinishTime = time.Now()
			if err != nil {
				return fmt.Errorf("Error stopping container [%s]: %w", validatorDutyContainerName, err)
			}
			if response != validatorDutyContainerName {
				return fmt.Errorf("Unexpected response when stopping container [%s]: %s", validatorDutyContainerName, response)
			}
		}

		// Print the warning and start the time lockout
		safeStartTime := validatorFinishTime.Add(15 * time.Minute)
		remainingTime := time.Until(safeStartTime)
		if remainingTime <= 0 {
			fmt.Printf("The validator has been offline for %s, which is long enough to prevent slashing.\n", time.Since(validatorFinishTime))
			fmt.Println("The new client can be safely started.")
		} else {
			fmt.Printf("%s=== WARNING ===\n", colorRed)
			fmt.Printf("You have changed your validator client from %s to %s.\n", currentValidatorName, pendingValidatorName)
			fmt.Println("If you have active validators, starting the new client immediately will cause them to be slashed due to duplicate attestations!")
			fmt.Println("To prevent slashing, Rocket Pool will delay activating the new client for 15 minutes.")
			fmt.Printf("If you want to bypass this cooldown and understand the risks, run `rocketpool service start --ignore-slash-timer`.%s\n\n", colorReset)

			// Wait for 15 minutes
			for remainingTime > 0 {
				fmt.Printf("Remaining time: %s", remainingTime)
				time.Sleep(1 * time.Second)
				remainingTime = time.Until(safeStartTime)
				fmt.Printf("%s\r", clearLine)
			}

			fmt.Println(colorReset)
			fmt.Println("You may now safely start the validator without fear of being slashed.")
		}
	}

	return nil
}

// Get the name of the container responsible for validator duties based on the client name
// TODO: this is temporary and can change, clean it up when Nimbus supports split mode
func getContainerNameForValidatorDuties(CurrentValidatorClientName string, rp *rocketpool.Client) (string, error) {

	prefix, err := getContainerPrefix(rp)
	if err != nil {
		return "", err
	}

	if CurrentValidatorClientName == "nimbus" {
		return prefix + BeaconContainerSuffix, nil
	} else {
		return prefix + ValidatorContainerSuffix, nil
	}

}

// Get the time that the container responsible for validator duties exited
func getValidatorFinishTime(CurrentValidatorClientName string, rp *rocketpool.Client) (time.Time, error) {

	prefix, err := getContainerPrefix(rp)
	if err != nil {
		return time.Time{}, err
	}

	var validatorFinishTime time.Time
	if CurrentValidatorClientName == "nimbus" {
		validatorFinishTime, err = rp.GetDockerContainerShutdownTime(prefix + BeaconContainerSuffix)
	} else {
		validatorFinishTime, err = rp.GetDockerContainerShutdownTime(prefix + ValidatorContainerSuffix)
	}

	return validatorFinishTime, err

}

// Extract the image name from a Docker image string
func getDockerImageName(imageString string) (string, error) {

	// Return the empty string if the validator didn't exist (probably because this is the first time starting it up)
	if imageString == "" {
		return "", nil
	}

	reg := regexp.MustCompile(dockerImageRegex)
	matches := reg.FindStringSubmatch(imageString)
	if matches == nil {
		return "", fmt.Errorf("Couldn't parse the Docker image string [%s]", imageString)
	}
	imageIndex := reg.SubexpIndex("image")
	if imageIndex == -1 {
		return "", fmt.Errorf("Image name not found in Docker image [%s]", imageString)
	}

	imageName := matches[imageIndex]
	return imageName, nil
}

// Gets the prefix specified for Rocket Pool's Docker containers
func getContainerPrefix(rp *rocketpool.Client) (string, error) {

	cfg, isNew, err := rp.LoadConfig()
	if err != nil {
		return "", err
	}
	if isNew {
		return "", fmt.Errorf("Settings file not found. Please run `rocketpool service config` to set up your Smartnode.")
	}

	return cfg.Smartnode.ProjectName.Value.(string), nil
}

// Prepares the execution client for pruning
func pruneExecutionClient(c *cli.Context) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Get the config
	cfg, isNew, err := rp.LoadConfig()
	if err != nil {
		return err
	}
	if isNew {
		return fmt.Errorf("Settings file not found. Please run `rocketpool service config` to set up your Smartnode.")
	}

	fmt.Println("This will shut down your main ETH1 client and prune its database, freeing up disk space.")
	fmt.Println("Once pruning is complete, your ETH1 client will restart automatically.\n")

	if cfg.UseFallbackExecutionClient.Value == false {
		fmt.Printf("%sYou do not have a fallback ETH1 client configured.\nYou will continue attesting while ETH1 prunes, but block proposals and most of Rocket Pool's commands will not work.\nPlease configure a fallback client with `rocketpool service config` before running this.%s\n", colorRed, colorReset)
	} else {
		fmt.Printf("You have a fallback ETH1 client configured (%v). Rocket Pool (and your ETH2 client) will use that while the main client is pruning.\n", cfg.FallbackExecutionClient.Value.(config.ExecutionClient))
	}

	// Get the container prefix
	prefix, err := getContainerPrefix(rp)
	if err != nil {
		return fmt.Errorf("Error getting container prefix: %w", err)
	}

	// Prompt for stopping the node container if using Infura to prevent people from hitting the rate limit
	if cfg.FallbackExecutionClient.Value.(config.ExecutionClient) == config.ExecutionClient_Infura {
		fmt.Printf("\n%s=== NOTE ===\n\n", colorYellow)
		fmt.Printf("If you are using Infura's free tier, you may hit its rate limit if pruning takes a long time.\n")
		fmt.Printf("If this happens, you should temporarily disable the `%s` container until pruning is complete. This will:\n", prefix+NodeContainerSuffix)
		fmt.Println("\t- Stop collecting Rocket Pool's network metrics in the Grafana dashboard")
		fmt.Println("\t- Stop automatic operations (claiming RPL rewards and staking new minipools)\n")
		fmt.Printf("To disable the container, run: `docker stop %s`\n", prefix+NodeContainerSuffix)
		fmt.Printf("To re-enable the container one pruning is complete, run: `docker start %s`%s\n\n", prefix+NodeContainerSuffix, colorReset)
	}

	// Prompt for confirmation
	if !(c.Bool("yes") || cliutils.Confirm("Are you sure you want to prune your main ETH1 client?")) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Get the prune provisioner image
	pruneProvisioner := cfg.Smartnode.GetPruneProvisionerContainerTag()

	// Check for enough free space
	executionContainerName := prefix + ExecutionContainerSuffix
	volumePath, err := rp.GetClientVolumeSource(executionContainerName, clientDataVolumeName)
	if err != nil {
		return fmt.Errorf("Error getting ETH1 volume source path: %w", err)
	}
	partitions, err := disk.Partitions(false)
	if err != nil {
		return fmt.Errorf("Error getting partition list: %w", err)
	}

	longestPath := 0
	bestPartition := disk.PartitionStat{}
	for _, partition := range partitions {
		if strings.HasPrefix(volumePath, partition.Mountpoint) && len(partition.Mountpoint) > longestPath {
			bestPartition = partition
			longestPath = len(partition.Mountpoint)
		}
	}

	diskUsage, err := disk.Usage(bestPartition.Mountpoint)
	if err != nil {
		return fmt.Errorf("Error getting free disk space available: %w", err)
	}
	freeSpaceHuman := humanize.IBytes(diskUsage.Free)
	if diskUsage.Free < PruneFreeSpaceRequired {
		return fmt.Errorf("%sYour disk must have 50 GiB free to prune, but it only has %s free. Please free some space before pruning.%s", colorRed, freeSpaceHuman, colorReset)
	} else {
		fmt.Printf("Your disk has %s free, which is enough to prune.\n", freeSpaceHuman)
	}

	fmt.Printf("Stopping %s...\n", executionContainerName)
	result, err := rp.StopContainer(executionContainerName)
	if err != nil {
		return fmt.Errorf("Error stopping main ETH1 container: %w", err)
	}
	if result != executionContainerName {
		return fmt.Errorf("Unexpected output while stopping main ETH1 container: %s", result)
	}

	// Get the ETH1 volume name
	volume, err := rp.GetClientVolumeName(executionContainerName, clientDataVolumeName)
	if err != nil {
		return fmt.Errorf("Error getting ETH1 volume name: %w", err)
	}

	// Run the prune provisioner
	fmt.Printf("Provisioning pruning on volume %s...\n", volume)
	err = rp.RunPruneProvisioner(prefix+PruneProvisionerContainerSuffix, volume, pruneProvisioner)
	if err != nil {
		return fmt.Errorf("Error running prune provisioner: %w", err)
	}

	// Restart ETH1
	fmt.Printf("Restarting %s...\n", executionContainerName)
	result, err = rp.StartContainer(executionContainerName)
	if err != nil {
		return fmt.Errorf("Error starting main ETH1 container: %w", err)
	}
	if result != executionContainerName {
		return fmt.Errorf("Unexpected output while starting main ETH1 container: %s", result)
	}

	fmt.Printf("\nDone! Your main ETH1 client is now pruning. You can follow its progress with `rocketpool service logs eth1`.\n")
	fmt.Println("Once it's done, it will restart automatically and resume normal operation.")

	fmt.Printf("%sNOTE: While pruning, you **cannot** interrupt the client (e.g. by restarting) or you risk corrupting the database!\nYou must let it run to completion!%s\n", colorYellow, colorReset)

	return nil

}

// Pause the Rocket Pool service
func pauseService(c *cli.Context) error {

	// Prompt for confirmation
	if !(c.Bool("yes") || cliutils.Confirm("Are you sure you want to pause the Rocket Pool service? Any staking minipools will be penalized!")) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Pause service
	return rp.PauseService(getComposeFiles(c))

}

// Stop the Rocket Pool service
func stopService(c *cli.Context) error {

	// Prompt for confirmation
	if !(c.Bool("yes") || cliutils.Confirm("Are you sure you want to terminate the Rocket Pool service? Any staking minipools will be penalized, chain databases will be deleted, and ethereum nodes will lose ALL sync progress!")) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Stop service
	return rp.StopService(getComposeFiles(c))

}

// View the Rocket Pool service logs
func serviceLogs(c *cli.Context, serviceNames ...string) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Print service logs
	return rp.PrintServiceLogs(getComposeFiles(c), c.String("tail"), serviceNames...)

}

// View the Rocket Pool service stats
func serviceStats(c *cli.Context) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Print service stats
	return rp.PrintServiceStats(getComposeFiles(c))

}

// View the Rocket Pool service version information
func serviceVersion(c *cli.Context) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Print what network we're on
	err = cliutils.PrintNetwork(rp)
	if err != nil {
		return err
	}

	// Get RP service version
	serviceVersion, err := rp.GetServiceVersion()
	if err != nil {
		return err
	}

	// Get config
	cfg, isNew, err := rp.LoadConfig()
	if err != nil {
		return err
	}
	if isNew {
		return fmt.Errorf("Settings file not found. Please run `rocketpool service config` to set up your Smartnode.")
	}

	// Get the execution client string
	var eth1ClientString string
	eth1ClientMode := cfg.ExecutionClientMode.Value.(config.Mode)
	switch eth1ClientMode {
	case config.Mode_Local:
		eth1Client := cfg.ExecutionClient.Value.(config.ExecutionClient)
		switch eth1Client {
		case config.ExecutionClient_Geth:
			eth1ClientString = fmt.Sprintf("Geth (Locally managed)\n\tImage: %s", cfg.Geth.ContainerTag.Value.(string))
		case config.ExecutionClient_Infura:
			eth1ClientString = fmt.Sprintf("Infura (Locally managed)\n\tImage: %s", cfg.Smartnode.GetPowProxyContainerTag())
		case config.ExecutionClient_Pocket:
			eth1ClientString = fmt.Sprintf("Pocket (Locally managed)\n\tImage: %s", cfg.Smartnode.GetPowProxyContainerTag())
		default:
			return fmt.Errorf("unknown local execution client [%v]", eth1Client)
		}

	case config.Mode_External:
		eth1ClientString = "Externally managed"

	default:
		return fmt.Errorf("unknown execution client mode [%v]", eth1ClientMode)
	}

	// Get the consensus client string
	var eth2ClientString string
	eth2ClientMode := cfg.ConsensusClientMode.Value.(config.Mode)
	switch eth2ClientMode {
	case config.Mode_Local:
		eth2Client := cfg.ConsensusClient.Value.(config.ConsensusClient)
		switch eth2Client {
		case config.ConsensusClient_Lighthouse:
			eth2ClientString = fmt.Sprintf("Lighthouse (Locally managed)\n\tImage: %s", cfg.Lighthouse.ContainerTag.Value.(string))
		case config.ConsensusClient_Nimbus:
			eth2ClientString = fmt.Sprintf("Nimbus (Locally managed)\n\tImage: %s", cfg.Nimbus.ContainerTag.Value.(string))
		case config.ConsensusClient_Prysm:
			eth2ClientString = fmt.Sprintf("Prysm (Locally managed)\n\tBN image: %s\n\tVC image: %s", cfg.Prysm.BnContainerTag.Value.(string), cfg.Prysm.VcContainerTag.Value.(string))
		case config.ConsensusClient_Teku:
			eth2ClientString = fmt.Sprintf("Teku (Locally managed)\n\tImage: %s", cfg.Teku.ContainerTag.Value.(string))
		default:
			return fmt.Errorf("unknown local consensus client [%v]", eth2Client)
		}

	case config.Mode_External:
		eth2Client := cfg.ExternalConsensusClient.Value.(config.ConsensusClient)
		switch eth2Client {
		case config.ConsensusClient_Lighthouse:
			eth2ClientString = fmt.Sprintf("Lighthouse (Externally managed)\n\tVC Image: %s", cfg.ExternalLighthouse.ContainerTag.Value.(string))
		case config.ConsensusClient_Prysm:
			eth2ClientString = fmt.Sprintf("Prysm (Externally managed)\n\tVC image: %s", cfg.ExternalPrysm.ContainerTag.Value.(string))
		case config.ConsensusClient_Teku:
			eth2ClientString = fmt.Sprintf("Teku (Locally managed)\n\tImage: %s", cfg.ExternalTeku.ContainerTag.Value.(string))
		default:
			return fmt.Errorf("unknown external consensus client [%v]", eth2Client)
		}

	default:
		return fmt.Errorf("unknown consensus client mode [%v]", eth2ClientMode)
	}

	// Print version info
	fmt.Printf("Rocket Pool client version: %s\n", c.App.Version)
	fmt.Printf("Rocket Pool service version: %s\n", serviceVersion)
	fmt.Printf("Selected Eth 1.0 client: %s\n", eth1ClientString)
	fmt.Printf("Selected Eth 2.0 client: %s\n", eth2ClientString)
	return nil

}

// Get the compose file paths for a CLI context
func getComposeFiles(c *cli.Context) []string {
	return c.Parent().StringSlice("compose-file")
}

// Destroy and resync the eth1 client from scratch
func resyncEth1(c *cli.Context) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Get the config
	cfg, isNew, err := rp.LoadConfig()
	if err != nil {
		return err
	}
	if isNew {
		return fmt.Errorf("Settings file not found. Please run `rocketpool service config` to set up your Smartnode.")
	}

	fmt.Println("This will delete the chain data of your primary ETH1 client and resync it from scratch.")
	fmt.Printf("%sYou should only do this if your ETH1 client has failed and can no longer start or sync properly.\nThis is meant to be a last resort.%s\n", colorYellow, colorReset)

	if cfg.UseFallbackExecutionClient.Value == false {
		fmt.Printf("%sYou do not have a fallback ETH1 client configured.\nPlease configure a fallback client with `rocketpool service config` before running this.%s\n", colorRed, colorReset)
		return nil
	} else {
		fmt.Printf("You have a fallback ETH1 client configured (%v). Rocket Pool (and your ETH2 client) will use that while the main client is resyncing.\n", cfg.FallbackExecutionClient.Value.(config.ExecutionClient))
	}

	// Get the container prefix
	prefix, err := getContainerPrefix(rp)
	if err != nil {
		return fmt.Errorf("Error getting container prefix: %w", err)
	}

	// Prompt for stopping the node container if using Infura to prevent people from hitting the rate limit
	if cfg.FallbackExecutionClient.Value.(config.ExecutionClient) == config.ExecutionClient_Infura {
		fmt.Printf("\n%s=== NOTE ===\n\n", colorYellow)
		fmt.Printf("If you are using Infura's free tier, you will very likely hit its rate limit while resyncing.\n")
		fmt.Printf("You should temporarily disable the `%s` container until resyncing is complete. This will:\n", prefix+NodeContainerSuffix)
		fmt.Println("\t- Stop collecting Rocket Pool's network metrics in the Grafana dashboard")
		fmt.Println("\t- Stop automatic operations (claiming RPL rewards and staking new minipools)\n")
		fmt.Printf("To disable the container, run: `docker stop %s`\n", prefix+NodeContainerSuffix)
		fmt.Printf("To re-enable the container one resyncing is complete, run: `docker start %s`%s\n\n", prefix+NodeContainerSuffix, colorReset)
	}

	// Prompt for confirmation
	if !(c.Bool("yes") || cliutils.Confirm(fmt.Sprintf("%sAre you SURE you want to delete and resync your main ETH1 client from scratch? This cannot be undone!%s", colorRed, colorReset))) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Stop ETH1
	executionContainerName := prefix + ExecutionContainerSuffix
	fmt.Printf("Stopping %s...\n", executionContainerName)
	result, err := rp.StopContainer(executionContainerName)
	if err != nil {
		fmt.Printf("%sWARNING: Stopping main ETH1 container failed: %s%s\n", colorYellow, err.Error(), colorReset)
	}
	if result != executionContainerName {
		fmt.Printf("%sWARNING: Unexpected output while stopping main ETH1 container: %s%s\n", colorYellow, result, colorReset)
	}

	// Get ETH1 volume name
	volume, err := rp.GetClientVolumeName(executionContainerName, clientDataVolumeName)
	if err != nil {
		return fmt.Errorf("Error getting ETH1 volume name: %w", err)
	}

	// Remove ETH1
	fmt.Printf("Deleting %s...\n", executionContainerName)
	result, err = rp.RemoveContainer(executionContainerName)
	if err != nil {
		return fmt.Errorf("Error deleting main ETH1 container: %w", err)
	}
	if result != executionContainerName {
		return fmt.Errorf("Unexpected output while deleting main ETH1 container: %s", result)
	}

	// Delete the ETH1 volume
	fmt.Printf("Deleting volume %s...\n", volume)
	result, err = rp.DeleteVolume(volume)
	if err != nil {
		return fmt.Errorf("Error deleting volume: %w", err)
	}
	if result != volume {
		return fmt.Errorf("Unexpected output while deleting volume: %s", result)
	}

	// Restart Rocket Pool
	fmt.Printf("Rebuilding %s and restarting Rocket Pool...\n", executionContainerName)
	err = startService(c)
	if err != nil {
		return fmt.Errorf("Error starting Rocket Pool: %s", err)
	}

	fmt.Printf("\nDone! Your main ETH1 client is now resyncing. You can follow its progress with `rocketpool service logs eth1`.\n")

	return nil

}

// Destroy and resync the eth2 client from scratch
func resyncEth2(c *cli.Context) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Get the merged config
	cfg, isNew, err := rp.LoadConfig()
	if err != nil {
		return err
	}
	if isNew {
		return fmt.Errorf("Settings file not found. Please run `rocketpool service config` to set up your Smartnode.")
	}

	fmt.Println("This will delete the chain data of your ETH2 client and resync it from scratch.")
	fmt.Printf("%sYou should only do this if your ETH2 client has failed and can no longer start or sync properly.\nThis is meant to be a last resort.%s\n\n", colorYellow, colorReset)

	// Get the parameters that the selected client doesn't support
	var unsupportedParams []string
	var clientName string
	eth2ClientMode := cfg.ConsensusClientMode.Value.(config.Mode)
	switch eth2ClientMode {
	case config.Mode_Local:
		selectedClientConfig, err := cfg.GetSelectedConsensusClientConfig()
		if err != nil {
			return fmt.Errorf("error getting selected consensus client config: %w", err)
		}
		unsupportedParams = selectedClientConfig.(config.LocalConsensusConfig).GetUnsupportedCommonParams()
		clientName = selectedClientConfig.GetName()

	case config.Mode_External:
		fmt.Println("You use an externally-managed Consensus client. Rocket Pool cannot resync it for you.")
		return nil

	default:
		return fmt.Errorf("unknown consensus client mode [%v]", eth2ClientMode)
	}

	// Check if the selected client supports checkpoint sync
	supportsCheckpointSync := true
	for _, param := range unsupportedParams {
		if param == config.CheckpointSyncUrlID {
			supportsCheckpointSync = false
		}
	}
	if !supportsCheckpointSync {
		fmt.Printf("%sYour ETH2 client (%s) does not support checkpoint sync.\nIf you have active validators, they %swill be considered offline and will leak ETH%s%s while the client is syncing.%s\n\n", colorRed, clientName, colorBold, colorReset, colorRed, colorReset)
	} else {
		// Get the current checkpoint sync URL
		checkpointSyncUrl := cfg.ConsensusCommon.CheckpointSyncProvider.Value.(string)
		if checkpointSyncUrl == "" {
			fmt.Printf("%sYou do not have a checkpoint sync provider configured.\nIf you have active validators, they %swill be considered offline and will lose ETH%s%s until your ETH2 client finishes syncing.\nWe strongly recommend you configure a checkpoint sync provider with `rocketpool service config` so it syncs instantly before running this.%s\n\n", colorRed, colorBold, colorReset, colorRed, colorReset)
		} else {
			fmt.Printf("You have a checkpoint sync provider configured (%s).\nYour ETH2 client will use it to sync to the head of the Beacon Chain instantly after being rebuilt.\n\n", checkpointSyncUrl)
		}
	}

	// Get the container prefix
	prefix, err := getContainerPrefix(rp)
	if err != nil {
		return fmt.Errorf("Error getting container prefix: %w", err)
	}

	// Prompt for confirmation
	if !(c.Bool("yes") || cliutils.Confirm(fmt.Sprintf("%sAre you SURE you want to delete and resync your main ETH2 client from scratch? This cannot be undone!%s", colorRed, colorReset))) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Stop ETH2
	beaconContainerName := prefix + BeaconContainerSuffix
	fmt.Printf("Stopping %s...\n", beaconContainerName)
	result, err := rp.StopContainer(beaconContainerName)
	if err != nil {
		fmt.Printf("%sWARNING: Stopping ETH2 container failed: %s%s\n", colorYellow, err.Error(), colorReset)
	}
	if result != beaconContainerName {
		fmt.Printf("%sWARNING: Unexpected output while stopping ETH2 container: %s%s\n", colorYellow, result, colorReset)
	}

	// Get ETH2 volume name
	volume, err := rp.GetClientVolumeName(beaconContainerName, clientDataVolumeName)
	if err != nil {
		return fmt.Errorf("Error getting ETH2 volume name: %w", err)
	}

	// Remove ETH2
	fmt.Printf("Deleting %s...\n", beaconContainerName)
	result, err = rp.RemoveContainer(beaconContainerName)
	if err != nil {
		return fmt.Errorf("Error deleting ETH2 container: %w", err)
	}
	if result != beaconContainerName {
		return fmt.Errorf("Unexpected output while deleting ETH2 container: %s", result)
	}

	// Delete the ETH2 volume
	fmt.Printf("Deleting volume %s...\n", volume)
	result, err = rp.DeleteVolume(volume)
	if err != nil {
		return fmt.Errorf("Error deleting volume: %w", err)
	}
	if result != volume {
		return fmt.Errorf("Unexpected output while deleting volume: %s", result)
	}

	// Restart Rocket Pool
	fmt.Printf("Rebuilding %s and restarting Rocket Pool...\n", beaconContainerName)
	err = startService(c)
	if err != nil {
		return fmt.Errorf("Error starting Rocket Pool: %s", err)
	}

	fmt.Printf("\nDone! Your ETH2 client is now resyncing. You can follow its progress with `rocketpool service logs eth2`.\n")

	return nil

}

// Migrate a legacy configuration to a new one
func migrateConfig(c *cli.Context, oldConfig string, oldSettings string, newConfig string) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	newCfg, err := rp.MigrateLegacyConfig(oldConfig, oldSettings)
	if err != nil {
		return err
	}

	settings := newCfg.Serialize()
	configBytes, err := yaml.Marshal(settings)
	if err != nil {
		return fmt.Errorf("could not serialize settings file: %w", err)
	}

	if err := ioutil.WriteFile(newConfig, configBytes, 0664); err != nil {
		return fmt.Errorf("could not write Rocket Pool config to %s: %w", shellescape.Quote(newConfig), err)
	}

	return nil
}
