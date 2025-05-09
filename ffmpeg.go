package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SubtitleTrack represents a subtitle track in an MKV file
type SubtitleTrack struct {
	Index    int
	Language string
	Format   string
}

// FFmpeg encapsulates ffmpeg functionality
type FFmpeg struct {
	Path      string // Path to the ffmpeg executable
	LogOutput bool   // Whether to print command output to console
}

// NewFFmpeg creates a new FFmpeg instance
func NewFFmpeg() (*FFmpeg, error) {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found: %v", err)
	}
	return &FFmpeg{
		Path:      path,
		LogOutput: true, // Default to logging output
	}, nil
}

// NewFFmpegWithPath creates a new FFmpeg instance with a custom path
func NewFFmpegWithPath(path string) (*FFmpeg, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("ffmpeg not found at path %s: %v", path, err)
	}
	return &FFmpeg{
		Path:      path,
		LogOutput: true,
	}, nil
}

// SetLogOutput sets whether to print command output to console
func (ff *FFmpeg) SetLogOutput(logOutput bool) {
	ff.LogOutput = logOutput
}

// RunCommand executes an ffmpeg command and captures its output
func (ff *FFmpeg) RunCommand(args ...string) (string, string, error) {
	cmd := exec.Command(ff.Path, args...)

	// Create pipes for stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	// Read stdout into buffer while also printing to console if enabled
	var stdout bytes.Buffer
	var stdoutDone = make(chan struct{})
	go func() {
		defer close(stdoutDone)
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				stdout.Write(buf[:n])
				if ff.LogOutput {
					os.Stdout.Write(buf[:n])
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Read stderr into buffer while also printing to console if enabled
	var stderr bytes.Buffer
	var stderrDone = make(chan struct{})
	go func() {
		defer close(stderrDone)
		buf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				stderr.Write(buf[:n])
				if ff.LogOutput {
					os.Stderr.Write(buf[:n])
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for command to finish
	err = cmd.Wait()
	<-stdoutDone
	<-stderrDone

	return stdout.String(), stderr.String(), err
}

// ListSubtitleTracks lists all subtitle tracks in a media file
func (ff *FFmpeg) ListSubtitleTracks(mediaPath string) ([]SubtitleTrack, error) {
	// Run ffmpeg to get information about the media file
	_, stderr, _ := ff.RunCommand("-i", mediaPath)

	// Parse the output to find subtitle tracks
	lines := strings.Split(stderr, "\n")

	var tracks []SubtitleTrack
	trackIndex := 0

	fmt.Println("Scanning media file for subtitle streams...")
	for _, line := range lines {
		if strings.Contains(line, "Stream") {
			if strings.Contains(line, "Subtitle") {
				// Example: Stream #0:2(eng): Subtitle: subrip
				track := SubtitleTrack{
					Index: trackIndex,
				}

				// Extract language
				if langStart := strings.Index(line, "("); langStart != -1 {
					if langEnd := strings.Index(line[langStart:], ")"); langEnd != -1 {
						track.Language = line[langStart+1 : langStart+langEnd]
					}
				}

				// Extract format
				// Extract type and format
				if formatStart := strings.Index(line, "Subtitle: "); formatStart != -1 {
					restOfLine := line[formatStart+10:]        // Skip "Subtitle: "
					restOfLine = strings.TrimSpace(restOfLine) // Trim leading spaces
					for i, char := range restOfLine {
						if char == ' ' || char == '(' {
							track.Format = restOfLine[:i]
							break
						}
					}
					// If no space or parenthesis is found, use the entire remaining string
					if track.Format == "" {
						track.Format = restOfLine
					}
				}
				fmt.Printf("Parsed track: Index=%d, Language=%s, Format=%s\n", track.Index, track.Language, track.Format)
				fmt.Printf("Parsed subtitle track: Index=%d, Language=%s, Format=%s\n", track.Index, track.Language, track.Format)

				tracks = append(tracks, track)
				trackIndex++
			}

		}

	}
	return tracks, nil
}

// ExtractSubtitleTrack extracts a subtitle track from a media file
// trackIndex is the index of the track to extract (0 for first subtitle track)
// outputFormat should be "srt" or "ass"
func (ff *FFmpeg) ExtractSubtitleTrack(mediaPath string, trackIndex int, outputFormat string, langCode string) (string, error) {
	// Create output filename based on input filename and language code
	baseFilename := filepath.Base(mediaPath)
	baseFilename = strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename))
	outputDir := filepath.Dir(mediaPath) // Get the directory of the input file
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.%s.%s", baseFilename, langCode, outputFormat))

	// Run ffmpeg to extract subtitle
	_, stderr, err := ff.RunCommand(
		"-y", "-i", mediaPath,
		"-map", fmt.Sprintf("0:s:%d", trackIndex),
		"-c:s", outputFormat,
		outputPath,
	)

	if err != nil {
		return "", fmt.Errorf("failed to extract subtitle: %v\nffmpeg error: %s", err, stderr)
	}

	return outputPath, nil
}
