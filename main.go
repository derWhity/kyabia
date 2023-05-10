package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/daemon"
	kyabia "github.com/derWhity/kyabia/internal"
	"github.com/derWhity/kyabia/internal/ctxhelper"
	"github.com/derWhity/kyabia/internal/log"
	"github.com/derWhity/kyabia/internal/migrate"
	"github.com/derWhity/kyabia/internal/models"
	eventrepo "github.com/derWhity/kyabia/internal/repos/event/sqlite"
	plrepo "github.com/derWhity/kyabia/internal/repos/playlist/sqlite"
	sessionrepo "github.com/derWhity/kyabia/internal/repos/session/inmem"
	userrepo "github.com/derWhity/kyabia/internal/repos/user/inmem"
	vidrepo "github.com/derWhity/kyabia/internal/repos/video/sqlite"
	"github.com/derWhity/kyabia/internal/scraper"
	"github.com/jmoiron/sqlx"
	"github.com/kardianos/osext"
	_ "github.com/mattn/go-sqlite3" // Just needed for the sqlite driver
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	appName    = "Kyabia"
	appVersion = "0.0.1"
	dbFile     = "kyabia.db"
)

// Checks and tries to create the given directory recursively (or panics if this fails)
func checkAndCreateDir(path string, logger *logrus.Entry) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if e, ok := err.(*os.PathError); ok && e.Err == syscall.ENOENT {
			logger.WithField(log.FldPath, path).Info("Directory does not exist - trying to create...")
			if err = os.MkdirAll(path, os.ModePerm); err != nil {
				logger.WithError(err).Fatal("Failed to create directory")
			}
			logger.Info("Directory created successfully")
		} else {
			logger.WithError(err).Fatal("Stat has failed")
		}
	} else {
		if !fileInfo.IsDir() {
			logger.Fatalf("'%s' is not a directory. Remove the plain file if you want to continue", path)
		}
	}
}

func main() {
	execDir, err := osext.ExecutableFolder()
	if err != nil {
		panic(err)
	}

	configFile := flag.String(
		"config",
		filepath.Join(execDir, "config.json"),
		"The configuration file to load the application's configuration from",
	)

	ctx := context.Background()

	// Initialize the logger
	logger := logrus.WithField(log.FldVersion, appVersion)
	logger.Infof("%s version %s is starting up...", appName, appVersion)
	ctx = context.WithValue(ctx, ctxhelper.KeyLogger, logger)

	// Load the main configuration file
	cs := kyabia.NewConfigService(*configFile)
	if err := cs.Load(ctx); err != nil {
		logger.WithError(err).Error("Cannot load config. Using defaults")
	}
	conf := cs.GetConfig(ctx)

	logger.Infof("Using '%s' as data directory", conf.DataDir)
	checkAndCreateDir(conf.DataDir, logger)

	// Set up the database connection and perform pending migrations
	dbFileName := path.Join(conf.DataDir, dbFile)
	var db *sqlx.DB
	if db, err = sqlx.Open("sqlite3", dbFileName); err != nil {
		logger.WithError(err).Fatal("Failed to open database connection")
	}
	logger.Info("Performing database migrations...")
	if err = migrate.ExecuteMigrationsOnDb(db, logger); err != nil {
		logger.WithError(err).Fatal("Database migration has failed. Please check database for consistency and try again.")
	}

	// Prepare the in-memory user repo and fill it with the default user
	// TODO: Implement proper user management with database backend
	userRepo := userrepo.New()
	u := models.User{
		Name:     strings.ToLower(conf.DefaultUser.Name),
		FullName: conf.DefaultUser.Name,
	}
	err = u.SetPassword(conf.DefaultUser.Password)
	if err != nil {
		logger.WithError(err).Fatal("Failed to set password for default user")
		panic("Without user, there is no use to live on!")
	}
	userRepo.Create(&u)
	logger.Info(fmt.Sprintf("Created user '%s' with password hash %s", u.Name, u.PasswordHash))

	videoRepo := vidrepo.New(db, logger)
	playlistRepo := plrepo.New(db, logger)
	eventRepo := eventrepo.New(db, logger)
	sessionRepo := sessionrepo.New()

	scr := scraper.NewDefault(videoRepo, logger)

	scrServ := kyabia.NewScrapingService(scr, logger)
	viSrv := kyabia.NewVideoService(videoRepo, logger)
	evSrv := kyabia.NewEventService(eventRepo, playlistRepo, logger)
	plSrv := kyabia.NewPlaylistService(playlistRepo, videoRepo, evSrv, cs, logger)
	sessServ := kyabia.NewSessionService(sessionRepo, userRepo, logger)

	// Auto-Select an event with matchin start and end times
	evts, _ := eventRepo.GetByDate(time.Now())
	if len(evts) > 0 {
		logger.Infof("Auto-selecting event %d (%s) as current event", evts[0].ID, evts[0].Name)
		evSrv.SetCurrentEvent(ctx, evts[0].ID)
	}

	httpLogger := logger.WithField(log.FldTransport, "HTTP")

	h := kyabia.MakeHTTPHandler(
		scrServ,
		viSrv,
		plSrv,
		evSrv,
		sessServ,
		cs,
		httpLogger,
	)

	// Start listening
	errs := make(chan error)

	// Listen for stop signals that will end the service
	go func() {
		c := make(chan os.Signal, 2)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		err := fmt.Errorf("%s", <-c)
		logger.Info("Caught signal to stop. Shutting down.")
		logger.Info("Stopping pending scrapes...")
		scr.StopAll()
		logger.Info("Scrapes have been stopped")
		errs <- err
	}()

	go func() {
		httpLogger.WithField("addr", conf.ListenAddress).Info("Starting listening port")
		errs <- http.ListenAndServe(conf.ListenAddress, h)
	}()

	// Watchdog for systemd
	go func() {
		interval, err := daemon.SdWatchdogEnabled(false)
		if err != nil || interval == 0 {
			return
		}
		logger.Info("Activating systemd watchdog goroutine")
		port := strings.Split(conf.ListenAddress, ":")[1]
		url := fmt.Sprintf("http://127.0.0.1:%s/alive", port)
		for {
			if _, err := http.Get(url); err == nil {
				daemon.SdNotify(false, "WATCHDOG=1")
			}
			time.Sleep(interval / 3)
		}
	}()

	// Notify systemd that we are ready to go (if available)
	daemon.SdNotify(false, "READY=1")

	logger.WithError(<-errs).Error("Shutdown complete")
}
