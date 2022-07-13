package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/robfig/cron"
	"go.uber.org/zap"
)

type backup struct {
	Schedule           string `required:"true"    envconfig:"SCHEDULE"`            // cron schedule
	Repository         string `required:"true"    envconfig:"RESTIC_REPOSITORY"`   // repository name
	Password           string `required:"true"    envconfig:"RESTIC_PASSWORD"`     // repository password
	Args               string `                   envconfig:"RESTIC_ARGS"`         // additional args for backup command
	RunOnBoot          bool   `                   envconfig:"RUN_ON_BOOT"`         // run a backup on startup
	PrometheusEndpoint string `default:"/metrics" envconfig:"PROMETHEUS_ENDPOINT"` // metrics endpoint
	PrometheusAddress  string `default:":8080"    envconfig:"PROMETHEUS_ADDRESS"`  // metrics host:port
	PreCommand         string `                   envconfig:"PRE_COMMAND"`         // command to execute before restic is executed
	PostCommand        string `                   envconfig:"POST_COMMAND"`        // command to execute after restic was executed (successfully)
	RcloneArgs         string `                   envconfig:"RCLONE_ARGS"`         // additional args for rclone command

	backupsTotal      prometheus.Counter
	backupsSuccessful prometheus.Counter
	backupsFailed     prometheus.Counter
	backupDuration    prometheus.Histogram
	filesNew          prometheus.Histogram
	filesChanged      prometheus.Histogram
	filesUnmodified   prometheus.Histogram
	filesProcessed    prometheus.Histogram
	bytesAdded        prometheus.Histogram
	bytesProcessed    prometheus.Histogram
}

var (
	matchExists     = regexp.MustCompile(`.*already (exists|initialized).*`)
	matchFileStats  = regexp.MustCompile(`Files:\s*([0-9.]*) new,\s*([0-9.]*) changed,\s*([0-9.]*) unmodified`)
	matchAddedBytes = regexp.MustCompile(`Added to the repo: ([0-9.]+) (\w+)`)
	matchProcessed  = regexp.MustCompile(`processed ([0-9.]*) files, ([0-9.]+) (\w+)`)
)

type stats struct {
	Duration        float64
	FilesNew        int
	FilesChanged    int
	FilesUnmodified int
	FilesProcessed  int
	BytesAdded      int
	BytesProcessed  int
}

func main() {
	b := backup{}
	err := envconfig.Process("", &b)
	if err != nil {
		logger.Fatal("failed to configure", zap.Error(err))
	}

	err = b.Ensure()
	if err != nil {
		logger.Fatal("failed to ensure repository", zap.Error(err))
	}

	go b.startMetricsServer()

	cr := cron.New()
	err = cr.AddJob(b.Schedule, &b)
	if err != nil {
		logger.Fatal("failed to schedule task", zap.Error(err))
	}
	if b.RunOnBoot {
		b.Run()
	}
	cr.Run()
}

// Run performs the backup
func (b *backup) Run() {
	logger.Info("backup started")
	startTime := time.Now()

	if len(b.PreCommand) > 0 {
		if stdout, err := b.executePreCommand(); err != nil {
			logger.Error("failed to execute pre-command: " + err.Error())
			b.backupsFailed.Inc()
			b.backupsTotal.Inc()
			return
		} else {
			logger.Info("output of pre-command: " + *stdout)
		}
	}

	args := []string{"backup"}
	if len(b.RcloneArgs) > 0 {
		args = append(args, "-o")
		args = append(args, b.RcloneArgs)
	}
	args = append(args, strings.Split(b.Args, " ")...)

	cmd := exec.Command("restic", args...)
	logger.Info("Launching backup command: " + cmd.String())

	errbuf := bytes.NewBuffer(nil)
	outbuf := bytes.NewBuffer(nil)
	cmd.Stderr = errbuf
	cmd.Stdout = outbuf

	if err := cmd.Run(); err != nil {
		logger.Error("failed to run backup",
			zap.Error(err),
			zap.String("output", errbuf.String()))
		b.backupsFailed.Inc()
		b.backupsTotal.Inc()
		return
	}

	if len(b.PostCommand) > 0 {
		if stdout, err := b.executePostCommand(); err != nil {
			logger.Error("failed to execute post-command: " + err.Error())
			b.backupsFailed.Inc()
			b.backupsTotal.Inc()
			return
		} else {
			logger.Info("output of post-command: " + *stdout)
		}
	}

	statistics, err := extractStats(outbuf.String())
	if err != nil {
		logger.Warn("failed to extract statistics from command output",
			zap.Error(err))
	}
	statistics.Duration = time.Since(startTime).Seconds()

	b.backupsSuccessful.Inc()
	b.backupsTotal.Inc()

	b.ObserveStats(statistics)
}

func (b *backup) ObserveStats(statistics stats) {

	logger.Info("backup reading",
		zap.Float64("duration", statistics.Duration),
		zap.Int("filesNew", statistics.FilesNew),
		zap.Int("filesChanged", statistics.FilesChanged),
		zap.Int("filesUnmodified", statistics.FilesUnmodified),
		zap.Int("filesProcessed", statistics.FilesProcessed),
		zap.Int("bytesAdded", statistics.BytesAdded),
		zap.Int("bytesProcessed", statistics.BytesProcessed),
	)

	b.filesNew.Observe(float64(statistics.FilesNew))
	b.backupDuration.Observe(float64(statistics.Duration))
	b.filesChanged.Observe(float64(statistics.FilesChanged))
	b.filesUnmodified.Observe(float64(statistics.FilesUnmodified))
	b.filesProcessed.Observe(float64(statistics.FilesProcessed))
	b.bytesAdded.Observe(float64(statistics.BytesAdded))
	b.bytesProcessed.Observe(float64(statistics.BytesProcessed))
}

func extractStats(s string) (result stats, err error) {
	fileStats := matchFileStats.FindAllStringSubmatch(s, -1)
	if len(fileStats[0]) != 4 {
		err = errors.Errorf("matchFileStats expected 4, got %d", len(fileStats[0]))
		return
	}
	result.FilesNew, _ = strconv.Atoi(fileStats[0][1])        //nolint:errcheck
	result.FilesChanged, _ = strconv.Atoi(fileStats[0][2])    //nolint:errcheck
	result.FilesUnmodified, _ = strconv.Atoi(fileStats[0][3]) //nolint:errcheck

	addedBytes := matchAddedBytes.FindAllStringSubmatch(s, -1)
	if len(addedBytes[0]) != 3 {
		err = errors.Errorf("matchAddedBytes expected 3, got %d", len(addedBytes[0]))
		return
	}
	amount, _ := strconv.ParseFloat(addedBytes[0][1], 64) //nolint:errcheck
	// restic doesn't use a comma to denote thousands
	amount *= 1000
	result.BytesAdded = convert(int(amount), addedBytes[0][2])

	filesProcessed := matchProcessed.FindAllStringSubmatch(s, -1)
	if len(filesProcessed[0]) != 4 {
		err = errors.Errorf("filesProcessed expected 4, got %d", len(filesProcessed[0]))
		return
	}
	result.FilesProcessed, _ = strconv.Atoi(filesProcessed[0][1]) //nolint:errcheck
	amount, _ = strconv.ParseFloat(filesProcessed[0][2], 64)      //nolint:errcheck
	amount *= 1000
	result.BytesProcessed = convert(int(amount), filesProcessed[0][3])

	return
}

func convert(b int, unit string) (result int) {
	switch unit {
	case "TiB":
		result = b * (1 << 40)
	case "GiB":
		result = b * (1 << 30)
	case "MiB":
		result = b * (1 << 20)
	case "KiB":
		result = b * (1 << 10)
	}
	return
}

// Ensure will create a repository if it does not already exist
func (b *backup) Ensure() (err error) {
	logger.Info("ensuring backup repository exists")
	cmd := exec.Command("restic", "init")
	out := bytes.NewBuffer(nil)
	cmd.Stderr = out
	err = cmd.Run()
	if err != nil {
		if matchExists.MatchString(strings.Trim(out.String(), " \n\r")) {
			logger.Info("repository exists")
			return nil
		}
		return errors.Wrap(err, out.String())
	}
	logger.Info("successfully created repository")
	return
}
