package rocketpool

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/Seb369888/smartnode/shared/services/config"
	"github.com/ethereum/go-ethereum/common"
)

// Config
const (
	FileMode fs.FileMode = 0644
)

// Checks if the fee recipient file exists and has the correct distributor address in it.
// The first return value is for file existence, the second is for validation of the fee recipient address inside.
func CheckFeeRecipientFile(feeRecipient common.Address, cfg *config.RocketPoolConfig) (bool, bool, error) {

	// Check if the file exists
	path := cfg.Smartnode.GetFeeRecipientFilePath()
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, false, nil
	} else if err != nil {
		return false, false, err
	}

	// Compare the file contents with the expected string
	expectedString := getFeeRecipientFileContents(feeRecipient, cfg)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return false, false, fmt.Errorf("error reading fee recipient file: %w", err)
	}
	existingString := string(bytes)
	if existingString != expectedString {
		// If it wrote properly, indicate a success but that the file needed to be updated
		return true, false, nil
	}

	// The file existed and had the expected address, all set.
	return true, true, nil
}

// Writes the given address to the fee recipient file. The VC should be restarted to pick up the new file.
func UpdateFeeRecipientFile(feeRecipient common.Address, cfg *config.RocketPoolConfig) error {

	// Create the distributor address string for the node
	expectedString := getFeeRecipientFileContents(feeRecipient, cfg)
	bytes := []byte(expectedString)

	// Write the file
	path := cfg.Smartnode.GetFeeRecipientFilePath()
	err := os.WriteFile(path, bytes, FileMode)
	if err != nil {
		return fmt.Errorf("error writing fee recipient file: %w", err)
	}
	return nil

}

// Gets the expected contents of the fee recipient file
func getFeeRecipientFileContents(feeRecipient common.Address, cfg *config.RocketPoolConfig) string {
	if !cfg.IsNativeMode {
		// Docker mode
		return feeRecipient.Hex()
	}

	// Native mode
	return fmt.Sprintf("%s=%s", config.FeeRecipientEnvVar, feeRecipient.Hex())
}
