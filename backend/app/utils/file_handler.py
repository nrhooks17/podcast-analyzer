"""
File parsing utilities for podcast transcript files.
Handles both .txt and .json formats with validation and word counting.
"""

import hashlib
import json
import re
from pathlib import Path
from typing import Dict, Any, Tuple

from app.utils.logger import get_logger

logger = get_logger(__name__)


class FileValidationError(Exception):
    """Raised when file validation fails."""
    pass


class TranscriptParser:
    """Parser for podcast transcript files in .txt and .json formats."""

    @staticmethod
    def validate_file_extension(filename: str, allowed_extensions: list[str]) -> None:
        """
        Validate that file has an allowed extension.
        
        Args:
            filename: Name of the uploaded file
            allowed_extensions: List of allowed file extensions (e.g., ['.txt', '.json'])
            
        Raises:
            FileValidationError: If extension is not allowed
        """
        file_ext = Path(filename).suffix.lower()
        if file_ext not in allowed_extensions:
            logger.warning("Invalid file extension", filename=filename, extension=file_ext)
            raise FileValidationError(
                f"File extension '{file_ext}' not allowed. Supported formats: {', '.join(allowed_extensions)}"
            )

    @staticmethod
    def validate_file_size(file_size: int, max_size: int) -> None:
        """
        Validate that file size is within limits.
        
        Args:
            file_size: Size of the file in bytes
            max_size: Maximum allowed file size in bytes
            
        Raises:
            FileValidationError: If file is too large
        """
        if file_size > max_size:
            max_mb = max_size / (1024 * 1024)
            current_mb = file_size / (1024 * 1024)
            logger.warning("File too large", file_size_mb=current_mb, max_size_mb=max_mb)
            raise FileValidationError(
                f"File size ({current_mb:.2f} MB) exceeds maximum allowed size ({max_mb:.2f} MB)"
            )

    @staticmethod
    def calculate_content_hash(content: str) -> str:
        """
        Calculate SHA-256 hash of file content for deduplication.
        
        Args:
            content: Text content of the file
            
        Returns:
            Hexadecimal SHA-256 hash string
        """
        return hashlib.sha256(content.encode('utf-8')).hexdigest()

    @staticmethod
    def count_words(text: str) -> int:
        """
        Count words in text content.
        
        Args:
            text: Input text to count words in
            
        Returns:
            Number of words in the text
        """
        # Remove timestamps and speaker labels, then count words
        # Pattern to remove timestamps like [00:00:00]
        text_without_timestamps = re.sub(r'\[\d{2}:\d{2}:\d{2}\]', '', text)
        
        # Pattern to remove speaker labels like "Speaker:" at start of lines
        text_without_speakers = re.sub(r'^[A-Za-z\s]+:', '', text_without_timestamps, flags=re.MULTILINE)
        
        # Count actual words (sequences of alphabetic characters)
        words = re.findall(r'\b[A-Za-z]+\b', text_without_speakers)
        return len(words)

    @staticmethod
    def parse_txt_file(content: str) -> Tuple[str, Dict[str, Any], int]:
        """
        Parse a .txt transcript file.
        
        Args:
            content: Raw text content of the file
            
        Returns:
            Tuple of (cleaned_content, metadata, word_count)
            
        Raises:
            FileValidationError: If file format is invalid
        """
        if not content.strip():
            raise FileValidationError("Text file is empty")

        # Basic validation - check for timestamp pattern
        timestamp_pattern = r'\[\d{2}:\d{2}:\d{2}\]'
        if not re.search(timestamp_pattern, content):
            logger.warning("Text file missing timestamp format")
            # Allow files without timestamps but log a warning
        
        word_count = TranscriptParser.count_words(content)
        if word_count == 0:
            raise FileValidationError("No words found in transcript")

        metadata = {"format": "txt", "has_timestamps": bool(re.search(timestamp_pattern, content))}
        
        logger.info("Parsed TXT file", word_count=word_count, has_timestamps=metadata["has_timestamps"])
        return content, metadata, word_count

    @staticmethod
    def parse_json_file(content: str) -> Tuple[str, Dict[str, Any], int]:
        """
        Parse a .json transcript file.
        
        Args:
            content: Raw JSON content of the file
            
        Returns:
            Tuple of (transcript_text, metadata, word_count)
            
        Raises:
            FileValidationError: If JSON format is invalid
        """
        try:
            data = json.loads(content)
        except json.JSONDecodeError as e:
            logger.error("Invalid JSON format", error=str(e))
            raise FileValidationError(f"Invalid JSON format: {str(e)}")

        if not isinstance(data, dict):
            raise FileValidationError("JSON must contain an object at the root level")

        # Extract transcript text from JSON structure
        transcript_text = ""
        
        if "transcript" in data and isinstance(data["transcript"], list):
            # Format: {"transcript": [{"speaker": "...", "text": "...", "timestamp": "..."}]}
            for entry in data["transcript"]:
                if not isinstance(entry, dict):
                    continue
                
                speaker = entry.get("speaker", "Unknown")
                text = entry.get("text", "")
                timestamp = entry.get("timestamp", "")
                
                if timestamp:
                    transcript_text += f"[{timestamp}] {speaker}: {text}\n"
                else:
                    transcript_text += f"{speaker}: {text}\n"
        
        elif "content" in data:
            # Simple format: {"content": "transcript text here..."}
            transcript_text = str(data["content"])
        
        else:
            raise FileValidationError(
                "JSON must contain either 'transcript' array or 'content' field"
            )

        if not transcript_text.strip():
            raise FileValidationError("No transcript content found in JSON")

        word_count = TranscriptParser.count_words(transcript_text)
        if word_count == 0:
            raise FileValidationError("No words found in transcript")

        # Extract metadata
        metadata = {
            "format": "json",
            "title": data.get("title"),
            "date": data.get("date"),
            "speakers": data.get("speakers", []),
            "original_structure": "transcript_array" if "transcript" in data else "content_field"
        }

        logger.info("Parsed JSON file", 
                   word_count=word_count, 
                   title=metadata.get("title"), 
                   speakers_count=len(metadata.get("speakers", [])))
        
        return transcript_text, metadata, word_count

    @staticmethod
    def parse_transcript_file(filename: str, content: str) -> Tuple[str, Dict[str, Any], int]:
        """
        Parse transcript file based on extension.
        
        Args:
            filename: Name of the uploaded file
            content: Raw content of the file
            
        Returns:
            Tuple of (transcript_text, metadata, word_count)
            
        Raises:
            FileValidationError: If file format is invalid or unsupported
        """
        file_ext = Path(filename).suffix.lower()
        
        logger.info("Parsing transcript file", filename=filename, extension=file_ext)
        
        if file_ext == '.txt':
            return TranscriptParser.parse_txt_file(content)
        elif file_ext == '.json':
            return TranscriptParser.parse_json_file(content)
        else:
            raise FileValidationError(f"Unsupported file extension: {file_ext}")