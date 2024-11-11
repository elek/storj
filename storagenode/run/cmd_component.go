// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"storj.io/common/cfgstruct"
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
)

// newExecCmd creates a new exec command.
func newComponentCmd(f *Factory, ball *mud.Ball) *cobra.Command {
	selector := modular.CreateSelector(ball)
	cmd := &cobra.Command{
		Use:   "components",
		Short: "list activated / available components",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			return cmdComponent(ctx, ball, selector)
		},
	}

	err := config.BindAll(context.Background(), cmd, ball, selector, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))
	if err != nil {
		panic(err)
	}

	return cmd
}

func cmdComponent(ctx context.Context, ball *mud.Ball, selector mud.ComponentSelector) error {
	for _, c := range mud.Find(ball, selector) {
		fmt.Println(c.Name())
	}
	return nil
}
