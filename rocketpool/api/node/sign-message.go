package node

import (
	"encoding/hex"
	"fmt"
	_ "time/tzdata"

	"github.com/urfave/cli"

	"github.com/Seb369888/smartnode/shared/services"
	"github.com/Seb369888/smartnode/shared/types/api"
	hexutils "github.com/Seb369888/smartnode/shared/utils/hex"
)

func signMessage(c *cli.Context, message string) (*api.NodeSignResponse, error) {
	w, err := services.GetWallet(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.NodeSignResponse{}
	signedBytes, err := w.SignMessage(message)
	if err != nil {
		return nil, fmt.Errorf("Error signing message [%s]: %w", message, err)
	}
	response.SignedData = hexutils.AddPrefix(hex.EncodeToString(signedBytes))

	// Return response
	return &response, nil

}
