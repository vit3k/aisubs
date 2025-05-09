# AISubTranslator

AISubTranslator is a tool and web service for extracting subtitles from video files (such as MKV) and translating them into Polish using OpenAI's API. It supports both command-line and REST API usage, making it easy to process subtitles in bulk or integrate with other systems.

## Features

- **Extracts subtitles** from MKV and other video files (supports SRT, ASS, SSA formats).
- **Translates subtitles** from English (or other languages) to Polish using OpenAI.
- **Command-line interface** for batch processing.
- **RESTful web service** for integration and automation.
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

## Running

### Command-Line Usage

```bash
./aisubtranslator <input_file> | -s
```

- `<input_file>`: Path to an MKV video or subtitle file.
  - If MKV: Extracts the first English subtitle and translates it to Polish.
  - If subtitle: Translates directly to Polish.
- `-s`: Runs the web service on port 8080.

**Example:**

```bash
./aisubtranslator mymovie.mkv
./aisubtranslator subtitles.srt
./aisubtranslator -s
```

### Web Service

Start the service:

```bash
./aisubs -s
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
- `GET /media`: List available media files in a directory with available subtitles.

## Environment Variables

- `OPENAI_API_KEY`: Your OpenAI API key (required for translation).

## Notes

- FFmpeg must be installed and accessible in your system `PATH` (automatically handled in Docker).
- Output Polish subtitles are saved alongside the input file, with `.pl` inserted before the extension.
- Temporary files are cleaned up automatically.

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

---

## Environment Variables

- `OPENAI_API_KEY`: Your OpenAI API key (required for translation).

## Notes

- FFmpeg must be installed and accessible in your system `PATH` (automatically handled in Docker).
- Output Polish subtitles are saved alongside the input file, with `.pl` inserted before the extension.
- Temporary files are cleaned up automatically.
