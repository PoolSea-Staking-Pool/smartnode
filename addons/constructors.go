package addons

import (
	"github.com/Seb369888/smartnode/addons/graffiti_wall_writer"
	"github.com/Seb369888/smartnode/shared/types/addons"
)

func NewGraffitiWallWriter() addons.SmartnodeAddon {
	return graffiti_wall_writer.NewGraffitiWallWriter()
}
