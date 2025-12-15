"""
Pydantic schemas for transcript-related API requests and responses.
"""

from datetime import datetime
from typing import Optional, Dict, Any
from uuid import UUID

from pydantic import BaseModel, Field


class TranscriptCreate(BaseModel):
    """Schema for creating a new transcript record."""
    filename: str = Field(..., min_length=1, max_length=255)
    content_hash: str = Field(..., min_length=64, max_length=64)
    word_count: int = Field(..., ge=0)
    transcript_metadata: Optional[Dict[str, Any]] = None


class TranscriptResponse(BaseModel):
    """Schema for transcript API responses."""
    id: UUID
    filename: str
    file_path: str
    content_hash: str
    word_count: int
    uploaded_at: datetime
    transcript_metadata: Optional[Dict[str, Any]] = None

    class Config:
        from_attributes = True


class TranscriptListResponse(BaseModel):
    """Schema for paginated transcript list responses."""
    transcripts: list[TranscriptResponse]
    total: int
    page: int
    per_page: int


class FileUploadResponse(BaseModel):
    """Schema for file upload API response."""
    transcript_id: UUID
    filename: str
    word_count: int
    message: str