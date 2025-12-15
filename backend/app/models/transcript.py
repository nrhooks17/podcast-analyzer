"""
SQLAlchemy model for podcast transcripts.
"""

import uuid
from datetime import datetime
from sqlalchemy import Column, String, Integer, DateTime, Text
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship

from app.db.database import Base


class Transcript(Base):
    """
    Database model for uploaded podcast transcripts.
    
    Stores metadata about uploaded transcript files including
    file information, content hash for deduplication, and upload timestamp.
    """
    
    __tablename__ = "transcripts"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4, index=True)
    filename = Column(String(255), nullable=False, index=True)
    file_path = Column(String(500), nullable=False)
    content_hash = Column(String(64), nullable=False, unique=True, index=True)
    word_count = Column(Integer, nullable=False)
    uploaded_at = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)
    transcript_metadata = Column(JSONB, nullable=True)  # Optional podcast metadata from JSON files

    # Relationship to analysis results
    analyses = relationship("AnalysisResult", back_populates="transcript", cascade="all, delete-orphan")

    def __repr__(self) -> str:
        return f"<Transcript(id={self.id}, filename='{self.filename}', word_count={self.word_count})>"