package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Global job manager and database
var jobManager *JobManager
var jobManagerOnce sync.Once

// GetJobManager returns the singleton job manager instance
func GetJobManager() *JobManager {
	jobManagerOnce.Do(func() {
		jobManager = NewJobManager()
	})
	return jobManager
}

// generateUUID creates a random UUID
func generateUUID() string {
	uuid := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, uuid)
	if err != nil {
		// Fall back to a timestamp-based identifier in case of error
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// Version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// JobStatus represents the status of a translation job
type JobStatus string

const (
	// JobStatusPending indicates the job is waiting to be processed
	JobStatusPending JobStatus = "pending"
	// JobStatusProcessing indicates the job is currently being processed
	JobStatusProcessing JobStatus = "processing"
	// JobStatusCompleted indicates the job has completed successfully
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed indicates the job has failed
	JobStatusFailed      JobStatus = "failed"
	JobStatusExtracting  JobStatus = "extracting"
	JobStatusTranslating JobStatus = "translating"
)

// JobResult represents the result of a completed job
type JobResult struct {
	OutputPath string `json:"outputPath,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Job represents a translation job
type Job struct {
	ID         string    `json:"id"`
	Status     JobStatus `json:"status"`
	Progress   float64   `json:"progress"`
	Path       string    `json:"path"`
	TrackIndex int       `json:"trackIndex"`
	Result     JobResult `json:"result,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// JobManager manages translation jobs
type JobManager struct {
	jobs  map[string]*Job
	mutex sync.RWMutex
}

// NewJobManager creates a new job manager
func NewJobManager() *JobManager {
	return &JobManager{
		jobs: make(map[string]*Job),
	}
}

// CreateJob creates a new job with the given parameters
func (jm *JobManager) CreateJob(path string, trackIndex int) *Job {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	id := generateUUID()
	now := time.Now()

	job := &Job{
		ID:         id,
		Status:     JobStatusPending,
		Progress:   0.0,
		Path:       path,
		TrackIndex: trackIndex,
		Result:     JobResult{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	jm.jobs[id] = job
	return job
}

// GetJob returns a job by its ID
func (jm *JobManager) GetJob(id string) (*Job, error) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	job, exists := jm.jobs[id]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	return job, nil
}

// UpdateJobStatus updates the status of a job
func (jm *JobManager) UpdateJobStatus(id string, status JobStatus) error {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	job, exists := jm.jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Status = status
	job.UpdatedAt = time.Now()
	return nil
}

// UpdateJobProgress updates the progress of a job
func (jm *JobManager) UpdateJobProgress(id string, progress float64) error {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	job, exists := jm.jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Progress = progress
	job.UpdatedAt = time.Now()
	return nil
}

// SetJobResult sets the result of a completed job
func (jm *JobManager) SetJobResult(id string, outputPath string) error {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	job, exists := jm.jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Status = JobStatusCompleted
	job.Progress = 100.0
	job.Result.OutputPath = outputPath
	job.UpdatedAt = time.Now()
	return nil
}

// SetJobError sets an error on a failed job
func (jm *JobManager) SetJobError(id string, err error) error {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	job, exists := jm.jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Status = JobStatusFailed
	job.Result.Error = err.Error()
	job.UpdatedAt = time.Now()
	return nil
}

// ProcessJob processes a translation job asynchronously
func (jm *JobManager) ProcessJob(id string) {
	go func() {
		// Get the job
		job, err := jm.GetJob(id)
		if err != nil {
			slog.Error("Error getting job", "id", id, "error", err)
			return
		}

		// Update job status to processing
		err = jm.UpdateJobStatus(id, JobStatusProcessing)
		if err != nil {
			slog.Error("Error updating job status", "id", id, "error", err)
			return
		}

		// Initialize progress at 0%
		err = jm.UpdateJobProgress(id, 0.0)
		if err != nil {
			slog.Error("Error updating job progress", "id", id, "error", err)
			return
		}

		// Create a progress channel for communication between components
		progressChan := make(chan float64)

		// Start a goroutine to handle progress updates
		go func() {
			for progress := range progressChan {
				err := jm.UpdateJobProgress(id, progress)
				if err != nil {
					slog.Error("Error updating job progress", "id", id, "error", err)
				}
			}
		}()

		// Detect file type
		fileType, err := DetectFileType(job.Path)
		if err != nil {
			slog.Error("Error detecting file type", "path", job.Path, "error", err)
			jm.SetJobError(id, fmt.Errorf("error detecting file type: %w", err))
			close(progressChan)
			return
		}

		// Update progress to 1%
		progressChan <- 1.0

		// Initialize variables for processing
		var extractedPath string

		// Process based on file type
		if fileType.IsVideo() {
			// Verify file exists and is accessible
			if _, err := os.Stat(job.Path); os.IsNotExist(err) {
				slog.Error("Video file does not exist", "id", id, "path", job.Path)
				jm.SetJobError(id, fmt.Errorf("video file '%s' does not exist", job.Path))
				close(progressChan)
				return
			}

			ff, err := NewFFmpeg()
			if err != nil {
				slog.Error("Error initializing FFmpeg", "id", id, "error", err)
				jm.SetJobError(id, fmt.Errorf("error initializing FFmpeg: %w", err))
				close(progressChan)
				return
			}
			jm.UpdateJobStatus(id, JobStatusExtracting)
			tracks, err := ff.ListSubtitleTracks(job.Path)
			if err != nil {
				slog.Error("Error listing subtitle tracks", "id", id, "path", job.Path, "error", err)
				jm.SetJobError(id, fmt.Errorf("error listing subtitle tracks from '%s': %w", job.Path, err))
				close(progressChan)
				return
			}

			if len(tracks) == 0 {
				slog.Error("No subtitle tracks found", "id", id, "path", job.Path)
				jm.SetJobError(id, fmt.Errorf("no subtitle tracks found in the media file"))
				close(progressChan)
				return
			}

			if job.TrackIndex < 0 || job.TrackIndex >= len(tracks) {
				slog.Error("Invalid track index", "id", id, "index", job.TrackIndex, "total_tracks", len(tracks))
				jm.SetJobError(id, fmt.Errorf("invalid track index %d (file has %d tracks)", job.TrackIndex, len(tracks)))
				close(progressChan)
				return
			}

			// Update progress to 2%
			progressChan <- 2.0

			// Extract the subtitle track
			outputFormat := "srt"
			langCode := "en"
			// Use language from track if available
			if track := tracks[job.TrackIndex]; track.Language != "" {
				langCode = track.Language
			}

			slog.Info("Extracting subtitle track", "id", id, "track_index", job.TrackIndex,
				"format", outputFormat, "path", job.Path)

			extractedPath, err = ff.ExtractSubtitleTrack(job.Path, job.TrackIndex, outputFormat, langCode)
			if err != nil {
				slog.Error("Failed to extract subtitle", "id", id, "error", err)
				jm.SetJobError(id, fmt.Errorf("error extracting subtitle track %d from '%s': %w",
					job.TrackIndex, job.Path, err))
				close(progressChan)
				return
			}

			// Verify extracted file exists and is readable
			if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
				slog.Error("Extracted subtitle file does not exist", "id", id, "path", extractedPath)
				jm.SetJobError(id, fmt.Errorf("extracted subtitle file '%s' does not exist", extractedPath))
				close(progressChan)
				return
			}

			// Update progress to 20%
			progressChan <- 20.0
		} else if fileType.IsSubtitle() {
			// Verify subtitle file exists and is accessible
			if _, err := os.Stat(job.Path); os.IsNotExist(err) {
				slog.Error("Subtitle file does not exist", "id", id, "path", job.Path)
				jm.SetJobError(id, fmt.Errorf("subtitle file '%s' does not exist", job.Path))
				close(progressChan)
				return
			}

			extractedPath = job.Path
			slog.Info("Using subtitle file directly", "id", id, "path", extractedPath)

			// Update progress to 30% (skip extraction steps)
			progressChan <- 20.0
		} else {
			slog.Error("Unsupported file type", "id", id, "file_type", fileType)
			jm.SetJobError(id, fmt.Errorf("unsupported file type: %s", fileType))
			close(progressChan)
			return
		}
		jm.UpdateJobStatus(id, JobStatusTranslating)
		// Translate the extracted subtitle
		outputPath := deriveOutputPath(extractedPath)
		translator := NewTranslator()

		// Create a goroutine to handle translation progress scaling
		translationProgressChan := make(chan float64)
		go func() {
			for progress := range translationProgressChan {
				scaledProgress := 20.0 + (progress * 0.75)
				progressChan <- scaledProgress
			}
		}()

		// Set the translation progress channel
		translator.SetProgressChannel(translationProgressChan)

		err = translator.TranslateSubtitleFile(extractedPath, outputPath)
		if err != nil {
			jm.SetJobError(id, fmt.Errorf("error translating subtitles: %w", err))
			close(translationProgressChan)
			close(progressChan)
			return
		}

		// Close the translation progress channel as it's no longer needed
		close(translationProgressChan)

		// Update progress to 95%
		progressChan <- 99.0

		// Set the job result
		err = jm.SetJobResult(id, outputPath)
		if err != nil {
			slog.Error("Error setting job result", "id", id, "error", err)
			close(progressChan)
			return
		}

		// Update progress to 100%
		progressChan <- 100.0

		// Close the progress channel as we're done
		close(progressChan)

		slog.Info("Job completed successfully", "id", id)
	}()
}
