# Go Backend Implementation

This is a Go port of the Python FastAPI backend, implementing the same API endpoints and functionality with improved performance and type safety.

## Architecture

The Go backend follows standard Go project structure:

```
backend-golang/
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── config/          # Configuration management
│   ├── models/          # Database models (GORM)
│   ├── services/        # Business logic layer
│   ├── handlers/        # HTTP handlers (Gin)
│   └── middleware/      # HTTP middleware
├── pkg/
│   ├── logger/          # Structured logging
│   └── kafka/           # Kafka integration
└── migrations/          # Database migrations
```

## Key Features

- **Framework**: Gin HTTP framework for high performance
- **ORM**: GORM for database operations with PostgreSQL
- **Messaging**: Segmentio Kafka Go client
- **Logging**: Structured JSON logging with correlation IDs
- **Configuration**: Environment-based configuration
- **Type Safety**: Full Go type safety with proper error handling

## API Compatibility

The Go backend exposes the same REST API as the Python backend but on port **8001** (vs 8000 for Python):

- `POST /api/transcripts/` - Upload transcript
- `GET /api/transcripts/` - List transcripts 
- `GET /api/transcripts/:id` - Get transcript
- `DELETE /api/transcripts/:id` - Delete transcript
- `POST /api/analyze/:transcript_id` - Start analysis
- `GET /api/jobs/:job_id/status` - Check job status
- `GET /api/results/:analysis_id` - Get analysis results
- `GET /api/results/` - List analysis results
- `GET /health` - Health check

## Environment Variables

Same as Python backend plus:

- `SERVER_PORT` - Server port (default: 8001)

## Running the Go Backend

### With Docker Compose (Recommended)

Uncomment the `backend-golang` service in `docker-compose.yaml` and run:

```bash
docker-compose up backend-golang
```

### Locally for Development

```bash
cd backend-golang

# Install dependencies
go mod tidy

# Run the server
go run cmd/server/main.go
```

The server will start on http://localhost:8001

## Performance Benefits

- **Lower Memory Usage**: Go's efficient memory management
- **Faster Startup**: No interpreter overhead
- **Better Concurrency**: Go's goroutine-based concurrency model
- **Type Safety**: Compile-time error checking
- **Single Binary**: Easy deployment without dependencies

## Database Models

Uses GORM for ORM with the same database schema as Python:

- `Transcript` - File metadata and content
- `AnalysisResult` - Analysis job status and results  
- `FactCheck` - Individual fact-check results

## Testing

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## Building

```bash
# Build binary
go build -o podcast-analyzer cmd/server/main.go

# Build for production
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o podcast-analyzer cmd/server/main.go
```

## Monitoring

- Health endpoint: `GET /health`
- Structured JSON logs with correlation IDs
- Request/response logging middleware
- Database connection health checks