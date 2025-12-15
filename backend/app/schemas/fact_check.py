"""
Pydantic schemas for fact-check related data structures.
"""

from datetime import datetime
from typing import Optional, List
from uuid import UUID

from pydantic import BaseModel, Field


class FactCheckCreate(BaseModel):
    """Schema for creating a fact-check record."""
    analysis_id: UUID
    claim: str = Field(..., min_length=1)
    verdict: str = Field(..., regex="^(true|false|partially_true|unverifiable)$")
    confidence: float = Field(..., ge=0.0, le=1.0)
    evidence: Optional[str] = None
    sources: Optional[List[str]] = None


class FactCheckResponse(BaseModel):
    """Schema for fact-check API responses."""
    id: UUID
    analysis_id: UUID
    claim: str
    verdict: str
    confidence: float
    evidence: Optional[str] = None
    sources: Optional[List[str]] = None
    checked_at: datetime

    class Config:
        from_attributes = True