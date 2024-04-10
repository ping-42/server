package cmd

import (
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/ping-42/42lib/db/migrations"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// opts defines and handles the CLI parameters
type opts struct {
	Port    string `short:"p" long:"port" default:"8080" description:"Port to listen for sensor connections"`
	Migrate bool   `short:"m" long:"migrate" description:"Run database migrations and exit"`
}

var Flags opts
var Parser = flags.NewParser(&Flags, flags.Default)

// HandleOpts contains what's needed for handling the cli args
type HandleOpts struct {
	DbClient *gorm.DB
	Logger   *logrus.Entry
}

// Handle will setup all command line arguments
func (f *opts) Handle(opts HandleOpts) {
	if f.Migrate {
		migrations.MigrateAndSeed(opts.DbClient)
		opts.Logger.Info("Migrations DONE")
		os.Exit(0)
	}

	if !strings.HasPrefix(f.Port, ":") {
		f.Port = ":" + f.Port
	}
}
