# Architecture Overview

Podcast transcript analysis application using AI.

## System Components

```
Frontend (React) → Backend (Go) → Database (PostgreSQL)
                        ↓
                  Kafka Queue → Analysis Worker → Claude AI
```

## Core Services

- **Frontend**: React app for file upload and results display
- **Backend**: Go REST API with validation and job management  
- **Worker**: Kafka consumer running AI analysis agents
- **Database**: PostgreSQL for metadata and results storage
- **Queue**: Apache Kafka for asynchronous job processing

## Data Flow

### File Upload
```
User → Frontend → Backend API → Validation → Storage → Database
```

### Analysis Processing  
```
Frontend → Backend API → Kafka → Worker → AI Agents → Database
```

**AI Agent Pipeline:**
1. **Summarizer**: Generates 200-300 word summary
2. **Takeaway Extractor**: Identifies key insights 
3. **Fact Checker**: Verifies claims with web search

### Status Updates
```
Frontend polls Backend API every 3 seconds until complete
```

## Key Technologies

- **Frontend**: React 18 + Vite
- **Backend**: Go 1.23+  
- **Database**: PostgreSQL 15
- **Queue**: Apache Kafka
- **AI**: Claude Sonnet 4
- **Storage**: Docker volumes
- **BaseAgent**: Abstract class with Claude API integration
- **SummarizerAgent**: Generates professional summaries
- **TakeawayExtractorAgent**: Extracts key insights
- **FactCheckerAgent**: Verifies factual claims

**Processing Flow:**
1. Consume job message from Kafka
2. Load transcript content from storage
3. Execute agents sequentially (summary → takeaways → fact-checks)
4. Save results to database
5. Update job status

### Database Schema

**PostgreSQL Tables:**

```sql
-- Transcript metadata and file information
CREATE TABLE transcripts (
    id UUID PRIMARY KEY,
    filename VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    content_hash VARCHAR(64) NOT NULL UNIQUE,
    word_count INTEGER NOT NULL,
    uploaded_at TIMESTAMP DEFAULT NOW(),
    metadata JSONB
);

-- Analysis jobs and results
CREATE TABLE analysis_results (
    id UUID PRIMARY KEY,
    transcript_id UUID REFERENCES transcripts(id),
    job_id UUID NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    summary TEXT,
    takeaways JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP,
    error_message TEXT
);

-- Individual fact-check results
CREATE TABLE fact_checks (
    id UUID PRIMARY KEY,
    analysis_id UUID REFERENCES analysis_results(id),
    claim TEXT NOT NULL,
    verdict VARCHAR(20) NOT NULL,
    confidence FLOAT NOT NULL,
    evidence TEXT,
    sources JSONB,
    checked_at TIMESTAMP DEFAULT NOW()
);
```

## AI Agent Architecture

### Base Agent Pattern

All AI agents inherit from `BaseAgent` which provides:
- Claude API client management
- Error handling and retry logic
- Structured logging with correlation IDs
- Response parsing and validation

### Agent Execution Model

**Sequential Processing:**
1. **Summarizer** runs first, processes full transcript
2. **Takeaway Extractor** can use summary for context
3. **Fact Checker** runs independently on original transcript

**Why Sequential?**
- Ensures consistent processing order
- Allows context sharing between agents
- Simplifies error handling and recovery
- Predictable resource usage

### Claude API Integration

**Model**: Claude Sonnet 4 (claude-sonnet-4-20250514)
**Configuration:**
- Temperature: 0.1 (low for consistency)
- Max tokens: 4000
- Timeout: 30 seconds
- Retry logic: 3 attempts with exponential backoff

## Scalability Considerations

### Horizontal Scaling

**Frontend:**
- Stateless React app can be replicated behind load balancer
- CDN for static assets

**Backend:**
- Multiple FastAPI instances behind load balancer
- Shared PostgreSQL database
- Sticky sessions not required

**Workers:**
- Multiple worker instances in Kafka consumer group
- Automatic load balancing via Kafka partitioning
- Independent scaling based on queue depth

### Performance Optimization

**Database:**
- Connection pooling (10 connections per instance)
- Query optimization with proper indexes
- Read replicas for analytics queries

**Caching:**
- Redis for frequently accessed analysis results
- File system caching for transcript content
- Claude API response caching for duplicate content

**Message Queue:**
- Kafka partitioning for parallel processing
- Dead letter queues for failed jobs
- Message compression for large transcripts

## Security Architecture

### Data Protection

**File Storage:**
- Volume-based storage with restricted access
- No direct URL access to uploaded files
- Automatic cleanup of failed uploads

**Database:**
- Parameterized queries via SQLAlchemy ORM
- Connection encryption (TLS)
- Regular backup and point-in-time recovery

**API Security:**
- CORS configuration for known origins
- Input validation via Pydantic schemas
- File type and size restrictions
- Rate limiting (recommended for production)

### Error Handling

**Graceful Degradation:**
- Partial results returned if individual agents fail
- Detailed error logging with correlation IDs
- User-friendly error messages (no internal details)

**Circuit Breakers:**
- Claude API timeout and retry logic
- Database connection health checks
- Kafka connection monitoring

## Monitoring and Observability

### Structured Logging

**Log Format:**
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "correlation_id": "abc-123",
  "event": "file_upload_received",
  "filename": "podcast.txt",
  "size_bytes": 15420
}
```

**Key Events Tracked:**
- File uploads and validations
- Job creation and status changes
- Agent execution start/completion
- Claude API calls and responses
- Database operations

### Health Checks

**Application Health:**
- `/health` endpoint for basic service status
- Database connectivity verification
- Kafka producer/consumer health

**Infrastructure Health:**
- PostgreSQL connection pooling status
- Kafka broker connectivity
- File storage volume capacity

## Deployment Architecture

### Development Environment

```yaml
services:
  - frontend (React dev server)
  - backend (Go with hot reload)
  - kafka-worker (Go with hot reload)
  - postgres (PostgreSQL 15)
  - zookeeper (Kafka dependency)
  - kafka (Apache Kafka)
```

### Production Considerations

**Container Orchestration:**
- Kubernetes with pod autoscaling
- Helm charts for deployment management
- Resource limits and requests

**Load Balancing:**
- nginx ingress for frontend
- Application Load Balancer for backend APIs
- Internal load balancing for database connections

**Data Persistence:**
- PostgreSQL cluster with replication
- Persistent volumes for file storage
- Automated backups and disaster recovery
