# Podcast Analyzer

AI-powered transcript analysis tool. Generates summaries, extracts key takeaways, and fact-checks claims from podcast transcripts.

## Features

- Upload .txt/.json transcripts (10MB max)
- AI analysis with Claude Sonnet 4
- Summary generation (200-300 words)
- Key takeaway extraction
- Fact-checking with confidence scores
- Analysis history tracking

## Quick Start

### Prerequisites
- **Docker** (v20.10+) and **Docker Compose** (v2.0+)
- **Anthropic API key** ([get here](https://console.anthropic.com/))
- **Serper API key** ([get here](https://serper.dev/))

### For Local Development (Optional)
- **Python 3.11+** with **pip**
- **Node.js 18+** with **npm**

### Setup
```bash
# Clone and navigate
git clone <repository-url>
cd podcast-analyzer

# Configure environment
cp .env.example .env
# Edit .env and add your API keys: ANTHROPIC_API_KEY and SERPER_API_KEY

# Start application
docker-compose up -d
```

### Access
- **App**: http://localhost:3000
- **API Docs**: http://localhost:8000/docs

## Usage

1. Upload transcript file (.txt or .json)
2. Click "Start Analysis"
3. Monitor progress
4. View results: summary, takeaways, fact-checks

### Example Transcript Format

**Text (.txt):**
```
[00:00:00] Host: Welcome to today's show about space exploration.
[00:00:15] Guest: NASA's Mars mission launches in 2026.
```

**JSON (.json):**
```json
{
  "title": "Space Talk",
  "transcript": [
    {"timestamp": "00:00:00", "speaker": "Host", "text": "Welcome..."},
    {"timestamp": "00:00:15", "speaker": "Guest", "text": "NASA's Mars..."}
  ]
}
```

## Development

### Run Tests
```bash
# Backend
cd backend && pytest

# Frontend  
cd frontend && npm test
```

### Local Development
```bash
# Backend
cd backend
pip install -r requirements.txt
uvicorn app.main:app --reload

# Frontend
cd frontend
npm install
npm run dev
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes | Claude AI API key |
| `SERPER_API_KEY` | Yes | Web search for fact-checking |
| `POSTGRES_PASSWORD` | No | Database password (default: postgres) |

## Troubleshooting

**Services won't start:**
```bash
docker-compose logs [service-name]
docker-compose restart
```

**Upload fails:**
- Check file format (.txt/.json)
- Verify file size (<10MB)
- Ensure UTF-8 encoding

**Analysis stuck:**
```bash
docker-compose logs kafka-worker
docker-compose restart kafka-worker
```

## Tech Stack

- **Frontend**: React + Vite
- **Backend**: FastAPI (Python)
- **Database**: PostgreSQL
- **Queue**: Apache Kafka
- **AI**: Claude Sonnet 4
