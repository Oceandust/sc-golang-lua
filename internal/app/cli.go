package app

import (
	"os"

	"sc_cli/internal/defgraph"

	"github.com/alecthomas/kong"
)

type defgraphService interface {
	LoadSnapshot(repoRoot string, compiledRoot string) (*defgraph.Snapshot, error)
}

type App struct {
	defgraph defgraphService
}

type CLI struct {
	Snapshot       snapshotCmd       `cmd:"" help:"Snapshot commands."`
	Defs           defsCmd           `cmd:"" help:"Definition inspection and validation commands."`
	TPAKPack       tpakPackCmd       `cmd:"" name:"tpak-pack" help:"Pack a mirrored directory tree into a set of TPAK archives."`
	TPAKUnpack     tpakUnpackCmd     `cmd:"" name:"tpak-unpack" help:"Unpack a directory of TPAK archives into mirrored directories."`
	TPAKUnpackFile tpakUnpackFileCmd `cmd:"" name:"tpak-unpack-file" help:"Unpack a single TPAK archive into a directory."`
	TPAKInspect    tpakInspectCmd    `cmd:"" name:"tpak-inspect" help:"Inspect a single TPAK archive."`
	YUPRewrite     yupRewriteCmd     `cmd:"" name:"yup-rewrite" help:"Rewrite a Star Conflict yup manifest with updated file metadata."`
}

func Run() error {
	application := &App{
		defgraph: newDefgraphService(),
	}

	cli := CLI{}
	parser, err := kong.New(
		&cli,
		kong.Name("sc_cli"),
		kong.Description("Multi-tool CLI for Star Conflict."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	if err != nil {
		return err
	}

	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	return ctx.Run(application)
}
