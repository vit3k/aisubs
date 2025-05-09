# Docker Setup for AISubs2

AISubs2 is a service for extracting subtitles from video files and translating them using OpenAI's API. This document explains how to run AISubs2 using Docker.

## Project Structure

The project is organized as follows:
- `src/`: Contains all Go source code files
- `data/`: Directory for storing video and subtitle files to be processed
- `Dockerfile` and `docker-compose.yml`: Docker configuration files

## Prerequisites

- Docker and Docker Compose installed on your system
- An OpenAI API key

## Getting Started

1. Clone this repository and navigate to the aisubs2 directory.

2. Set your OpenAI API key as an environment variable:

   ```bash
   export OPENAI_API_KEY=your_openai_api_key
   ```

   Or create a `.env` file in the same directory as your `docker-compose.yml` with:

   ```
   OPENAI_API_KEY=your_openai_api_key
   ```

3. Build and start the service:

   ```bash
   docker-compose up -d
   ```

4. Access the web service at `http://localhost:8080`.

## Usage

### Using the Web Service

The web service exposes several endpoints:

- `GET /subtitles?path=/app/data/video.mkv`: List subtitle tracks in a video file
- `POST /translate`: Start a subtitle translation job with payload:
  ```json
  {
    "path": "/app/data/video.mkv", 
    "track_index": 0
  }
  ```
- `GET /job?id=<job_id>`: Check job status with the job ID returned from the translate endpoint

### Working with Files

The Docker setup maps a `./data` directory from your host to `/app/data` inside the container. Place your video or subtitle files in this directory to process them.

For example, if you have a video file called `movie.mkv`, place it in the `./data` directory and then use the path `/app/data/movie.mkv` when making API requests.

Note: The application's source code is located in the `src/` directory, but you don't need to modify these files unless you want to customize the application.

### Using the Command Line Interface

You can also use the command line interface by overriding the Docker entrypoint:

```bash
docker-compose run --rm aisubs2 ./aisubs2 /app/data/subtitles.srt
```

Or for video files:

```bash
docker-compose run --rm aisubs2 ./aisubs2 /app/data/video.mkv
```

The translated subtitles will be saved to the same directory with `.pl` suffix.

## Configuration

You can adjust resource limits in the `docker-compose.yml` file if needed.

## Troubleshooting

- **FFmpeg Issues**: The container includes FFmpeg, but if you encounter issues, make sure your video files are in supported formats.
- **API Key**: Ensure your OpenAI API key is correctly set in the environment.
- **Permission Issues**: If you encounter permission issues with the data directory, check the ownership and permissions of the `./data` directory on your host system.