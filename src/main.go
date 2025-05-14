package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

func main() {
	w := os.Stderr

	// set global logger with custom options
	slog.SetDefault(slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: "2006-01-02 15:04:05.999999999",
		}),
	))

	slog.Info("Starting application")
	LoadConfig("./config.yaml")
	InitDatabase()
	stopChannel := RunBackgroundSync()
	RunWebService()
	stopChannel <- true
}

func RunBackgroundSync() chan (bool) {
	ticker := time.NewTicker(60 * time.Second)
	stopChannel := make(chan bool)
	mediaPaths := GetAllMediaPaths()
	db := GetDB()
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				slog.Info("Running background sync")
				startTime := time.Now()
				for _, path := range mediaPaths {
					slog.Info("Syncing media path", "path", path)
					current, err := db.GetCachedMediaFiles(path.Path)
					if err != nil {
						// Log the error but continue
						slog.Warn("Failed to get cached media files", "path", path.Path, "error", err)
						continue
					}
					mediaFiles, err := FindMediaFiles(path.Path, current)
					if err != nil {
						// Log the error but continue
						slog.Warn("Failed to find media files", "path", path.Path, "error", err)
						continue
					}
					if len(mediaFiles) > 0 {
						if err := db.CacheMediaFiles(mediaFiles); err != nil {
							// Log the error but continue
							slog.Warn("Failed to cache media files", "error", err)
						}
					}
				}
				endTime := time.Now()
				slog.Info("Background sync completed", "duration", endTime.Sub(startTime))
			case <-stopChannel:
				slog.Info("Stopping background sync")
				close(stopChannel)
				return
			}
		}
	}()
	return stopChannel
}
