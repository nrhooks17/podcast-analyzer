"""
Tests for file parsing utilities.
"""

import pytest
import json
from typing import Tuple, Dict, Any
from app.utils.file_handler import TranscriptParser, FileValidationError


class TestTranscriptParser:
    """Test the TranscriptParser class."""

    def test_validate_file_extension_valid(self) -> None:
        """Test valid file extensions."""
        parser = TranscriptParser()
        
        # Should not raise for valid extensions
        parser.validate_file_extension("test.txt", [".txt", ".json"])
        parser.validate_file_extension("test.json", [".txt", ".json"])
        parser.validate_file_extension("TEST.TXT", [".txt", ".json"])  # Case insensitive

    def test_validate_file_extension_invalid(self) -> None:
        """Test invalid file extensions."""
        parser = TranscriptParser()
        
        with pytest.raises(FileValidationError, match="File extension '.pdf' not allowed"):
            parser.validate_file_extension("test.pdf", [".txt", ".json"])

    def test_validate_file_size_valid(self) -> None:
        """Test valid file sizes."""
        parser = TranscriptParser()
        
        # Should not raise for valid sizes
        parser.validate_file_size(1000, 10000)
        parser.validate_file_size(10000, 10000)

    def test_validate_file_size_invalid(self) -> None:
        """Test invalid file sizes."""
        parser = TranscriptParser()
        
        with pytest.raises(FileValidationError, match="File size"):
            parser.validate_file_size(15000, 10000)

    def test_calculate_content_hash(self) -> None:
        """Test content hash calculation."""
        parser = TranscriptParser()
        
        content1 = "Hello world"
        content2 = "Hello world"
        content3 = "Different content"
        
        hash1 = parser.calculate_content_hash(content1)
        hash2 = parser.calculate_content_hash(content2)
        hash3 = parser.calculate_content_hash(content3)
        
        # Same content should produce same hash
        assert hash1 == hash2
        # Different content should produce different hash
        assert hash1 != hash3
        # Hash should be 64 characters (SHA-256)
        assert len(hash1) == 64

    def test_count_words(self) -> None:
        """Test word counting functionality."""
        parser = TranscriptParser()
        
        # Basic text
        assert parser.count_words("Hello world") == 2
        
        # Text with timestamps and speakers
        text_with_timestamps = "[00:00:00] Host: Hello world. This is a test."
        assert parser.count_words(text_with_timestamps) == 6  # Excludes timestamps and speaker labels
        
        # Empty text
        assert parser.count_words("") == 0
        assert parser.count_words("   ") == 0

    def test_parse_txt_file_valid(self) -> None:
        """Test parsing valid TXT files."""
        parser = TranscriptParser()
        
        content = """[00:00:00] Host: Welcome to the show.
[00:00:15] Guest: Thanks for having me."""
        
        transcript_text, metadata, word_count = parser.parse_txt_file(content)
        
        assert transcript_text == content
        assert metadata["format"] == "txt"
        assert metadata["has_timestamps"] == True
        # The actual word counting logic gives us 8 words (as shown by the log output)
        assert word_count == 8

    def test_parse_txt_file_no_timestamps(self) -> None:
        """Test parsing TXT files without timestamps."""
        parser = TranscriptParser()
        
        content = "Host: Welcome to the show. Guest: Thanks for having me."
        
        transcript_text, metadata, word_count = parser.parse_txt_file(content)
        
        assert transcript_text == content
        assert metadata["format"] == "txt"
        assert metadata["has_timestamps"] == False
        assert word_count == 9

    def test_parse_txt_file_empty(self) -> None:
        """Test parsing empty TXT files."""
        parser = TranscriptParser()
        
        with pytest.raises(FileValidationError, match="Text file is empty"):
            parser.parse_txt_file("")

    def test_parse_json_file_transcript_array(self) -> None:
        """Test parsing JSON files with transcript array format."""
        parser = TranscriptParser()
        
        data = {
            "title": "Test Podcast",
            "speakers": ["Host", "Guest"],
            "transcript": [
                {"timestamp": "00:00:00", "speaker": "Host", "text": "Welcome"},
                {"timestamp": "00:00:15", "speaker": "Guest", "text": "Thanks"}
            ]
        }
        content = json.dumps(data)
        
        transcript_text, metadata, word_count = parser.parse_json_file(content)
        
        assert "[00:00:00] Host: Welcome" in transcript_text
        assert "[00:00:15] Guest: Thanks" in transcript_text
        assert metadata["format"] == "json"
        assert metadata["title"] == "Test Podcast"
        assert metadata["speakers"] == ["Host", "Guest"]
        assert word_count == 2

    def test_parse_json_file_content_field(self) -> None:
        """Test parsing JSON files with content field format."""
        parser = TranscriptParser()
        
        data = {
            "content": "Host: Welcome to the show. Guest: Thanks for having me."
        }
        content = json.dumps(data)
        
        transcript_text, metadata, word_count = parser.parse_json_file(content)
        
        assert transcript_text == data["content"]
        assert metadata["format"] == "json"
        assert metadata["original_structure"] == "content_field"
        assert word_count == 9

    def test_parse_json_file_invalid_json(self) -> None:
        """Test parsing invalid JSON."""
        parser = TranscriptParser()
        
        with pytest.raises(FileValidationError, match="Invalid JSON format"):
            parser.parse_json_file("{invalid json")

    def test_parse_json_file_missing_content(self) -> None:
        """Test parsing JSON without transcript content."""
        parser = TranscriptParser()
        
        data = {"title": "Test"}
        content = json.dumps(data)
        
        with pytest.raises(FileValidationError, match="JSON must contain either 'transcript' array or 'content' field"):
            parser.parse_json_file(content)

    def test_parse_transcript_file_txt(self) -> None:
        """Test parsing transcript file with TXT extension."""
        parser = TranscriptParser()
        
        content = "[00:00:00] Host: Hello world"
        
        transcript_text, metadata, word_count = parser.parse_transcript_file("test.txt", content)
        
        assert transcript_text == content
        assert metadata["format"] == "txt"
        assert word_count == 2

    def test_parse_transcript_file_json(self) -> None:
        """Test parsing transcript file with JSON extension."""
        parser = TranscriptParser()
        
        data = {"content": "Hello world"}
        content = json.dumps(data)
        
        transcript_text, metadata, word_count = parser.parse_transcript_file("test.json", content)
        
        assert transcript_text == data["content"]
        assert metadata["format"] == "json"
        assert word_count == 2

    def test_parse_transcript_file_unsupported(self) -> None:
        """Test parsing unsupported file extension."""
        parser = TranscriptParser()
        
        with pytest.raises(FileValidationError, match="Unsupported file extension"):
            parser.parse_transcript_file("test.pdf", "content")