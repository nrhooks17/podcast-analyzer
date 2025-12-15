"""
Pydantic schemas for analysis-related API requests and responses.
"""

from datetime import datetime
from typing import Optional, List, Dict, Any
from uuid import UUID

from pydantic import BaseModel, Field


class AnalysisJobRequest(BaseModel):
    """Schema for starting an analysis job."""
    transcript_id: UUID


class AnalysisJobResponse(BaseModel):
    """Schema for analysis job creation response."""
    job_id: UUID
    transcript_id: UUID
    status: str
    message: str


class JobStatusResponse(BaseModel):
    """Schema for job status polling response."""
    job_id: UUID
    transcript_id: UUID
    status: str  # pending, processing, completed, failed
    created_at: datetime
    completed_at: Optional[datetime] = None
    error_message: Optional[str] = None

    class Config:
        from_attributes = True


class FactCheckResult(BaseModel):
    """Schema for individual fact-check results."""
    id: UUID
    claim: str
    verdict: str  # true, false, partially_true, unverifiable
    confidence: float = Field(..., ge=0.0, le=1.0)
    evidence: Optional[str] = None
    sources: Optional[List[str]] = None
    checked_at: datetime

    class Config:
        from_attributes = True


class AnalysisResultsResponse(BaseModel):
    """Schema for complete analysis results."""
    id: UUID
    job_id: UUID
    transcript_id: UUID
    status: str
    summary: Optional[str] = None
    takeaways: Optional[List[str]] = None
    fact_checks: List[FactCheckResult] = []
    created_at: datetime
    completed_at: Optional[datetime] = None
    transcript_filename: Optional[str] = None
    transcript_title: Optional[str] = None

    class Config:
        from_attributes = True


class AnalysisResultsListResponse(BaseModel):
    """Schema for paginated analysis results list."""
    results: List[AnalysisResultsResponse]
    total: int
    page: int
    per_page: int