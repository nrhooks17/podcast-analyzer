"""
SQLAlchemy model for analysis results.
"""

import uuid
from datetime import datetime
from sqlalchemy import Column, String, DateTime, Text, ForeignKey
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship

from app.db.database import Base


class AnalysisResult(Base):
    """
    Database model for podcast analysis results.
    
    Stores the results of AI analysis including summary, takeaways,
    and processing status. Links to the source transcript.
    """
    
    __tablename__ = "analysis_results"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4, index=True)
    transcript_id = Column(UUID(as_uuid=True), ForeignKey("transcripts.id"), nullable=False, index=True)
    job_id = Column(UUID(as_uuid=True), nullable=False, unique=True, index=True)
    status = Column(String(20), nullable=False, default="pending", index=True)  # pending, processing, completed, failed
    summary = Column(Text, nullable=True)
    takeaways = Column(JSONB, nullable=True)  # Array of key takeaways
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)
    completed_at = Column(DateTime, nullable=True)
    error_message = Column(Text, nullable=True)

    # Relationships
    transcript = relationship("Transcript", back_populates="analyses")
    fact_checks = relationship("FactCheck", back_populates="analysis", cascade="all, delete-orphan")

    def __repr__(self) -> str:
        return f"<AnalysisResult(id={self.id}, job_id={self.job_id}, status='{self.status}')>"