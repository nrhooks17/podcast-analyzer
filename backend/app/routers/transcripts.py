"""
API routes for transcript management.
Handles file upload, listing, retrieval, and deletion of transcripts.
"""

import uuid
from typing import List
from fastapi import APIRouter, Depends, UploadFile, File, HTTPException, Query
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.schemas.transcript import TranscriptResponse, TranscriptListResponse, FileUploadResponse
from app.services.transcript_service import TranscriptService, TranscriptNotFoundError
from app.utils.file_handler import FileValidationError
from app.utils.logger import get_logger, set_correlation_id

logger = get_logger(__name__)
router = APIRouter(prefix="/api/transcripts", tags=["transcripts"])


@router.post("/", response_model=FileUploadResponse)
async def upload_transcript(
    file: UploadFile = File(...),
    db: Session = Depends(get_db)
) -> FileUploadResponse:
    """
    Upload a new transcript file.
    
    Accepts .txt or .json files containing podcast transcripts.
    Returns transcript ID and metadata for further processing.
    """
    # Set correlation ID for request tracing
    correlation_id = set_correlation_id()
    
    logger.info("Upload transcript request received", 
               filename=file.filename,
               content_type=file.content_type)

    try:
        service = TranscriptService(db)
        transcript = await service.upload_transcript(file)
        
        return FileUploadResponse(
            transcript_id=transcript.id,
            filename=transcript.filename,
            word_count=transcript.word_count,
            message="Transcript uploaded successfully"
        )

    except FileValidationError as e:
        logger.warning("File validation failed", 
                      filename=file.filename, 
                      error=str(e))
        raise HTTPException(status_code=400, detail={
            "error": {
                "code": "FILE_VALIDATION_ERROR",
                "message": str(e),
                "correlation_id": correlation_id
            }
        })

    except Exception as e:
        logger.error("Unexpected error during transcript upload", 
                    filename=file.filename,
                    error=str(e))
        raise HTTPException(status_code=500, detail={
            "error": {
                "code": "INTERNAL_ERROR",
                "message": "An unexpected error occurred during upload",
                "correlation_id": correlation_id
            }
        })


@router.get("/", response_model=TranscriptListResponse)
def list_transcripts(
    page: int = Query(1, ge=1, description="Page number starting from 1"),
    per_page: int = Query(20, ge=1, le=100, description="Items per page (max 100)"),
    db: Session = Depends(get_db)
) -> TranscriptListResponse:
    """
    List all transcripts with pagination.
    
    Returns paginated list of transcripts ordered by upload date (newest first).
    """
    correlation_id = set_correlation_id()
    
    logger.info("List transcripts request", page=page, per_page=per_page)

    try:
        service = TranscriptService(db)
        skip = (page - 1) * per_page
        transcripts, total = service.list_transcripts(skip=skip, limit=per_page)
        
        return TranscriptListResponse(
            transcripts=[TranscriptResponse.from_orm(t) for t in transcripts],
            total=total,
            page=page,
            per_page=per_page
        )

    except Exception as e:
        logger.error("Error listing transcripts", error=str(e))
        raise HTTPException(status_code=500, detail={
            "error": {
                "code": "INTERNAL_ERROR",
                "message": "Failed to retrieve transcripts",
                "correlation_id": correlation_id
            }
        })


@router.get("/{transcript_id}", response_model=TranscriptResponse)
def get_transcript(
    transcript_id: uuid.UUID,
    db: Session = Depends(get_db)
) -> TranscriptResponse:
    """
    Get transcript details by ID.
    
    Returns metadata about a specific transcript including filename,
    upload date, word count, and any associated metadata.
    """
    correlation_id = set_correlation_id()
    
    logger.info("Get transcript request", transcript_id=transcript_id)

    try:
        service = TranscriptService(db)
        transcript = service.get_transcript_by_id(transcript_id)
        
        if not transcript:
            logger.warning("Transcript not found", transcript_id=transcript_id)
            raise HTTPException(status_code=404, detail={
                "error": {
                    "code": "TRANSCRIPT_NOT_FOUND",
                    "message": "The requested transcript does not exist",
                    "correlation_id": correlation_id
                }
            })

        return TranscriptResponse.from_orm(transcript)

    except HTTPException:
        raise
    except Exception as e:
        logger.error("Error retrieving transcript", 
                    transcript_id=transcript_id, 
                    error=str(e))
        raise HTTPException(status_code=500, detail={
            "error": {
                "code": "INTERNAL_ERROR",
                "message": "Failed to retrieve transcript",
                "correlation_id": correlation_id
            }
        })


@router.delete("/{transcript_id}")
def delete_transcript(
    transcript_id: uuid.UUID,
    db: Session = Depends(get_db)
) -> dict[str, str]:
    """
    Delete a transcript and its associated file.
    
    Permanently removes the transcript record and file from storage.
    This action cannot be undone.
    """
    correlation_id = set_correlation_id()
    
    logger.info("Delete transcript request", transcript_id=transcript_id)

    try:
        service = TranscriptService(db)
        deleted = service.delete_transcript(transcript_id)
        
        if not deleted:
            logger.warning("Transcript not found for deletion", transcript_id=transcript_id)
            raise HTTPException(status_code=404, detail={
                "error": {
                    "code": "TRANSCRIPT_NOT_FOUND",
                    "message": "The requested transcript does not exist",
                    "correlation_id": correlation_id
                }
            })

        return {"message": "Transcript deleted successfully"}

    except HTTPException:
        raise
    except Exception as e:
        logger.error("Error deleting transcript", 
                    transcript_id=transcript_id, 
                    error=str(e))
        raise HTTPException(status_code=500, detail={
            "error": {
                "code": "INTERNAL_ERROR",
                "message": "Failed to delete transcript",
                "correlation_id": correlation_id
            }
        })