package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB represents the database connection
type DB struct {
	conn *sql.DB
}

var dbOnce sync.Once
var databaseConn *DB

func InitDatabase() {
	dbOnce.Do(func() {
		dbFilePath := GetConfig().Database.Path
		db, err := NewDB(dbFilePath)
		if err != nil {
			slog.Error("Could not initialize database", "error", err)
			// Continue without database support
			panic(err)
		}
		databaseConn = db
	})
}

// GetDB returns the singleton database connection
func GetDB() *DB {
	InitDatabase()
	return databaseConn
}

// NewDB creates a new database connection
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	db := &DB{conn: conn}
	if err := db.initialize(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// initialize sets up the database schema
func (db *DB) initialize() error {
	_, err := db.conn.Exec(`
	PRAGMA foreign_keys = ON;

	CREATE TABLE IF NOT EXISTS videos (
		id INTEGER PRIMARY KEY,
		path TEXT UNIQUE NOT NULL,
		file_type TEXT NOT NULL,
		scan_time INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS subtitles (
		id INTEGER PRIMARY KEY,
		video_id INTEGER,
		path TEXT,
		track_index INTEGER,
		language TEXT NOT NULL,
		format TEXT NOT NULL,
		embedded INTEGER NOT NULL,
		subtitle_type TEXT,
		title TEXT,
		FOREIGN KEY (video_id) REFERENCES videos(id) ON DELETE CASCADE,
		UNIQUE(video_id, path, track_index) ON CONFLICT REPLACE
	);

	CREATE INDEX IF NOT EXISTS idx_videos_path ON videos(path);
	CREATE INDEX IF NOT EXISTS idx_subtitles_video_id ON subtitles(video_id);
	`)

	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// CacheMediaFiles stores the media files in the database
func (db *DB) CacheMediaFiles(mediaFiles []GroupedMediaFile) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Current scan time
	scanTime := time.Now().Unix()

	// Prepare statements
	insertVideo, err := tx.Prepare(`
		INSERT OR REPLACE INTO videos (path, file_type, scan_time)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare video insert statement: %v", err)
	}
	defer insertVideo.Close()

	insertSubtitle, err := tx.Prepare(`
		INSERT OR REPLACE INTO subtitles (
			video_id, path, track_index, language, format,
			embedded, subtitle_type, title
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare subtitle insert statement: %v", err)
	}
	defer insertSubtitle.Close()

	// Insert each media file
	for _, media := range mediaFiles {
		var videoID int64 = 0

		// Only insert video if it exists
		if media.VideoFile != "" {
			fileType, err := DetectFileType(media.VideoFile)
			if err != nil {
				continue // Skip if we can't determine file type
			}

			result, err := insertVideo.Exec(
				media.VideoFile,
				fileType.String(),
				scanTime,
			)
			if err != nil {
				return fmt.Errorf("failed to insert video: %v", err)
			}

			videoID, err = result.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed to get last insert ID: %v", err)
			}
		}

		// Insert all subtitles
		for _, sub := range media.Subtitles {
			embedded := 0
			if sub.Embedded {
				embedded = 1
			}

			_, err = insertSubtitle.Exec(
				sqlNullInt64(videoID),
				sqlNullString(sub.Path),
				sub.TrackIndex,
				sub.Language,
				sub.Format,
				embedded,
				sqlNullString(sub.SubtitleType),
				sqlNullString(sub.Title),
			)
			if err != nil {
				return fmt.Errorf("failed to insert subtitle: %v", err)
			}
		}
	}

	return tx.Commit()
}

// GetCachedMediaFiles retrieves the cached media files for a directory
func (db *DB) GetCachedMediaFiles(dirPath string) ([]GroupedMediaFile, error) {
	var result []GroupedMediaFile

	// Query to find all videos in the specified directory
	rows, err := db.conn.Query(`
		SELECT id, path, file_type, scan_time
		FROM videos
		WHERE path LIKE ? || '%'
	`, dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to query videos: %v", err)
	}
	defer rows.Close()

	// Map to store video IDs and paths
	videoMap := make(map[int64]string)
	mediaMap := make(map[string]*GroupedMediaFile)

	// Process video results
	for rows.Next() {
		var id int64
		var path, fileType string
		var scanTime int64
		if err := rows.Scan(&id, &path, &fileType, &scanTime); err != nil {
			return nil, fmt.Errorf("failed to scan video row: %v", err)
		}
		parsedScanTime := time.Unix(scanTime, 0)
		videoMap[id] = path
		mediaMap[path] = &GroupedMediaFile{
			ScanTime:  parsedScanTime,
			VideoFile: path,
			Subtitles: []SubtitleInfo{},
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating video rows: %v", err)
	}

	// Query to get all subtitles for these videos
	subtitles, err := db.conn.Query(`
		SELECT video_id, path, track_index, language, format,
		       embedded, subtitle_type, title
		FROM subtitles
		WHERE video_id IN (
			SELECT id FROM videos WHERE path LIKE ? || '%'
		) OR path LIKE ? || '%'
	`, dirPath, dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to query subtitles: %v", err)
	}
	defer subtitles.Close()

	// Process subtitle results
	for subtitles.Next() {
		var videoID sql.NullInt64
		var path sql.NullString
		var trackIndex int
		var language, format string
		var embedded int
		var subType, title sql.NullString

		if err := subtitles.Scan(
			&videoID, &path, &trackIndex, &language, &format,
			&embedded, &subType, &title,
		); err != nil {
			return nil, fmt.Errorf("failed to scan subtitle row: %v", err)
		}

		// Create subtitle info
		subtitleInfo := SubtitleInfo{
			TrackIndex:   trackIndex,
			Language:     language,
			Format:       format,
			Embedded:     embedded == 1,
			SubtitleType: nullStringValue(subType),
			Title:        nullStringValue(title),
		}

		if path.Valid {
			subtitleInfo.Path = path.String
		}

		// Add to appropriate video or as an orphaned subtitle
		if videoID.Valid {
			if videoPath, ok := videoMap[videoID.Int64]; ok {
				if media, exists := mediaMap[videoPath]; exists {
					media.Subtitles = append(media.Subtitles, subtitleInfo)
				}
			}
		} else if path.Valid {
			// This is an orphaned subtitle
			pathDir := filepath.Dir(path.String)
			key := fmt.Sprintf("orphaned-%s", pathDir)
			if _, exists := mediaMap[key]; !exists {
				mediaMap[key] = &GroupedMediaFile{
					Subtitles: []SubtitleInfo{},
				}
			}
			mediaMap[key].Subtitles = append(mediaMap[key].Subtitles, subtitleInfo)
		}
	}
	if err = subtitles.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subtitle rows: %v", err)
	}

	// Convert map to slice
	for _, media := range mediaMap {
		result = append(result, *media)
	}

	return result, nil
}

// PruneOldEntries removes entries that weren't updated in the latest scan
func (db *DB) PruneOldEntries(scanTime int64) error {
	_, err := db.conn.Exec("DELETE FROM videos WHERE scan_time < ?", scanTime)
	return err
}

// GetCachedMediaFile retrieves a specific media file by path
func (db *DB) GetCachedMediaFile(videoPath string) (*GroupedMediaFile, error) {
	// First check if the video exists in the database
	var videoID int64
	var fileType string

	err := db.conn.QueryRow(
		"SELECT id, file_type FROM videos WHERE path = ?",
		videoPath,
	).Scan(&videoID, &fileType)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No video found
		}
		return nil, fmt.Errorf("failed to query video: %v", err)
	}

	// Create the GroupedMediaFile
	result := &GroupedMediaFile{
		VideoFile: videoPath,
		Subtitles: []SubtitleInfo{},
	}

	// Get all subtitles for this video
	rows, err := db.conn.Query(`
		SELECT path, track_index, language, format,
		       embedded, subtitle_type, title
		FROM subtitles
		WHERE video_id = ?
	`, videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to query subtitles: %v", err)
	}
	defer rows.Close()

	// Process subtitle results
	for rows.Next() {
		var path sql.NullString
		var trackIndex int
		var language, format string
		var embedded int
		var subType, title sql.NullString

		if err := rows.Scan(
			&path, &trackIndex, &language, &format,
			&embedded, &subType, &title,
		); err != nil {
			return nil, fmt.Errorf("failed to scan subtitle row: %v", err)
		}

		// Create subtitle info
		subtitleInfo := SubtitleInfo{
			TrackIndex:   trackIndex,
			Language:     language,
			Format:       format,
			Embedded:     embedded == 1,
			SubtitleType: nullStringValue(subType),
			Title:        nullStringValue(title),
		}

		if path.Valid {
			subtitleInfo.Path = path.String
		}

		result.Subtitles = append(result.Subtitles, subtitleInfo)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subtitle rows: %v", err)
	}

	return result, nil
}

// Helper functions for SQL NULL handling
func sqlNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func sqlNullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}

func nullStringValue(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

// FindMediaFilesWithCache tries to retrieve media files from cache first,
// then falls back to the filesystem if needed
func FindMediaFilesWithCache(db *DB, dirPath string) ([]GroupedMediaFile, error) {
	// Try to get from cache first
	cachedFiles, err := db.GetCachedMediaFiles(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached media files: %v", err)
	}

	// If we found cached files, return them
	if len(cachedFiles) > 0 {
		return cachedFiles, nil
	}

	// Otherwise, scan the filesystem
	mediaFiles, err := FindMediaFiles(dirPath, nil)
	if err != nil {
		return nil, err
	}

	// Cache the results for future use
	if len(mediaFiles) > 0 {
		if err := db.CacheMediaFiles(mediaFiles); err != nil {
			// Log the error but continue
			slog.Warn("Failed to cache media files", "error", err)
		}
	}

	return mediaFiles, nil
}

// RefreshMediaFilesCache rescans the directory and updates the cache
func RefreshMediaFilesCache(db *DB, dirPath string) ([]GroupedMediaFile, error) {
	// Scan the filesystem
	mediaFiles, err := FindMediaFiles(dirPath, nil)
	if err != nil {
		return nil, err
	}

	// Cache the results
	scanTime := time.Now().Unix()
	if err := db.CacheMediaFiles(mediaFiles); err != nil {
		return nil, fmt.Errorf("failed to cache media files: %v", err)
	}

	// Prune old entries
	if err := db.PruneOldEntries(scanTime); err != nil {
		// Log the error but continue
		slog.Warn("Failed to prune old entries", "error", err)
	}

	return mediaFiles, nil
}
