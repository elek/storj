// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package internalcmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/shared/process"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/pieces/lazyfilewalker/execwrapper"
	"storj.io/storj/storagenode/storagenodedb"
)

// FilewalkerCfg is the config structure for the lazyfilewalker commands.
type FilewalkerCfg struct {
	lazyfilewalker.Config
}

// DatabaseConfig returns the storagenodedb.Config that should be used with this lazyfilewalker.
func (config *FilewalkerCfg) DatabaseConfig() storagenodedb.Config {
	return storagenodedb.Config{
		Storage:   config.Storage,
		Info:      config.Info,
		Info2:     config.Info2,
		Pieces:    config.Pieces,
		Filestore: config.Filestore,
		Driver:    config.Driver,
	}
}

// RunOptions defines the options for the lazyfilewalker runners.
type RunOptions struct {
	Ctx    context.Context
	Logger *zap.Logger
	config *FilewalkerCfg

	stdin  io.Reader
	stderr io.Writer
	stdout io.Writer
}

// LazyFilewalkerCmd is a wrapper for the lazyfilewalker commands.
type LazyFilewalkerCmd struct {
	Command *cobra.Command
	*RunOptions
}

var _ execwrapper.Command = (*LazyFilewalkerCmd)(nil)

// SetArgs sets arguments for the command.
// The command or executable path should be passed as the first argument.
func (cmd *LazyFilewalkerCmd) SetArgs(args []string) {
	if len(args) > 0 {
		// arg[0] is the command name or executable path, which we don't need
		// args[1] is the lazyfilewalker subcommand.
		args = args[2:]
	}
	cmd.Command.SetArgs(args)
}

// Run runs the LazyFileWalker.
func (cmd *LazyFilewalkerCmd) Run() error {
	return cmd.Command.ExecuteContext(cmd.Ctx)
}

// Start starts the LazyFileWalker command, assuming it behaves like the Start method on exec.Cmd.
// This is a no-op and only exists to satisfy the execwrapper.Command interface.
// Wait must be called to actually run the command.
func (cmd *LazyFilewalkerCmd) Start() error {
	return nil
}

// Wait waits for the LazyFileWalker to finish, assuming it behaves like the Wait method on exec.Cmd.
func (cmd *LazyFilewalkerCmd) Wait() error {
	return cmd.Run()
}

func (r *RunOptions) normalize(cmd *cobra.Command) {
	if r.Ctx == nil {
		ctx, _ := process.Ctx(cmd)
		r.Ctx = ctx
	}

	if r.stdin == nil {
		r.SetIn(os.Stdin)
	}

	if r.stdout == nil {
		r.SetOut(os.Stdout)
	}

	if r.stderr == nil {
		r.SetErr(os.Stderr)
	}

	if r.Logger == nil {
		r.Logger = zap.L()
	}
}

// SetIn sets the stdin reader.
func (r *RunOptions) SetIn(reader io.Reader) {
	r.stdin = reader
}

// SetOut sets the stdout writer.
func (r *RunOptions) SetOut(writer io.Writer) {
	r.stdout = writer
}

// SetErr sets the stderr writer.
func (r *RunOptions) SetErr(writer io.Writer) {
	r.stderr = writer
	r.tryCreateNewLogger()
}

func (r *RunOptions) tryCreateNewLogger() {
	// If the writer is os.Stderr, we don't need to register it because the stderr
	// writer is registered by default.
	if r.stderr == os.Stderr {
		return
	}
	writerkey := "zapwriter"

	err := zap.RegisterSink(writerkey, func(u *url.URL) (zap.Sink, error) {
		return nopWriterSyncCloser{r.stderr}, nil
	})

	// this error is expected if the sink is already registered.
	duplicateSinkErr := fmt.Errorf("sink factory already registered for scheme %q", writerkey)
	if err != nil && err.Error() != duplicateSinkErr.Error() {
		r.Logger.Error("failed to register logger sink", zap.Error(err))
		return
	}

	err = flag.Set("log.encoding", "json")
	if err != nil {
		r.Logger.Error("failed to set log encoding", zap.Error(err))
		return
	}

	// create a new logger with the writer as the output path.
	path := fmt.Sprintf("%s:subprocess", writerkey)
	logger, err := process.NewLoggerWithOutputPaths("lazyfilewalker", path)
	if err != nil {
		r.Logger.Error("failed to create logger", zap.Error(err))
		return
	}

	// set the logger to the new logger.
	r.Logger = logger
}

type nopWriterSyncCloser struct {
	io.Writer
}

func (cw nopWriterSyncCloser) Close() error { return nil }
func (cw nopWriterSyncCloser) Sync() error  { return nil }
