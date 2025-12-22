# Podcast Analyzer Backend

A high-performance Go backend for analyzing podcast transcripts with AI-powered summarization and fact-checking.

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

## API Endpoints

The backend exposes the following REST API endpoints on port **8001**:

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

- `SERVER_PORT` - Server port (default: 8001)
- `DATABASE_URL` - PostgreSQL connection string
- `KAFKA_BROKERS` - Kafka broker addresses
- `ANTHROPIC_API_KEY` - Claude API key for AI processing
- `SERPER_API_KEY` - Serper API key for web search
- `LOG_LEVEL` - Logging level (DEBUG, INFO, WARN, ERROR)

## Running the Backend

### With Docker Compose (Recommended)

```bash
docker-compose up
```

### Locally for Development

```bash
# Install dependencies
go mod tidy

# Run the server
go run cmd/server/main.go
```

The server will start on http://localhost:8001

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