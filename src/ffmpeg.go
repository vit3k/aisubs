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
	Title    string
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
		return nil, fmt.Errorf("ffmpeg not found in PATH: %v", err)
	}

	// Verify the executable exists and is executable
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error checking ffmpeg executable: %v", err)
	}

	// Check if it's a regular file and has execute permission
	mode := fileInfo.Mode()
	if !mode.IsRegular() {
		return nil, fmt.Errorf("ffmpeg path is not a regular file: %s", path)
	}

	// On Unix systems, check execute permission (0100)
	if mode&0111 == 0 {
		return nil, fmt.Errorf("ffmpeg file is not executable: %s", path)
	}

	// Test execute ffmpeg to verify it works
	cmd := exec.Command(path, "-version")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to execute ffmpeg: %v", err)
	}

	return &FFmpeg{
		Path:      path,
		LogOutput: true, // Default to logging output
	}, nil
}

// NewFFmpegWithPath creates a new FFmpeg instance with a custom path
func NewFFmpegWithPath(path string) (*FFmpeg, error) {
	if path == "" {
		return nil, fmt.Errorf("ffmpeg path cannot be empty")
	}

	// Check if the file exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("ffmpeg not found at path %s", path)
		}
		return nil, fmt.Errorf("error checking ffmpeg executable: %v", err)
	}

	// Check if it's a regular file
	if !fileInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("ffmpeg path is not a regular file: %s", path)
	}

	// On Unix systems, check execute permission (0100)
	if fileInfo.Mode()&0111 == 0 {
		return nil, fmt.Errorf("ffmpeg file is not executable: %s", path)
	}

	// Verify ffmpeg works by running a simple command
	cmd := exec.Command(path, "-version")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to execute ffmpeg at %s: %v", path, err)
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
	if ff.Path == "" {
		return "", "", fmt.Errorf("ffmpeg path is not set")
	}

	// Verify ffmpeg executable exists
	if _, err := os.Stat(ff.Path); os.IsNotExist(err) {
		return "", "", fmt.Errorf("ffmpeg executable not found at %s", ff.Path)
	}

	cmd := exec.Command(ff.Path, args...)

	// Create context with timeout if needed
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	// defer cancel()
	// cmd = exec.CommandContext(ctx, ff.Path, args...)

	// Log the command being executed for debugging
	if ff.LogOutput {
		fmt.Printf("Executing: %s %s\n", ff.Path, strings.Join(args, " "))
	}

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
		// Check common errors
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("ffmpeg executable not found: %v", err)
		}
		if os.IsPermission(err) {
			return "", "", fmt.Errorf("permission denied when running ffmpeg: %v", err)
		}
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

	// Parse specific errors from stderr
	stderrStr := stderr.String()
	if err != nil && stderrStr != "" {
		if strings.Contains(stderrStr, "No such file or directory") {
			return stdout.String(), stderrStr, fmt.Errorf("ffmpeg couldn't find the input file: %v", err)
		}
		if strings.Contains(stderrStr, "Permission denied") {
			return stdout.String(), stderrStr, fmt.Errorf("permission denied when accessing file: %v", err)
		}
		if strings.Contains(stderrStr, "Invalid data found when processing input") {
			return stdout.String(), stderrStr, fmt.Errorf("invalid or corrupted input file: %v", err)
		}
	}

	return stdout.String(), stderrStr, err
}

// ListSubtitleTracks lists all subtitle tracks in a media file
func (ff *FFmpeg) ListSubtitleTracks(mediaPath string) ([]SubtitleTrack, error) {
	// Check if the media file exists
	if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("media file does not exist: %s", mediaPath)
	}

	// Run ffmpeg to get information about the media file
	_, stderr, err := ff.RunCommand("-i", mediaPath)
	if err != nil {
		// Don't return an error here as ffmpeg returns non-zero when used with -i flag alone
		// We just need the output for parsing
		if !strings.Contains(stderr, "Input #0") {
			return nil, fmt.Errorf("failed to get media info: %v", err)
		}
	}

	// Parse the output to find subtitle tracks
	lines := strings.Split(stderr, "\n")

	var tracks []SubtitleTrack
	trackIndex := 0

	fmt.Println("Scanning media file for subtitle streams...")
	//var lastSubtitleIdx = -1
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "Stream") && strings.Contains(line, "Subtitle") {
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
			if formatStart := strings.Index(line, "Subtitle: "); formatStart != -1 {
				restOfLine := line[formatStart+10:]        // Skip "Subtitle: "
				restOfLine = strings.TrimSpace(restOfLine) // Trim leading spaces
				for j, char := range restOfLine {
					if char == ' ' || char == '(' {
						track.Format = restOfLine[:j]
						break
					}
				}
				// If no space or parenthesis is found, use the entire remaining string
				if track.Format == "" {
					track.Format = restOfLine
				}
			}

			// Look ahead for Metadata and title
			if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "Metadata:" {
				i++ // move to Metadata:
				for i+1 < len(lines) && (strings.HasPrefix(lines[i+1], "      ") || strings.HasPrefix(lines[i+1], "\t")) {
					metaLine := strings.TrimSpace(lines[i+1])
					if strings.HasPrefix(metaLine, "title") {
						// metaLine is like "title           : English"
						if colonIdx := strings.Index(metaLine, ":"); colonIdx != -1 {
							track.Title = strings.TrimSpace(metaLine[colonIdx+1:])
						}
					}
					i++
				}
			}

			// If language is still empty, try to infer from title
			if track.Language == "" && track.Title != "" {
				track.Language = normalizeLanguageCode(track.Title)
			}

			fmt.Printf("Parsed track: Index=%d, Language=%s, Format=%s, Title=%s\n", track.Index, track.Language, track.Format, track.Title)
			tracks = append(tracks, track)
			trackIndex++
		}
	}

	if len(tracks) == 0 {
		return tracks, fmt.Errorf("no subtitle tracks found in the media file: %s", mediaPath)
	}

	return tracks, nil
}

// ExtractSubtitleTrack extracts a subtitle track from a media file
// trackIndex is the index of the track to extract (0 for first subtitle track)
// outputFormat should be "srt" or "ass"
func (ff *FFmpeg) ExtractSubtitleTrack(mediaPath string, trackIndex int, outputFormat string, langCode string) (string, error) {
	// Validate input parameters
	if mediaPath == "" {
		return "", fmt.Errorf("media path cannot be empty")
	}

	// Check if the media file exists
	if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
		return "", fmt.Errorf("media file does not exist: %s", mediaPath)
	}

	// Validate track index
	if trackIndex < 0 {
		return "", fmt.Errorf("invalid track index: %d, must be >= 0", trackIndex)
	}

	// Validate output format
	if outputFormat != "srt" && outputFormat != "ass" {
		return "", fmt.Errorf("invalid output format: %s, must be 'srt' or 'ass'", outputFormat)
	}

	// Create output filename based on input filename and language code
	baseFilename := filepath.Base(mediaPath)
	baseFilename = strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename))
	outputDir := filepath.Dir(mediaPath) // Get the directory of the input file
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.%s.%s", baseFilename, langCode, outputFormat))

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}

	// Run ffmpeg to extract subtitle
	_, stderr, err := ff.RunCommand(
		"-y", "-i", mediaPath,
		"-map", fmt.Sprintf("0:s:%d", trackIndex),
		"-c:s", outputFormat,
		outputPath,
	)

	if err != nil {
		// Check if the error is due to the track index being out of range
		if strings.Contains(stderr, "Invalid stream specifier") {
			return "", fmt.Errorf("invalid subtitle track index %d: %v", trackIndex, err)
		}
		// Check if it failed to write the output file
		if strings.Contains(stderr, "Permission denied") {
			return "", fmt.Errorf("permission denied when writing to %s: %v", outputPath, err)
		}
		return "", fmt.Errorf("failed to extract subtitle: %v\nffmpeg error: %s", err, stderr)
	}

	// Verify output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("ffmpeg ran successfully but output file was not created: %s", outputPath)
	}

	return outputPath, nil
}
