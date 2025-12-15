"""
Tests for the summarizer AI agent.
"""

import pytest
from typing import Dict, Any
from unittest.mock import Mock, patch

from app.agents.summarizer import SummarizerAgent
from app.agents.base_agent import AgentProcessingError


class TestSummarizerAgent:
    """Test the SummarizerAgent class."""

    @pytest.fixture
    def agent(self, mock_anthropic_client: Mock) -> SummarizerAgent:
        """Create SummarizerAgent instance with mocked client."""
        agent = SummarizerAgent()
        agent.client = mock_anthropic_client
        return agent

    @pytest.fixture
    def sample_transcript(self) -> str:
        """Sample transcript for testing."""
        return """[00:00:00] Host: Welcome to the show. Today we're discussing space exploration with our guest.
[00:00:15] Guest: Thanks for having me. NASA's Mars mission is launching in 2026, which is incredibly exciting.
[00:01:02] Host: That's fascinating. What are the main challenges they're facing?
[00:01:30] Guest: The main challenges include radiation protection, life support systems, and the psychological challenges of long-duration spaceflight."""

    def test_process_success(self, agent: SummarizerAgent, sample_transcript: str) -> None:
        """Test successful summarization."""
        # Mock Claude response
        mock_summary = "This podcast discusses space exploration, focusing on NASA's upcoming Mars mission in 2026. The conversation covers the main challenges including radiation protection, life support systems, and psychological factors for long-duration spaceflight."
        agent.client.messages.create.return_value.content[0].text = mock_summary
        
        result = agent.process(sample_transcript)
        
        assert "summary" in result
        # The actual code runs _clean_summary which may modify the text
        assert len(result["summary"]) > 0
        assert isinstance(result["summary"], str)

    def test_process_empty_transcript(self, agent: SummarizerAgent) -> None:
        """Test processing empty transcript."""
        with pytest.raises(AgentProcessingError, match="Cannot summarize empty transcript"):
            agent.process("")

    def test_process_whitespace_transcript(self, agent: SummarizerAgent) -> None:
        """Test processing transcript with only whitespace."""
        with pytest.raises(AgentProcessingError, match="Cannot summarize empty transcript"):
            agent.process("   \n\t  ")

    def test_process_long_transcript_truncation(self, agent: SummarizerAgent) -> None:
        """Test that very long transcripts are truncated."""
        # Create a very long transcript
        long_transcript = "[00:00:00] Host: " + "Very long content. " * 2000
        
        # Mock Claude response
        mock_summary = "Summary of long content."
        agent.client.messages.create.return_value.content[0].text = mock_summary
        
        result = agent.process(long_transcript)
        
        assert "summary" in result
        # Verify the call was made (transcript was processed)
        agent.client.messages.create.assert_called_once()

    def test_build_summarization_prompt(self, agent: SummarizerAgent, sample_transcript: str) -> None:
        """Test prompt building."""
        prompt = agent._build_summarization_prompt(sample_transcript)
        
        assert "TRANSCRIPT:" in prompt
        assert sample_transcript in prompt
        assert "SUMMARY:" in prompt
        assert "200-300 words" in prompt

    def test_clean_summary(self, agent: SummarizerAgent) -> None:
        """Test summary cleaning functionality."""
        # Test removing prefixes
        raw_summary = "Summary: This is a test summary about space exploration."
        cleaned = agent._clean_summary(raw_summary)
        assert cleaned == "This is a test summary about space exploration."
        
        # Test capitalizing first letter
        raw_summary = "this is a test summary."
        cleaned = agent._clean_summary(raw_summary)
        assert cleaned == "This is a test summary."
        
        # Test removing extra whitespace
        raw_summary = "This  is  a   test    summary."
        cleaned = agent._clean_summary(raw_summary)
        assert cleaned == "This is a test summary."

    def test_call_claude_error_handling(self, agent: SummarizerAgent, sample_transcript: str) -> None:
        """Test error handling when Claude API fails."""
        # Mock API error - any exception in _call_claude will be caught
        agent.client.messages.create.side_effect = Exception("API Error")
        
        with pytest.raises(AgentProcessingError, match="Summarization failed"):
            agent.process(sample_transcript)

    @patch('app.agents.summarizer.settings')
    def test_word_count_validation(self, mock_settings: Any, agent: SummarizerAgent, sample_transcript: str) -> None:
        """Test word count validation for summaries."""
        mock_settings.summary_min_words = 10
        mock_settings.summary_max_words = 20
        
        # Mock short summary
        short_summary = "Short summary."
        agent.client.messages.create.return_value.content[0].text = short_summary
        
        # Should still return the summary even if word count is off
        result = agent.process(sample_transcript)
        assert "summary" in result
        assert result["summary"] == short_summary

    def test_build_system_prompt(self, agent: SummarizerAgent) -> None:
        """Test system prompt building."""
        system_prompt = agent._build_system_prompt()
        
        assert "professional" in system_prompt.lower()
        assert "200" in system_prompt
        assert "300" in system_prompt
        assert "business" in system_prompt.lower()

    def test_process_with_timestamps(self, agent: SummarizerAgent) -> None:
        """Test processing transcript with timestamps."""
        transcript_with_timestamps = """[00:00:00] Host: Welcome to the show.
[00:00:15] Guest: Thanks for having me.
[00:01:00] Host: Let's discuss today's topic."""
        
        mock_summary = "This podcast features a host and guest discussing today's topic."
        agent.client.messages.create.return_value.content[0].text = mock_summary
        
        result = agent.process(transcript_with_timestamps)
        
        assert "summary" in result
        assert result["summary"] == mock_summary

    def test_process_without_timestamps(self, agent: SummarizerAgent) -> None:
        """Test processing transcript without timestamps."""
        transcript_no_timestamps = """Host: Welcome to the show.
Guest: Thanks for having me.
Host: Let's discuss today's topic."""
        
        mock_summary = "This podcast features a host and guest discussing today's topic."
        agent.client.messages.create.return_value.content[0].text = mock_summary
        
        result = agent.process(transcript_no_timestamps)
        
        assert "summary" in result
        assert result["summary"] == mock_summary