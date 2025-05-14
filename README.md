# AISubTranslator

AISubTranslator is a tool and web service for extracting subtitles from video files (such as MKV) and translating them into Polish using OpenAI's API. It supports both command-line and REST API usage, making it easy to process subtitles in bulk or integrate with other systems.

## Features

- **Extracts subtitles** from MKV and other video files (supports SRT, ASS, SSA formats).
- **Translates subtitles** from English (or other languages) to Polish using OpenAI.
- **Command-line interface** for batch processing.
- **RESTful web service** for integration and automation.
- **SQLite database caching** for faster media file scanning.
- **Docker support** for easy deployment.
- **Handles both video and standalone subtitle files**.

## Project Structure

- `src/`: Go source code for the CLI and web service.
- `Dockerfile`, `docker-compose.yml`: Docker configuration files.

## Prerequisites

- Go 1.21+ (for building locally)
- FFmpeg installed and available in your `PATH`
- OpenAI API key (for translation)
- Docker and Docker Compose (for containerized deployment)

## Building

### Locally

1. Install Go (1.21 or newer) and FFmpeg.
2. Clone this repository and navigate to the `aisubtranslator` directory.
3. Build the binary:

   ```bash
   cd src
   go build -o ../aisubtranslator
   ```

### With Docker

1. Ensure Docker is installed.
2. Build the Docker image:

   ```bash
   docker build -t aisubtranslator .
   ```

## Configuration

The application can be configured using a YAML configuration file. Create a file named `config.yaml` in either:
- The current directory where you run the application
- The application data directory (`~/.aisubs2/config.yaml`)

Example configuration:

```yaml
# Web service configuration
web_service:
  # Port on which the web service will listen
  port: 8080

# Media paths configuration
# Each path has a name and a file system path
media_paths:
  movies:
    path: "/path/to/your/movies"
    description: "Main movie collection"
  
  tv_shows:
    path: "/path/to/your/tv_shows"
    description: "TV series collection"
```

## Running

### Command-Line Usage

```bash
./aisubtranslator <input_file> | -s | -c <directory>
```

- `<input_file>`: Path to an MKV video or subtitle file.
  - If MKV: Extracts the first English subtitle and translates it to Polish.
  - If subtitle: Translates directly to Polish.
- `-s`: Runs the web service on the configured port (default: 8080).
- `-c <directory>`: Scans and caches media files in the specified directory.

**Example:**

```bash
./aisubtranslator mymovie.mkv
./aisubtranslator subtitles.srt
./aisubtranslator -s
./aisubtranslator -c /path/to/my/media
```

### Web Service

Start the service:

```bash
./aisubtranslator -s
```

Or with Docker Compose:

1. Set your OpenAI API key as an environment variable:

   ```bash
   export OPENAI_API_KEY=your_openai_api_key
   ```

   Or create a `.env` file with:

   ```
   OPENAI_API_KEY=your_openai_api_key
   ```

2. Start the service:

   ```bash
   docker-compose up -d
   ```

3. Access the API at [http://localhost:8080](http://localhost:8080).

### API Endpoints

- `GET /subtitles`: Get a list of available subtitles in media file.
- `POST /translate`: Translate subtitles from provided file to Polish.
- `GET /job`: Check the status of a translation job.
- `GET /media`: List available media files in a directory with available subtitles (uses cache if available).
  - Use `path=/path/to/dir` for direct path access
  - Or use `name=movies` to reference a named media path from configuration
  - Optional `refresh=true` parameter forces a fresh scan and cache update.
- `POST /cache`: Manage the media files cache (action=refresh).

## Environment Variables

- `OPENAI_API_KEY`: Your OpenAI API key (required for translation).
- Configuration file takes precedence over default values.

## Notes

- FFmpeg must be installed and accessible in your system `PATH` (automatically handled in Docker).
- Output Polish subtitles are saved alongside the input file, with `.pl` inserted before the extension.
- Temporary files are cleaned up automatically.
- Media file scanning results are cached in an SQLite database in the user's home directory (`~/.aisubs2/aisubs.db`).
- Media scanning is significantly faster on subsequent runs due to caching.

## Usage Examples

### Command-Line

Extract and translate subtitles from a video file:
```bash
./aisubtranslator mymovie.mkv
```

Translate an existing subtitle file:
```bash
./aisubtranslator subtitles.srt
```

Run the web service:
```bash
./aisubtranslator -s
```

### Docker

Build and run the service with Docker Compose:
```bash
export OPENAI_API_KEY=your_openai_api_key
docker-compose up -d
```

### Cache Usage

Manually scan and cache a directory:
```bash
./aisubtranslator -c /path/to/media
```

Use the API to force cache refresh for a directory:
```bash
curl -X POST "http://localhost:8080/cache?action=refresh&path=/path/to/media"
```

Get media files with cached data:
```bash
# Using direct path
curl "http://localhost:8080/media?path=/path/to/media"

# Using named path from configuration
curl "http://localhost:8080/media?name=movies"
```

Force a fresh scan (bypassing cache):
```bash
curl "http://localhost:8080/media?path=/path/to/media&refresh=true"
```

---

## Environment Variables

- `OPENAI_API_KEY`: Your OpenAI API key (required for translation).

## Notes

- FFmpeg must be installed and accessible in your system `PATH` (automatically handled in Docker).
- Output Polish subtitles are saved alongside the input file, with `.pl` inserted before the extension.
- Temporary files are cleaned up automatically.
- Media file scanning results are cached in an SQLite database in the user's home directory (`~/.aisubs2/aisubs.db`).
- Media scanning is significantly faster on subsequent runs due to caching.
