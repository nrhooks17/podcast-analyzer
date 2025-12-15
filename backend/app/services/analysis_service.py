"""
Service layer for analysis-related business logic.
Handles analysis job creation, status tracking, and results retrieval.
"""

import uuid
from datetime import datetime
from typing import List, Optional, Tuple, Dict, Any
from sqlalchemy.orm import Session

from app.models.analysis import AnalysisResult
from app.models.fact_check import FactCheck
from app.models.transcript import Transcript
from app.schemas.analysis import AnalysisJobResponse, JobStatusResponse, AnalysisResultsResponse
from app.utils.logger import get_logger

logger = get_logger(__name__)


class AnalysisNotFoundError(Exception):
    """Raised when an analysis job is not found in the database."""
    pass


class AnalysisService:
    """
    Service class for handling analysis operations.
    
    Provides methods for creating analysis jobs, tracking status,
    and retrieving results with comprehensive logging.
    """

    def __init__(self, db: Session) -> None:
        """
        Initialize the analysis service.
        
        Args:
            db: Database session for operations
        """
        self.db: Session = db

    def create_analysis_job(self, transcript_id: uuid.UUID) -> AnalysisJobResponse:
        """
        Create a new analysis job for a transcript.
        
        Args:
            transcript_id: UUID of the transcript to analyze
            
        Returns:
            AnalysisJobResponse with job details
            
        Raises:
            AnalysisNotFoundError: If transcript is not found
        """
        # Verify transcript exists
        transcript = self.db.query(Transcript).filter(Transcript.id == transcript_id).first()
        if not transcript:
            logger.error("Transcript not found for analysis", transcript_id=transcript_id)
            raise AnalysisNotFoundError(f"Transcript {transcript_id} not found")

        # Generate job ID
        job_id = uuid.uuid4()

        # Create analysis record
        analysis = AnalysisResult(
            transcript_id=transcript_id,
            job_id=job_id,
            status="pending"
        )

        try:
            self.db.add(analysis)
            self.db.commit()
            self.db.refresh(analysis)

            logger.info("Analysis job created", 
                       job_id=job_id, 
                       transcript_id=transcript_id,
                       analysis_id=analysis.id)

            return AnalysisJobResponse(
                job_id=job_id,
                transcript_id=transcript_id,
                status="pending",
                message="Analysis job created and queued for processing"
            )

        except Exception as e:
            self.db.rollback()
            logger.error("Failed to create analysis job", 
                        transcript_id=transcript_id, 
                        error=str(e))
            raise

    def get_job_status(self, job_id: uuid.UUID) -> JobStatusResponse:
        """
        Get the status of an analysis job.
        
        Args:
            job_id: UUID of the analysis job
            
        Returns:
            JobStatusResponse with current job status
            
        Raises:
            AnalysisNotFoundError: If job is not found
        """
        analysis = self.db.query(AnalysisResult).filter(AnalysisResult.job_id == job_id).first()
        if not analysis:
            logger.error("Analysis job not found", job_id=job_id)
            raise AnalysisNotFoundError(f"Analysis job {job_id} not found")

        logger.info("Retrieved job status", 
                   job_id=job_id, 
                   status=analysis.status,
                   analysis_id=analysis.id)

        return JobStatusResponse.from_orm(analysis)

    def update_job_status(self, job_id: uuid.UUID, status: str, error_message: str = None) -> None:
        """
        Update the status of an analysis job.
        
        Args:
            job_id: UUID of the analysis job
            status: New status (pending, processing, completed, failed)
            error_message: Optional error message if status is failed
        """
        analysis = self.db.query(AnalysisResult).filter(AnalysisResult.job_id == job_id).first()
        if not analysis:
            logger.error("Analysis job not found for status update", job_id=job_id)
            return

        analysis.status = status
        if error_message:
            analysis.error_message = error_message
        if status in ["completed", "failed"]:
            analysis.completed_at = datetime.utcnow()

        try:
            self.db.commit()
            logger.info("Updated job status", 
                       job_id=job_id, 
                       status=status,
                       has_error=bool(error_message))

        except Exception as e:
            self.db.rollback()
            logger.error("Failed to update job status", 
                        job_id=job_id, 
                        status=status, 
                        error=str(e))

    def save_analysis_results(self, job_id: uuid.UUID, summary: str, takeaways: List[str]) -> AnalysisResult:
        """
        Save the analysis results (summary and takeaways) to the database.
        
        Args:
            job_id: UUID of the analysis job
            summary: Generated summary text
            takeaways: List of key takeaway strings
            
        Returns:
            Updated AnalysisResult model instance
            
        Raises:
            AnalysisNotFoundError: If job is not found
        """
        analysis = self.db.query(AnalysisResult).filter(AnalysisResult.job_id == job_id).first()
        if not analysis:
            logger.error("Analysis job not found for results save", job_id=job_id)
            raise AnalysisNotFoundError(f"Analysis job {job_id} not found")

        analysis.summary = summary
        analysis.takeaways = takeaways

        try:
            self.db.commit()
            self.db.refresh(analysis)
            
            logger.info("Saved analysis results", 
                       job_id=job_id,
                       analysis_id=analysis.id,
                       summary_length=len(summary),
                       takeaways_count=len(takeaways))

            return analysis

        except Exception as e:
            self.db.rollback()
            logger.error("Failed to save analysis results", 
                        job_id=job_id, 
                        error=str(e))
            raise

    def save_fact_checks(self, analysis_id: uuid.UUID, fact_checks: List[Dict[str, Any]]) -> List[FactCheck]:
        """
        Save fact-check results to the database.
        
        Args:
            analysis_id: UUID of the analysis record
            fact_checks: List of fact-check result dictionaries
            
        Returns:
            List of created FactCheck model instances
        """
        created_fact_checks = []

        for fact_check_data in fact_checks:
            fact_check = FactCheck(
                analysis_id=analysis_id,
                claim=fact_check_data["claim"],
                verdict=fact_check_data["verdict"],
                confidence=fact_check_data["confidence"],
                evidence=fact_check_data.get("evidence"),
                sources=fact_check_data.get("sources", [])
            )
            
            self.db.add(fact_check)
            created_fact_checks.append(fact_check)

        try:
            self.db.commit()
            
            for fact_check in created_fact_checks:
                self.db.refresh(fact_check)

            logger.info("Saved fact-check results", 
                       analysis_id=analysis_id,
                       fact_checks_count=len(created_fact_checks))

            return created_fact_checks

        except Exception as e:
            self.db.rollback()
            logger.error("Failed to save fact-check results", 
                        analysis_id=analysis_id, 
                        error=str(e))
            raise

    def get_analysis_results(self, analysis_id: uuid.UUID) -> AnalysisResultsResponse:
        """
        Get complete analysis results including fact-checks.
        
        Args:
            analysis_id: UUID of the analysis record
            
        Returns:
            AnalysisResultsResponse with complete results
            
        Raises:
            AnalysisNotFoundError: If analysis is not found
        """
        # Join with transcript to get filename and metadata
        result = (self.db.query(AnalysisResult, Transcript)
                 .join(Transcript, AnalysisResult.transcript_id == Transcript.id)
                 .filter(AnalysisResult.id == analysis_id)
                 .first())
        
        if not result:
            logger.error("Analysis not found", analysis_id=analysis_id)
            raise AnalysisNotFoundError(f"Analysis {analysis_id} not found")

        analysis, transcript = result

        logger.info("Retrieved analysis results", 
                   analysis_id=analysis_id,
                   status=analysis.status,
                   fact_checks_count=len(analysis.fact_checks))

        # Create response with transcript information
        response_data = {
            'id': analysis.id,
            'job_id': analysis.job_id,
            'transcript_id': analysis.transcript_id,
            'status': analysis.status,
            'summary': analysis.summary,
            'takeaways': analysis.takeaways,
            'created_at': analysis.created_at,
            'completed_at': analysis.completed_at,
            'transcript_filename': transcript.filename,
            'transcript_title': None,  # Extract from metadata if needed
            'fact_checks': [
                {
                    'id': fc.id,
                    'claim': fc.claim,
                    'verdict': fc.verdict,
                    'confidence': fc.confidence,
                    'evidence': fc.evidence,
                    'sources': fc.sources,
                    'checked_at': fc.checked_at
                }
                for fc in analysis.fact_checks
            ]
        }

        # Extract title from transcript metadata if available
        if transcript.transcript_metadata and isinstance(transcript.transcript_metadata, dict):
            response_data['transcript_title'] = transcript.transcript_metadata.get('title')

        return AnalysisResultsResponse(**response_data)

    def get_analysis_by_job_id(self, job_id: uuid.UUID) -> Optional[AnalysisResult]:
        """
        Get analysis record by job ID.
        
        Args:
            job_id: UUID of the analysis job
            
        Returns:
            AnalysisResult model instance or None if not found
        """
        return self.db.query(AnalysisResult).filter(AnalysisResult.job_id == job_id).first()

    def list_analysis_results(self, skip: int = 0, limit: int = 100) -> Tuple[List[Dict[str, Any]], int]:
        """
        List analysis results with transcript information and pagination.
        
        Args:
            skip: Number of records to skip
            limit: Maximum number of records to return
            
        Returns:
            Tuple of (enriched_analysis_list, total_count)
        """
        # Join analysis results with transcript data
        query = (self.db.query(AnalysisResult, Transcript)
                 .join(Transcript, AnalysisResult.transcript_id == Transcript.id)
                 .order_by(AnalysisResult.created_at.desc()))
        
        total = query.count()
        results = query.offset(skip).limit(limit).all()
        
        # Enrich analysis results with transcript information
        enriched_analyses = []
        for analysis, transcript in results:
            # Extract title from transcript metadata or use filename
            title = None
            if transcript.transcript_metadata and isinstance(transcript.transcript_metadata, dict):
                title = transcript.transcript_metadata.get('title')
            
            # Load fact checks for this analysis
            fact_checks = self.db.query(FactCheck).filter(FactCheck.analysis_id == analysis.id).all()
            fact_checks_data = [
                {
                    'id': fc.id,
                    'claim': fc.claim,
                    'verdict': fc.verdict,
                    'confidence': fc.confidence,
                    'evidence': fc.evidence,
                    'sources': fc.sources,
                    'checked_at': fc.checked_at
                }
                for fc in fact_checks
            ]
            
            # Create enriched analysis object
            analysis_dict = {
                'id': analysis.id,
                'job_id': analysis.job_id,
                'transcript_id': analysis.transcript_id,
                'status': analysis.status,
                'summary': analysis.summary,
                'takeaways': analysis.takeaways,
                'created_at': analysis.created_at,
                'completed_at': analysis.completed_at,
                'transcript_filename': transcript.filename,
                'transcript_title': title or transcript.filename,  # Fallback to filename
                'fact_checks': fact_checks_data
            }
            enriched_analyses.append(analysis_dict)
        
        logger.info("Listed analysis results", 
                   count=len(enriched_analyses), 
                   total=total, 
                   skip=skip, 
                   limit=limit)
        
        return enriched_analyses, total