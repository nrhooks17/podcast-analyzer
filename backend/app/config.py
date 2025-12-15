"""
Configuration management for the Podcast Analyzer application.
Handles environment variables and application settings.
"""

import os
from functools import lru_cache
from typing import List

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application configuration settings loaded from environment variables."""
    
    # Database configuration
    database_url: str = os.getenv(
        "DATABASE_URL", 
        "postgresql://postgres:postgres@localhost:5432/podcast_analyzer"
    )
    
    # Anthropic API configuration
    anthropic_api_key: str = os.getenv("ANTHROPIC_API_KEY", "")
    
    # Serper API configuration for web search
    serper_api_key: str = os.getenv("SERPER_API_KEY", "")
    
    # Kafka configuration
    kafka_bootstrap_servers: str = os.getenv("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
    kafka_topic_analysis: str = "analysis-jobs"
    
    # File storage configuration
    storage_path: str = os.getenv("STORAGE_PATH", "/app/storage/transcripts")
    max_file_size: int = 10 * 1024 * 1024  # 10MB
    allowed_extensions: List[str] = [".txt", ".json"]
    
    # Logging configuration
    log_level: str = os.getenv("LOG_LEVEL", "INFO")
    
    # CORS configuration
    cors_origins_str: str = os.getenv("CORS_ORIGINS", "http://localhost:3000")
    
    @property
    def cors_origins(self) -> List[str]:
        """Parse CORS origins from environment variable."""
        return [origin.strip() for origin in self.cors_origins_str.split(",")]
    
    # AI model configuration
    claude_model: str = "claude-sonnet-4-20250514"
    summary_max_words: int = 300
    summary_min_words: int = 200
    
    class Config:
        env_file = ".env"
        case_sensitive = False


@lru_cache()
def get_settings() -> Settings:
    """Get cached application settings."""
    return Settings()