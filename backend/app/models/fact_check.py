"""
SQLAlchemy model for fact-check results.
"""

import uuid
from datetime import datetime
from sqlalchemy import Column, String, DateTime, Text, Float, ForeignKey
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship

from app.db.database import Base


class FactCheck(Base):
    """
    Database model for individual fact-check results.
    
    Stores verification results for specific claims found in transcripts,
    including the verdict, confidence score, and supporting evidence.
    """
    
    __tablename__ = "fact_checks"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4, index=True)
    analysis_id = Column(UUID(as_uuid=True), ForeignKey("analysis_results.id"), nullable=False, index=True)
    claim = Column(Text, nullable=False)
    verdict = Column(String(20), nullable=False, index=True)  # true, false, partially_true, unverifiable
    confidence = Column(Float, nullable=False)  # 0.0 to 1.0
    evidence = Column(Text, nullable=True)
    sources = Column(JSONB, nullable=True)  # Array of source URLs
    checked_at = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)

    # Relationship
    analysis = relationship("AnalysisResult", back_populates="fact_checks")

    def __repr__(self) -> str:
        return f"<FactCheck(id={self.id}, verdict='{self.verdict}', confidence={self.confidence})>"