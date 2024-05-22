package cmd

import (
	"os"
	"strings"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jessevdk/go-flags"
	"github.com/ping-42/42lib/db/migrations"
	"github.com/ping-42/42lib/db/models"
	"github.com/ping-42/42lib/sensor"
	"github.com/ping-42/server/wsServer"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Define a struct for the 'run' command options
type RunOptions struct {
	Port string `short:"p" long:"port" default:"8080" description:"Port to listen for sensor connections"`
}

// Define a struct for the 'mksensor' command options
type CreateNewSensorOptions struct {
	Name     string `short:"n" long:"name" description:"The new sensor name" required:"true"`
	Location string `short:"l" long:"location" description:"The new sensor location" required:"true"`
}
type MigrateOptions struct{}

// opts defines and handles the CLI parameters
type opts struct {
	Run             RunOptions             `command:"run" description:"Run telemetry server" required:"false"`
	Migrate         MigrateOptions         `command:"migrate" description:"Run database migrations and exit" required:"false"`
	CreateNewSensor CreateNewSensorOptions `command:"mksensor" description:"Create new sensor" required:"false"`
}

var Flags opts
var Parser = flags.NewParser(&Flags, flags.Default)

// HandleOpts contains what's needed for handling the cli args
type HandleOpts struct {
	DbClient    *gorm.DB
	RedisClient *redis.Client
	Logger      *logrus.Entry
}

// Handle will setup all command line arguments
func (f *opts) Handle(opts HandleOpts) {
	if Parser.Command.Active == nil {
		opts.Logger.Infof("nothing to do")
		return
	}

	switch Parser.Command.Active.Name {
	case "run":
		handleServerRun(&f.Run, opts)
		os.Exit(0)
	case "migrate":
		handleMigrate(&f.Migrate, opts)
		os.Exit(0)
	case "mksensor":
		handleBuildNewSensor(&f.CreateNewSensor, opts)
		os.Exit(0)
	}
}

// Function to handle logic for the 'run' command
func handleServerRun(buildUserOpts *RunOptions, opts HandleOpts) {
	if !strings.HasPrefix(buildUserOpts.Port, ":") {
		buildUserOpts.Port = ":" + buildUserOpts.Port
	}
	server.Init(opts.DbClient, opts.RedisClient, opts.Logger, buildUserOpts.Port)
}

// Function to handle logic for the 'migrate' command
func handleMigrate(buildUserOpts *MigrateOptions, opts HandleOpts) {
	migrations.MigrateAndSeed(opts.DbClient)
	opts.Logger.Info("Migrations DONE")
	os.Exit(0)
}

// Function to handle logic for the 'CreateNewSensor' command
func handleBuildNewSensor(buildUserOpts *CreateNewSensorOptions, opts HandleOpts) {
	// Insert the new Sensor
	newSensor := models.Sensor{
		ID:       uuid.New(),
		Name:     buildUserOpts.Name,
		Location: buildUserOpts.Location,
		Secret:   uuid.New().String(),
	}
	tx := opts.DbClient.Create(&newSensor)
	if tx.Error != nil {
		opts.Logger.Errorf("creating newSensor err:%v", tx.Error)
		return
	}

	sensorCreds := sensor.Creds{
		SensorId: newSensor.ID,
		Secret:   newSensor.Secret,
	}
	envToken, err := sensorCreds.GetSensorEnvToken()
	if err != nil {
		opts.Logger.Errorf("GetSensorEnvToken err:%v", err)
		return
	}

	opts.Logger.Infof("new sensor Id:%v", newSensor.ID)
	opts.Logger.Infof("new sensor Name:%v", newSensor.Name)
	opts.Logger.Infof("new sensor Location:%v", newSensor.Location)
	opts.Logger.Infof("new sensor EnvToken:%v", envToken)
}
