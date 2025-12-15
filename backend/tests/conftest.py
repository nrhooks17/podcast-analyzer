"""
Pytest configuration and fixtures for testing.
"""

import os
import pytest
import tempfile
from typing import Generator, Dict, Any
from fastapi.testclient import TestClient
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, Session
from sqlalchemy.engine import Engine
from unittest.mock import Mock

from app.main import app
from app.db.database import get_db, Base
from app.config import get_settings


@pytest.fixture(scope="session")
def test_settings() -> Any:
    """Override settings for testing."""
    settings = get_settings()
    # Use in-memory SQLite for faster tests
    settings.database_url = "sqlite:///:memory:"
    settings.anthropic_api_key = "test-key"
    settings.kafka_bootstrap_servers = "localhost:9092"
    settings.storage_path = tempfile.mkdtemp()
    return settings


@pytest.fixture(scope="session")
def test_engine(test_settings: Any) -> Engine:
    """Create test database engine."""
    from sqlalchemy import String, Text
    from sqlalchemy.dialects.postgresql import UUID, JSONB
    
    # Patch PostgreSQL-specific types for SQLite compatibility
    def mock_uuid(*args: Any, **kwargs: Any) -> Any:
        return String(36)  # UUID as string
    
    def mock_jsonb(*args: Any, **kwargs: Any) -> Any:
        return Text()  # JSONB as text
    
    # Temporarily replace PostgreSQL types for SQLite
    import app.models.transcript
    import app.models.analysis
    import app.models.fact_check
    
    original_uuid = UUID
    original_jsonb = JSONB
    
    # Replace in transcript module
    app.models.transcript.UUID = mock_uuid
    app.models.transcript.JSONB = mock_jsonb
    
    # Replace in analysis module  
    app.models.analysis.UUID = mock_uuid
    app.models.analysis.JSONB = mock_jsonb
    
    # Replace in fact_check module
    app.models.fact_check.UUID = mock_uuid
    
    try:
        engine = create_engine(
            test_settings.database_url,
            connect_args={"check_same_thread": False}
        )
        Base.metadata.create_all(bind=engine)
        return engine
    finally:
        # Restore original types
        app.models.transcript.UUID = original_uuid
        app.models.transcript.JSONB = original_jsonb
        app.models.analysis.UUID = original_uuid
        app.models.analysis.JSONB = original_jsonb
        app.models.fact_check.UUID = original_uuid


@pytest.fixture(scope="function")
def test_db(test_engine: Engine) -> Generator[Session, None, None]:
    """Create test database session."""
    TestingSessionLocal = sessionmaker(
        autocommit=False, 
        autoflush=False, 
        bind=test_engine
    )
    
    session = TestingSessionLocal()
    try:
        yield session
    finally:
        session.close()


@pytest.fixture(scope="function")
def client(test_db: Session) -> Generator[TestClient, None, None]:
    """Create test client with database dependency override."""
    def override_get_db() -> Generator[Session, None, None]:
        try:
            yield test_db
        finally:
            pass

    app.dependency_overrides[get_db] = override_get_db
    with TestClient(app) as test_client:
        yield test_client
    app.dependency_overrides.clear()


@pytest.fixture
def mock_anthropic_client() -> Mock:
    """Mock Anthropic client for testing."""
    mock_client = Mock()
    mock_response = Mock()
    mock_response.content = [Mock()]
    mock_response.content[0].text = "Test response from Claude"
    mock_response.usage.input_tokens = 100
    mock_response.usage.output_tokens = 50
    mock_client.messages.create.return_value = mock_response
    return mock_client


@pytest.fixture
def sample_transcript_txt() -> str:
    """Sample TXT transcript content."""
    return """[00:00:00] Host: Welcome to the show. Today we're discussing space exploration.
[00:00:15] Guest: Thanks for having me. NASA's Mars mission launches in 2026, which is exciting.
[00:01:02] Host: Absolutely. I've also heard Bitcoin will reach $200K by 2025.
[00:01:30] Guest: That's, uh, speculation. But the Mars mission is confirmed."""


@pytest.fixture
def sample_transcript_json() -> Dict[str, Any]:
    """Sample JSON transcript content."""
    return {
        "title": "Space and Finance Talk",
        "date": "2024-01-15",
        "speakers": ["Host", "Guest"],
        "transcript": [
            {"timestamp": "00:00:00", "speaker": "Host", "text": "Welcome to the show. Today we're discussing space exploration."},
            {"timestamp": "00:00:15", "speaker": "Guest", "text": "Thanks for having me. NASA's Mars mission launches in 2026, which is exciting."},
            {"timestamp": "00:01:02", "speaker": "Host", "text": "Absolutely. I've also heard Bitcoin will reach $200K by 2025."},
            {"timestamp": "00:01:30", "speaker": "Guest", "text": "That's, uh, speculation. But the Mars mission is confirmed."}
        ]
    }


@pytest.fixture(autouse=True)
def cleanup_test_files(test_settings: Any) -> Generator[None, None, None]:
    """Clean up test files after each test."""
    yield
    # Clean up any files created during testing
    import shutil
    if os.path.exists(test_settings.storage_path):
        shutil.rmtree(test_settings.storage_path, ignore_errors=True)