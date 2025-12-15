"""
Tests for transcript service business logic.
"""

import pytest
import tempfile
import os
import uuid
from typing import Generator, Tuple, List, Any
from unittest.mock import Mock, patch, MagicMock
from fastapi import UploadFile
from io import BytesIO

from app.services.transcript_service import TranscriptService, TranscriptNotFoundError
from app.models.transcript import Transcript


class TestTranscriptService:
    """Test the TranscriptService class."""

    @pytest.fixture
    def mock_db(self) -> Mock:
        """Create mock database session."""
        return Mock()

    @pytest.fixture
    def service(self, mock_db: Mock) -> TranscriptService:
        """Create TranscriptService instance with mocked DB."""
        return TranscriptService(mock_db)

    @pytest.fixture
    def upload_file_txt(self) -> UploadFile:
        """Create mock UploadFile for TXT."""
        content = b"[00:00:00] Host: Welcome to the show.\n[00:00:15] Guest: Thanks."
        file_obj = BytesIO(content)
        file_obj.seek(0)
        
        upload_file = UploadFile(
            filename="test.txt",
            file=file_obj,
            size=len(content)
        )
        return upload_file

    @pytest.mark.asyncio
    async def test_upload_transcript_success(self, service: TranscriptService, upload_file_txt: UploadFile, mock_db: Mock) -> None:
        """Test successful transcript upload."""
        # Mock database operations
        mock_transcript = Mock()
        mock_transcript.id = uuid.uuid4()
        mock_transcript.filename = "test.txt"
        mock_transcript.file_path = "/temp/test.txt"
        mock_transcript.content_hash = "abcd1234" * 8
        mock_transcript.word_count = 5
        from datetime import datetime
        mock_transcript.uploaded_at = datetime.utcnow()
        mock_transcript.transcript_metadata = None
        
        mock_db.query().filter().first.return_value = None  # No duplicate
        mock_db.add = Mock()
        mock_db.commit = Mock()
        mock_db.refresh = Mock()
        
        with patch('app.services.transcript_service.settings.storage_path', tempfile.mkdtemp()), \
             patch('app.services.transcript_service.Transcript', return_value=mock_transcript):
            result = await service.upload_transcript(upload_file_txt)
            
            assert result.filename == "test.txt"
            assert result.word_count == 5
            assert result.id is not None

    @pytest.mark.asyncio
    async def test_upload_transcript_duplicate_overwrites(self, service: TranscriptService, upload_file_txt: UploadFile, mock_db: Mock) -> None:
        """Test duplicate transcript upload overwrites existing record."""
        # Mock existing transcript with all required attributes
        existing_transcript = Mock()
        existing_transcript.id = uuid.uuid4()
        existing_transcript.filename = "test.txt"
        existing_transcript.file_path = "/temp/test.txt"
        existing_transcript.content_hash = "abcd1234" * 8
        existing_transcript.word_count = 5
        from datetime import datetime
        existing_transcript.uploaded_at = datetime.utcnow()
        existing_transcript.transcript_metadata = None
        
        mock_db.query().filter().first.return_value = existing_transcript
        mock_db.commit = Mock()
        mock_db.refresh = Mock()
        
        with patch('app.services.transcript_service.settings.storage_path', tempfile.mkdtemp()):
            # Reset file position
            upload_file_txt.file.seek(0)
            
            result = await service.upload_transcript(upload_file_txt)
            
            # Should return the existing transcript (updated)
            assert result.id == existing_transcript.id
            assert result.filename == "test.txt"
            assert result.word_count == 5

    @pytest.mark.asyncio
    async def test_upload_transcript_invalid_extension(self, service: TranscriptService, mock_db: Mock) -> None:
        """Test upload with invalid file extension."""
        content = b"Test content"
        file_obj = BytesIO(content)
        
        upload_file = UploadFile(
            filename="test.pdf",
            file=file_obj
        )
        
        from app.utils.file_handler import FileValidationError
        with pytest.raises(FileValidationError, match="File extension"):
            await service.upload_transcript(upload_file)

    @pytest.mark.asyncio
    async def test_upload_transcript_too_large(self, service: TranscriptService, mock_db: Mock) -> None:
        """Test upload with file too large."""
        # Create large content
        content = b"x" * (11 * 1024 * 1024)  # 11MB
        file_obj = BytesIO(content)
        
        upload_file = UploadFile(
            filename="test.txt",
            file=file_obj,
            size=len(content)
        )
        
        from app.utils.file_handler import FileValidationError
        with pytest.raises(FileValidationError, match="File size"):
            await service.upload_transcript(upload_file)

    def test_get_transcript_by_id_exists(self, service: TranscriptService, mock_db: Mock) -> None:
        """Test retrieving existing transcript by ID."""
        # Mock existing transcript
        mock_transcript = Mock()
        mock_transcript.id = uuid.uuid4()
        mock_transcript.filename = "test.txt"
        
        mock_db.query().filter().first.return_value = mock_transcript
        
        # Retrieve transcript
        result = service.get_transcript_by_id(mock_transcript.id)
        
        assert result is not None
        assert result.id == mock_transcript.id
        assert result.filename == "test.txt"

    def test_get_transcript_by_id_not_exists(self, service: TranscriptService, mock_db: Mock) -> None:
        """Test retrieving non-existent transcript by ID."""
        mock_db.query().filter().first.return_value = None
        
        fake_id = uuid.uuid4()
        result = service.get_transcript_by_id(fake_id)
        
        assert result is None

    def test_get_transcript_by_hash(self, service: TranscriptService, mock_db: Mock) -> None:
        """Test retrieving transcript by content hash."""
        # Mock existing transcript
        content_hash = "abcd1234" * 8
        mock_transcript = Mock()
        mock_transcript.content_hash = content_hash
        
        mock_db.query().filter().first.return_value = mock_transcript
        
        # Retrieve transcript by hash
        result = service.get_transcript_by_hash(content_hash)
        
        assert result is not None
        assert result.content_hash == content_hash

    def test_list_transcripts(self, service: TranscriptService, mock_db: Mock) -> None:
        """Test listing transcripts with pagination."""
        # Mock query results - need to set up the chain properly
        mock_transcripts = [Mock() for _ in range(3)]
        mock_query = Mock()
        mock_query.order_by().offset().limit().all.return_value = mock_transcripts
        mock_query.order_by().count.return_value = 5
        mock_db.query.return_value = mock_query
        
        # List first 3 transcripts
        transcripts, total = service.list_transcripts(skip=0, limit=3)
        
        assert len(transcripts) == 3
        assert total == 5

    def test_delete_transcript_exists(self, service: TranscriptService, mock_db: Mock) -> None:
        """Test deleting existing transcript."""
        # Mock existing transcript
        transcript_id = uuid.uuid4()
        mock_transcript = Mock()
        mock_transcript.id = transcript_id
        mock_transcript.file_path = "/tmp/test.txt"
        
        mock_db.query().filter().first.return_value = mock_transcript
        mock_db.delete = Mock()
        mock_db.commit = Mock()
        
        # Create dummy file
        with open("/tmp/test.txt", "w") as f:
            f.write("test content")
        
        try:
            # Delete transcript
            result = service.delete_transcript(transcript_id)
            
            assert result is True
            mock_db.delete.assert_called_once_with(mock_transcript)
            mock_db.commit.assert_called_once()
            
            # Verify file is deleted
            assert not os.path.exists("/tmp/test.txt")
            
        finally:
            # Cleanup
            if os.path.exists("/tmp/test.txt"):
                os.remove("/tmp/test.txt")

    def test_delete_transcript_not_exists(self, service: TranscriptService, mock_db: Mock) -> None:
        """Test deleting non-existent transcript."""
        mock_db.query().filter().first.return_value = None
        
        fake_id = uuid.uuid4()
        result = service.delete_transcript(fake_id)
        
        assert result is False

    @pytest.mark.asyncio
    async def test_read_transcript_content(self, service: TranscriptService) -> None:
        """Test reading transcript file content."""
        # Create test file
        test_content = "Test transcript content"
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
            f.write(test_content)
            temp_path = f.name
        
        try:
            # Create transcript object
            transcript = Mock()
            transcript.file_path = temp_path
            transcript.id = "test-id"
            
            # Read content
            content = await service.read_transcript_content(transcript)
            
            assert content == test_content
            
        finally:
            # Cleanup
            os.remove(temp_path)

    @pytest.mark.asyncio
    async def test_read_transcript_content_missing_file(self, service: TranscriptService) -> None:
        """Test reading transcript with missing file."""
        # Create transcript object with non-existent path
        transcript = Mock()
        transcript.file_path = "/non/existent/path.txt"
        transcript.id = "test-id"
        
        # Should raise TranscriptNotFoundError
        with pytest.raises(TranscriptNotFoundError):
            await service.read_transcript_content(transcript)