package modules

import (
	"github.com/spf13/cobra"
	"storj.io/private/cfgstruct"
)

var DynamicConfigs []ConfigBinder

type ConfigBinder func(cmd *cobra.Command, opts ...cfgstruct.BindOpt)
