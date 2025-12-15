# Podcast Analyzer

An AI-powered podcast transcript analysis tool for ad agencies. Automatically generates summaries, extracts key takeaways, and fact-checks claims from podcast transcripts using Claude AI.

## Features

- **📝 Transcript Upload**: Support for .txt and .json transcript files
- **🤖 AI Analysis**: Powered by Claude Sonnet 4 for intelligent content analysis
- **📋 Summary Generation**: 200-300 word professional summaries
- **💡 Key Takeaways**: Extraction of important insights and actionable points
- **✅ Fact Checking**: Verification of factual claims with confidence scores
- **📊 Professional UI**: Clean, business-appropriate interface
- **⚡ Asynchronous Processing**: Kafka-based job queue for scalable processing
- **🔍 Analysis History**: Track and review past analyses

## Technology Stack

| Component | Technology |
|-----------|------------|
| Frontend | React 18 with Vite |
| Backend | FastAPI (Python) |
| Database | PostgreSQL 15 |
| Job Queue | Apache Kafka |
| AI Engine | Claude Sonnet 4 (Anthropic) |
| File Storage | Local volume (simulating S3) |
| Containerization | Docker Compose |

## Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Frontend  │    │   Backend   │    │  Database   │
│   (React)   │◄──►│  (FastAPI)  │◄──►│(PostgreSQL) │
└─────────────┘    └─────────────┘    └─────────────┘
                           │
                           ▼
                   ┌─────────────┐    ┌─────────────┐
                   │    Kafka    │◄──►│   Worker    │
                   │   (Queue)   │    │ (Analysis)  │
                   └─────────────┘    └─────────────┘
                                              │
                                              ▼
                                      ┌─────────────┐
                                      │ AI Agents   │
                                      │  (Claude)   │
                                      └─────────────┘
```

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Anthropic API key ([get one here](https://console.anthropic.com/))

### 1. Clone Repository

```bash
git clone <repository-url>
cd podcast-analyzer
```

### 2. Environment Setup

```bash
# Copy environment template
cp .env.example .env

# Edit .env and add your Anthropic API key
ANTHROPIC_API_KEY=your_anthropic_api_key_here
```

### 3. Start Application

```bash
# Start all services
docker-compose up

# Or run in background
docker-compose up -d
```

### 4. Access Application

- **Frontend**: http://localhost:3000
- **API Documentation**: http://localhost:8000/docs
- **Health Check**: http://localhost:8000/health

## Usage Guide

### Uploading Transcripts

1. **Navigate to Upload Page**: Go to http://localhost:3000
2. **Select File**: Drag and drop or click to select a transcript file
3. **Supported Formats**:
   - `.txt`: Plain text with optional timestamps
   - `.json`: Structured format with metadata

#### Example Formats

**Text Format (.txt):**
```
[00:00:00] Host: Welcome to the show. Today we're discussing space exploration.
[00:00:15] Guest: Thanks for having me. NASA's Mars mission launches in 2026.
[00:01:02] Host: Absolutely. I've also heard Bitcoin will reach $200K by 2025.
```

**JSON Format (.json):**
```json
{
  "title": "Space and Finance Talk",
  "date": "2024-01-15",
  "speakers": ["Host", "Guest"],
  "transcript": [
    {"timestamp": "00:00:00", "speaker": "Host", "text": "Welcome to the show..."},
    {"timestamp": "00:00:15", "speaker": "Guest", "text": "Thanks for having me..."}
  ]
}
```

### Analysis Process

1. **Upload**: Upload your transcript file
2. **Start Analysis**: Click "Start Analysis" to begin processing
3. **Monitor Progress**: Track status updates in real-time
4. **View Results**: See summary, takeaways, and fact-checks when complete

### Analysis Results

- **Summary**: Professional 200-300 word overview
- **Key Takeaways**: Bulleted list of important insights
- **Fact Checks**: Table showing:
  - Claims identified in the transcript
  - Verification status (True/False/Partially True/Unverifiable)
  - Confidence scores (0-100%)
  - Supporting evidence and sources

## API Documentation

### Core Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/transcripts/` | Upload transcript file |
| `GET` | `/api/transcripts/` | List all transcripts |
| `GET` | `/api/transcripts/{id}` | Get transcript details |
| `DELETE` | `/api/transcripts/{id}` | Delete transcript |
| `POST` | `/api/analyze/{transcript_id}` | Start analysis job |
| `GET` | `/api/jobs/{job_id}/status` | Check job status |
| `GET` | `/api/results/{analysis_id}` | Get analysis results |

### Authentication

Currently no authentication required. For production deployment, implement:
- API key authentication
- Rate limiting
- User management

## Development

### Backend Development

```bash
# Navigate to backend directory
cd backend

# Install dependencies
pip install -r requirements.txt

# Run development server
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

### Frontend Development

```bash
# Navigate to frontend directory
cd frontend

# Install dependencies
npm install

# Start development server
npm run dev
```

### Running Tests

**Backend Tests:**
```bash
cd backend
pytest
```

**Frontend Tests:**
```bash
cd frontend
npm test
```

### Database Management

```bash
# View database logs
docker-compose logs postgres

# Connect to database
docker-compose exec postgres psql -U postgres -d podcast_analyzer

# Reset database (WARNING: Deletes all data)
docker-compose down -v
docker-compose up
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ANTHROPIC_API_KEY` | Claude AI API key | Required |
| `DATABASE_URL` | PostgreSQL connection string | Auto-configured |
| `KAFKA_BOOTSTRAP_SERVERS` | Kafka broker addresses | `kafka:9092` |
| `STORAGE_PATH` | File storage location | `/app/storage/transcripts` |
| `LOG_LEVEL` | Logging level | `INFO` |
| `CORS_ORIGINS` | Allowed CORS origins | `http://localhost:3000` |

### File Limits

- **Maximum file size**: 10 MB
- **Supported formats**: `.txt`, `.json`
- **Character encoding**: UTF-8

## Troubleshooting

### Common Issues

**1. Services won't start**
```bash
# Check Docker resources
docker system df

# Check service logs
docker-compose logs [service-name]

# Restart services
docker-compose restart
```

**2. Upload fails**
- Verify file format (.txt or .json)
- Check file size (< 10MB)
- Ensure valid UTF-8 encoding

**3. Analysis stuck in processing**
```bash
# Check worker logs
docker-compose logs kafka-worker

# Check Kafka connectivity
docker-compose logs kafka

# Restart worker
docker-compose restart kafka-worker
```

**4. Database connection issues**
```bash
# Check database health
docker-compose exec postgres pg_isready

# View database logs
docker-compose logs postgres
```

### Performance Optimization

**For Production:**
1. Use PostgreSQL connection pooling
2. Configure Kafka for high availability
3. Implement Redis caching for API responses
4. Use CDN for static assets
5. Enable gzip compression

## Security Considerations

### Current Implementation
- File validation and size limits
- SQL injection protection via SQLAlchemy
- CORS configuration
- Input sanitization

### Production Hardening
- [ ] Implement authentication/authorization
- [ ] Add rate limiting
- [ ] Use HTTPS/TLS encryption
- [ ] Secure Kafka with SASL/SSL
- [ ] Database encryption at rest
- [ ] Input validation enhancements
- [ ] Security headers (HSTS, CSP, etc.)

## Monitoring and Logging

### Structured Logging
All services use structured JSON logging with correlation IDs for request tracing.

### Key Metrics to Monitor
- Upload success/failure rates
- Analysis processing times
- Queue depth and processing lag
- Database query performance
- Claude API response times and token usage

### Health Checks
- **Backend**: `GET /health`
- **Database**: PostgreSQL connection validation
- **Kafka**: Topic creation and message production

## Contributing

1. **Fork the repository**
2. **Create feature branch**: `git checkout -b feature/amazing-feature`
3. **Commit changes**: `git commit -m 'Add amazing feature'`
4. **Run tests**: Ensure all tests pass
5. **Push to branch**: `git push origin feature/amazing-feature`
6. **Open Pull Request**

### Code Standards
- **Python**: Follow PEP 8, use type hints
- **JavaScript**: Use ESLint configuration
- **Tests**: Maintain >80% code coverage
- **Documentation**: Update README for new features

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) file for details.

## Support

For questions, issues, or support:
1. Check the [troubleshooting section](#troubleshooting)
2. Search [existing issues](../../issues)
3. Create a [new issue](../../issues/new) with detailed description

---

**Built with ❤️ using FastAPI, React, and Claude AI**# podcast-analyzer
