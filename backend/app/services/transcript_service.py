"""
Service layer for transcript-related business logic.
Handles file upload, validation, storage, and database operations.
"""

import os
import uuid
import aiofiles
from pathlib import Path
from datetime import datetime
from typing import List, Optional, Tuple, Dict, Any
from fastapi import UploadFile
from sqlalchemy.orm import Session
from sqlalchemy.exc import IntegrityError

from app.config import get_settings
from app.models.transcript import Transcript
from app.schemas.transcript import TranscriptCreate, TranscriptResponse
from app.utils.file_handler import TranscriptParser, FileValidationError
from app.utils.logger import get_logger, set_correlation_id, get_correlation_id

logger = get_logger(__name__)
settings = get_settings()


class TranscriptNotFoundError(Exception):
    """Raised when a transcript is not found in the database."""
    pass



class TranscriptService:
    """
    Service class for handling transcript operations.
    
    Provides methods for file upload, validation, storage, and retrieval
    with comprehensive logging and error handling.
    """

    def __init__(self, db: Session) -> None:
        """
        Initialize the transcript service.
        
        Args:
            db: Database session for operations
        """
        self.db: Session = db
        self.parser: TranscriptParser = TranscriptParser()

    async def upload_transcript(self, file: UploadFile) -> TranscriptResponse:
        """
        Upload and process a transcript file.
        
        Args:
            file: Uploaded file object from FastAPI
            
        Returns:
            TranscriptResponse with created transcript details
            
        Raises:
            FileValidationError: If file validation fails
            DuplicateTranscriptError: If transcript already exists
        """
        correlation_id = get_correlation_id()
        
        logger.info("File upload received", 
                   filename=file.filename, 
                   content_type=file.content_type,
                   size_bytes=file.size if hasattr(file, 'size') else 'unknown')

        # Validate file extension
        self.parser.validate_file_extension(file.filename, settings.allowed_extensions)
        
        # Read file content
        content = await file.read()
        
        # Validate file size
        self.parser.validate_file_size(len(content), settings.max_file_size)
        
        # Decode content
        try:
            content_str = content.decode('utf-8')
        except UnicodeDecodeError:
            logger.error("File encoding error", filename=file.filename)
            raise FileValidationError("File must be valid UTF-8 encoded text")

        logger.info("File validation passed", filename=file.filename)

        # Parse transcript content
        try:
            transcript_text, metadata, word_count = self.parser.parse_transcript_file(
                file.filename, content_str
            )
            
            # Calculate content hash for deduplication
            content_hash = self.parser.calculate_content_hash(transcript_text)
            
            logger.info("File parsed successfully", 
                       filename=file.filename, 
                       word_count=word_count,
                       content_hash=content_hash[:16])  # Log first 16 chars of hash

        except FileValidationError:
            raise
        except Exception as e:
            logger.error("File parsing failed", filename=file.filename, error=str(e))
            raise FileValidationError(f"Failed to parse transcript file: {str(e)}")

        # Check for duplicate content and handle overwrite
        existing_transcript = self.get_transcript_by_hash(content_hash)
        if existing_transcript:
            logger.info("Duplicate transcript detected, updating existing record", 
                       filename=file.filename,
                       existing_id=existing_transcript.id,
                       existing_filename=existing_transcript.filename)
            
            # Update existing transcript with new file information
            try:
                # Update transcript metadata and filename
                existing_transcript.filename = file.filename
                existing_transcript.word_count = word_count
                existing_transcript.transcript_metadata = metadata
                existing_transcript.uploaded_at = datetime.utcnow()
                
                self.db.commit()
                self.db.refresh(existing_transcript)
                
                logger.info("Updated existing transcript record", 
                           transcript_id=existing_transcript.id,
                           filename=existing_transcript.filename)
                
                return TranscriptResponse.from_orm(existing_transcript)
                
            except Exception as e:
                self.db.rollback()
                logger.error("Failed to update existing transcript", 
                            transcript_id=existing_transcript.id, 
                            error=str(e))
                raise

        # Generate unique filename and save file
        file_id = str(uuid.uuid4())
        file_extension = Path(file.filename).suffix
        unique_filename = f"{file_id}{file_extension}"
        file_path = os.path.join(settings.storage_path, unique_filename)

        try:
            # Ensure storage directory exists
            os.makedirs(settings.storage_path, exist_ok=True)
            
            # Save file to storage
            async with aiofiles.open(file_path, 'w', encoding='utf-8') as f:
                await f.write(transcript_text)
            
            logger.info("File saved to storage", 
                       filename=file.filename,
                       storage_path=file_path)

        except Exception as e:
            logger.error("File storage failed", filename=file.filename, error=str(e))
            raise FileValidationError(f"Failed to save file: {str(e)}")

        # Create database record
        try:
            transcript_data = TranscriptCreate(
                filename=file.filename,
                content_hash=content_hash,
                word_count=word_count,
                transcript_metadata=metadata
            )
            
            transcript = self.create_transcript(transcript_data, file_path)
            
            logger.info("Transcript record created", 
                       transcript_id=transcript.id,
                       filename=transcript.filename)

            return TranscriptResponse.from_orm(transcript)

        except Exception as e:
            # Clean up saved file if database operation fails
            try:
                os.remove(file_path)
                logger.info("Cleaned up file after database error", file_path=file_path)
            except:
                pass
            
            logger.error("Database operation failed", filename=file.filename, error=str(e))
            raise

    def create_transcript(self, transcript_data: TranscriptCreate, file_path: str) -> Transcript:
        """
        Create a new transcript record in the database.
        
        Args:
            transcript_data: Validated transcript data
            file_path: Path to the stored transcript file
            
        Returns:
            Created Transcript model instance
        """
        transcript = Transcript(
            filename=transcript_data.filename,
            file_path=file_path,
            content_hash=transcript_data.content_hash,
            word_count=transcript_data.word_count,
            transcript_metadata=transcript_data.transcript_metadata
        )
        
        try:
            self.db.add(transcript)
            self.db.commit()
            self.db.refresh(transcript)
            
            logger.info("Created transcript record",
                       transcript_id=transcript.id,
                       filename=transcript.filename,
                       word_count=transcript.word_count)
            
            return transcript
            
        except IntegrityError as e:
            self.db.rollback()
            logger.error("Database integrity error", error=str(e))
            raise

    def get_transcript_by_id(self, transcript_id: uuid.UUID) -> Optional[Transcript]:
        """
        Retrieve a transcript by its ID.
        
        Args:
            transcript_id: UUID of the transcript to retrieve
            
        Returns:
            Transcript model instance or None if not found
        """
        return self.db.query(Transcript).filter(Transcript.id == transcript_id).first()

    def get_transcript_by_hash(self, content_hash: str) -> Optional[Transcript]:
        """
        Retrieve a transcript by its content hash.
        
        Args:
            content_hash: SHA-256 hash of the transcript content
            
        Returns:
            Transcript model instance or None if not found
        """
        return self.db.query(Transcript).filter(Transcript.content_hash == content_hash).first()

    def list_transcripts(self, skip: int = 0, limit: int = 100) -> Tuple[List[Transcript], int]:
        """
        List transcripts with pagination.
        
        Args:
            skip: Number of records to skip
            limit: Maximum number of records to return
            
        Returns:
            Tuple of (transcript_list, total_count)
        """
        query = self.db.query(Transcript).order_by(Transcript.uploaded_at.desc())
        total = query.count()
        transcripts = query.offset(skip).limit(limit).all()
        
        logger.info("Listed transcripts", count=len(transcripts), total=total, skip=skip, limit=limit)
        
        return transcripts, total

    def delete_transcript(self, transcript_id: uuid.UUID) -> bool:
        """
        Delete a transcript and its associated file.
        
        Args:
            transcript_id: UUID of the transcript to delete
            
        Returns:
            True if deleted successfully, False if not found
        """
        transcript = self.get_transcript_by_id(transcript_id)
        if not transcript:
            logger.warning("Transcript not found for deletion", transcript_id=transcript_id)
            return False

        try:
            # Delete the file from storage
            if os.path.exists(transcript.file_path):
                os.remove(transcript.file_path)
                logger.info("Deleted transcript file", file_path=transcript.file_path)

            # Delete the database record
            self.db.delete(transcript)
            self.db.commit()
            
            logger.info("Deleted transcript record", 
                       transcript_id=transcript_id, 
                       filename=transcript.filename)
            
            return True

        except Exception as e:
            self.db.rollback()
            logger.error("Failed to delete transcript", 
                        transcript_id=transcript_id, 
                        error=str(e))
            raise

    async def read_transcript_content(self, transcript: Transcript) -> str:
        """
        Read the content of a transcript file.
        
        Args:
            transcript: Transcript model instance
            
        Returns:
            Content of the transcript file
            
        Raises:
            TranscriptNotFoundError: If transcript file is missing
        """
        if not os.path.exists(transcript.file_path):
            logger.error("Transcript file not found", 
                        transcript_id=transcript.id, 
                        file_path=transcript.file_path)
            raise TranscriptNotFoundError(f"Transcript file not found: {transcript.file_path}")

        try:
            async with aiofiles.open(transcript.file_path, 'r', encoding='utf-8') as f:
                content = await f.read()
            
            logger.info("Read transcript content", 
                       transcript_id=transcript.id, 
                       content_length=len(content))
            
            return content
            
        except Exception as e:
            logger.error("Failed to read transcript file", 
                        transcript_id=transcript.id, 
                        error=str(e))
            raise TranscriptNotFoundError(f"Failed to read transcript file: {str(e)}")