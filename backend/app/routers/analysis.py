"""
API routes for analysis management.
Handles analysis job creation, status checking, and results retrieval.
"""

import uuid
from typing import List
from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.schemas.analysis import (
    AnalysisJobRequest, AnalysisJobResponse, JobStatusResponse,
    AnalysisResultsResponse, AnalysisResultsListResponse
)
from app.services.analysis_service import AnalysisService, AnalysisNotFoundError
from app.services.transcript_service import TranscriptService
from app.services.kafka_service import KafkaService, KafkaConnectionError
from app.utils.logger import get_logger, set_correlation_id

logger = get_logger(__name__)
router = APIRouter(prefix="/api", tags=["analysis"])


@router.post("/analyze/{transcript_id}", response_model=AnalysisJobResponse)
def start_analysis(
    transcript_id: uuid.UUID,
    db: Session = Depends(get_db)
) -> AnalysisJobResponse:
    """
    Start an analysis job for a transcript.
    
    Creates a new analysis job and queues it for processing.
    Returns job ID for status tracking.
    """
    correlation_id = set_correlation_id()
    
    logger.info("Start analysis request", transcript_id=transcript_id)

    try:
        # Verify transcript exists
        transcript_service = TranscriptService(db)
        transcript = transcript_service.get_transcript_by_id(transcript_id)
        
        if not transcript:
            logger.warning("Transcript not found for analysis", transcript_id=transcript_id)
            raise HTTPException(status_code=404, detail={
                "error": {
                    "code": "TRANSCRIPT_NOT_FOUND",
                    "message": "The requested transcript does not exist",
                    "correlation_id": correlation_id
                }
            })

        # Create analysis job
        analysis_service = AnalysisService(db)
        job_response = analysis_service.create_analysis_job(transcript_id)

        # Publish job to Kafka
        try:
            with KafkaService() as kafka:
                kafka.publish_analysis_job(job_response.job_id, transcript_id)
                
            logger.info("Analysis job queued", 
                       job_id=job_response.job_id,
                       transcript_id=transcript_id)

        except KafkaConnectionError as e:
            logger.error("Failed to queue analysis job", 
                        job_id=job_response.job_id,
                        error=str(e))
            # Update job status to failed
            analysis_service.update_job_status(
                job_response.job_id, 
                "failed", 
                f"Failed to queue job: {str(e)}"
            )
            raise HTTPException(status_code=503, detail={
                "error": {
                    "code": "QUEUE_ERROR",
                    "message": "Unable to queue analysis job for processing",
                    "correlation_id": correlation_id
                }
            })

        return job_response

    except HTTPException:
        raise
    except Exception as e:
        logger.error("Error starting analysis", 
                    transcript_id=transcript_id, 
                    error=str(e))
        raise HTTPException(status_code=500, detail={
            "error": {
                "code": "INTERNAL_ERROR",
                "message": "Failed to start analysis job",
                "correlation_id": correlation_id
            }
        })


@router.get("/jobs/{job_id}/status", response_model=JobStatusResponse)
def get_job_status(
    job_id: uuid.UUID,
    db: Session = Depends(get_db)
) -> JobStatusResponse:
    """
    Get the status of an analysis job.
    
    Returns current status and completion information.
    Poll this endpoint to track job progress.
    """
    correlation_id = set_correlation_id()
    
    logger.info("Job status request", job_id=job_id)

    try:
        service = AnalysisService(db)
        status = service.get_job_status(job_id)
        
        return status

    except AnalysisNotFoundError as e:
        logger.warning("Analysis job not found", job_id=job_id)
        raise HTTPException(status_code=404, detail={
            "error": {
                "code": "JOB_NOT_FOUND",
                "message": "The requested analysis job does not exist",
                "correlation_id": correlation_id
            }
        })

    except Exception as e:
        logger.error("Error getting job status", 
                    job_id=job_id, 
                    error=str(e))
        raise HTTPException(status_code=500, detail={
            "error": {
                "code": "INTERNAL_ERROR",
                "message": "Failed to retrieve job status",
                "correlation_id": correlation_id
            }
        })


@router.get("/results/{analysis_id}", response_model=AnalysisResultsResponse)
def get_analysis_results(
    analysis_id: uuid.UUID,
    db: Session = Depends(get_db)
) -> AnalysisResultsResponse:
    """
    Get complete analysis results.
    
    Returns summary, key takeaways, and fact-check table
    for a completed analysis job.
    """
    correlation_id = set_correlation_id()
    
    logger.info("Analysis results request", analysis_id=analysis_id)

    try:
        service = AnalysisService(db)
        results = service.get_analysis_results(analysis_id)
        
        if results.status != "completed":
            logger.warning("Analysis not completed", 
                          analysis_id=analysis_id, 
                          status=results.status)
            raise HTTPException(status_code=409, detail={
                "error": {
                    "code": "ANALYSIS_NOT_COMPLETED",
                    "message": f"Analysis is not completed (current status: {results.status})",
                    "correlation_id": correlation_id
                }
            })

        return results

    except AnalysisNotFoundError as e:
        logger.warning("Analysis not found", analysis_id=analysis_id)
        raise HTTPException(status_code=404, detail={
            "error": {
                "code": "ANALYSIS_NOT_FOUND",
                "message": "The requested analysis does not exist",
                "correlation_id": correlation_id
            }
        })

    except HTTPException:
        raise
    except Exception as e:
        logger.error("Error getting analysis results", 
                    analysis_id=analysis_id, 
                    error=str(e))
        raise HTTPException(status_code=500, detail={
            "error": {
                "code": "INTERNAL_ERROR",
                "message": "Failed to retrieve analysis results",
                "correlation_id": correlation_id
            }
        })


@router.get("/results", response_model=AnalysisResultsListResponse)
def list_analysis_results(
    page: int = Query(1, ge=1, description="Page number starting from 1"),
    per_page: int = Query(20, ge=1, le=100, description="Items per page (max 100)"),
    db: Session = Depends(get_db)
) -> AnalysisResultsListResponse:
    """
    List all analysis results with pagination.
    
    Returns paginated list of analysis results ordered by creation date (newest first).
    Includes summary information and status for each analysis.
    """
    correlation_id = set_correlation_id()
    
    logger.info("List analysis results request", page=page, per_page=per_page)

    try:
        service = AnalysisService(db)
        skip = (page - 1) * per_page
        analyses, total = service.list_analysis_results(skip=skip, limit=per_page)
        
        return AnalysisResultsListResponse(
            results=[AnalysisResultsResponse(**a) for a in analyses],
            total=total,
            page=page,
            per_page=per_page
        )

    except Exception as e:
        logger.error("Error listing analysis results", error=str(e))
        raise HTTPException(status_code=500, detail={
            "error": {
                "code": "INTERNAL_ERROR",
                "message": "Failed to retrieve analysis results",
                "correlation_id": correlation_id
            }
        })