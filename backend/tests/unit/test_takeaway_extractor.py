"""
Tests for the TakeawayExtractorAgent AI agent.
"""

import pytest
from typing import Dict, Any, List
from unittest.mock import Mock, patch

from app.agents.takeaway_extractor import TakeawayExtractorAgent
from app.agents.base_agent import AgentProcessingError


class TestTakeawayExtractorAgent:
    """Test the TakeawayExtractorAgent class."""

    @pytest.fixture
    def agent(self, mock_anthropic_client: Mock) -> TakeawayExtractorAgent:
        """Create TakeawayExtractorAgent instance with mocked client."""
        agent = TakeawayExtractorAgent()
        agent.client = mock_anthropic_client
        return agent

    @pytest.fixture
    def sample_transcript(self) -> str:
        """Sample transcript for testing."""
        return """[00:00:00] Host: Welcome to the show. Today we're discussing space exploration with our expert guest.
[00:00:15] Guest: Thanks for having me. NASA's Mars mission launching in 2026 represents a significant milestone in human space exploration.
[00:01:02] Host: What advice would you give to young people interested in space careers?
[00:01:30] Guest: First, focus on STEM education. Second, stay curious and never stop learning. The space industry needs diverse talents.
[00:02:15] Host: Any predictions for the next decade?
[00:02:45] Guest: I believe we'll see the first human landing on Mars by 2030, and space tourism will become mainstream by 2035."""

    @pytest.fixture
    def sample_summary(self) -> str:
        """Sample summary for context testing."""
        return "This podcast discusses space exploration, focusing on NASA's Mars mission and career advice for aspiring space professionals."

    def test_process_success_with_takeaways(self, agent: TakeawayExtractorAgent, sample_transcript: str) -> None:
        """Test successful takeaway extraction."""
        # Mock Claude response
        mock_response = """1. NASA's Mars mission in 2026 represents a major milestone in human space exploration
2. Focus on STEM education to pursue a career in the space industry
3. Stay curious and never stop learning to succeed in space-related fields
4. The space industry needs diverse talents and backgrounds
5. First human Mars landing predicted for 2030
6. Space tourism expected to become mainstream by 2035"""
        
        agent.client.messages.create.return_value.content[0].text = mock_response
        
        result = agent.process(sample_transcript)
        
        assert "takeaways" in result
        assert len(result["takeaways"]) == 6
        
        # Check specific takeaways
        takeaways = result["takeaways"]
        assert "NASA's Mars mission in 2026 represents a major milestone in human space exploration." in takeaways
        assert "Focus on STEM education to pursue a career in the space industry." in takeaways
        assert "Stay curious and never stop learning to succeed in space-related fields." in takeaways

    def test_process_with_summary_context(self, agent: TakeawayExtractorAgent, sample_transcript: str, sample_summary: str) -> None:
        """Test processing with summary context."""
        mock_response = """1. STEM education is crucial for space careers
2. Curiosity and continuous learning are essential
3. Mars landing expected by 2030"""
        
        agent.client.messages.create.return_value.content[0].text = mock_response
        
        result = agent.process(sample_transcript, summary=sample_summary)
        
        assert "takeaways" in result
        assert len(result["takeaways"]) == 3
        
        # Verify the API call included the summary in the prompt
        agent.client.messages.create.assert_called_once()
        call_args = agent.client.messages.create.call_args
        prompt = call_args[1]["messages"][0]["content"]
        assert "CONTEXT SUMMARY:" in prompt
        assert sample_summary in prompt

    def test_process_empty_transcript(self, agent: TakeawayExtractorAgent) -> None:
        """Test processing empty transcript."""
        with pytest.raises(AgentProcessingError, match="Cannot extract takeaways from empty transcript"):
            agent.process("")

    def test_process_whitespace_transcript(self, agent: TakeawayExtractorAgent) -> None:
        """Test processing transcript with only whitespace."""
        with pytest.raises(AgentProcessingError, match="Cannot extract takeaways from empty transcript"):
            agent.process("   \n\t  ")

    def test_process_no_takeaways_extracted(self, agent: TakeawayExtractorAgent, sample_transcript: str) -> None:
        """Test when no takeaways are extracted."""
        # Mock response that results in empty takeaways after parsing
        # Use content that gets filtered out (less than 3 words or contains skip phrases)
        agent.client.messages.create.return_value.content[0].text = "Summary: No takeaways\nkey takeaways found\nshort"
        
        # This should trigger the "No takeaways extracted" error since _parse_takeaways returns empty list
        with pytest.raises(AgentProcessingError, match="No takeaways extracted from transcript"):
            agent.process(sample_transcript)

    def test_process_long_transcript_truncation(self, agent: TakeawayExtractorAgent) -> None:
        """Test that very long transcripts are truncated."""
        # Create a very long transcript
        long_transcript = "[00:00:00] Host: " + "Very long content. " * 2000
        
        # Mock takeaway response
        mock_response = "1. Key insight from long content"
        agent.client.messages.create.return_value.content[0].text = mock_response
        
        result = agent.process(long_transcript)
        
        assert "takeaways" in result
        assert len(result["takeaways"]) == 1
        
        # Verify the call was made with truncated content
        agent.client.messages.create.assert_called_once()
        call_args = agent.client.messages.create.call_args
        prompt = call_args[1]["messages"][0]["content"]
        assert "[...transcript truncated...]" in prompt

    def test_build_extraction_prompt(self, agent: TakeawayExtractorAgent, sample_transcript: str) -> None:
        """Test prompt building."""
        prompt = agent._build_extraction_prompt(sample_transcript)
        
        assert "Analyze the following podcast transcript" in prompt
        assert "TRANSCRIPT:" in prompt
        assert sample_transcript in prompt
        assert "KEY TAKEAWAYS:" in prompt
        assert "4-8 key takeaways" in prompt

    def test_build_extraction_prompt_with_summary(self, agent: TakeawayExtractorAgent, sample_transcript: str, sample_summary: str) -> None:
        """Test prompt building with summary context."""
        prompt = agent._build_extraction_prompt(sample_transcript, sample_summary)
        
        assert "CONTEXT SUMMARY:" in prompt
        assert sample_summary in prompt
        assert "TRANSCRIPT:" in prompt
        assert sample_transcript in prompt

    def test_parse_takeaways_numbered_list(self, agent: TakeawayExtractorAgent) -> None:
        """Test parsing numbered list takeaways."""
        response = """1. First takeaway about space exploration
2. Second takeaway about career advice
3. Third takeaway about future predictions"""
        
        takeaways = agent._parse_takeaways(response)
        
        assert len(takeaways) == 3
        assert "First takeaway about space exploration." in takeaways
        assert "Second takeaway about career advice." in takeaways
        assert "Third takeaway about future predictions." in takeaways

    def test_parse_takeaways_various_formats(self, agent: TakeawayExtractorAgent) -> None:
        """Test parsing takeaways from different list formats."""
        response = """1. First takeaway with number
2) Second takeaway with parenthesis
- Third takeaway with dash
• Fourth takeaway with bullet
* Fifth takeaway with asterisk"""
        
        takeaways = agent._parse_takeaways(response)
        
        assert len(takeaways) == 5
        expected_takeaways = [
            "First takeaway with number.",
            "Second takeaway with parenthesis.",
            "Third takeaway with dash.",
            "Fourth takeaway with bullet.",
            "Fifth takeaway with asterisk."
        ]
        
        for expected in expected_takeaways:
            assert expected in takeaways

    def test_parse_takeaways_filters_short_lines(self, agent: TakeawayExtractorAgent) -> None:
        """Test that parsing filters out very short lines."""
        response = """1. This is a proper takeaway with enough words
2. Short
3. Another valid takeaway that should be included
4. No
5. Valid takeaway here"""
        
        takeaways = agent._parse_takeaways(response)
        
        # Should only include lines with 3+ words
        assert len(takeaways) == 3
        assert "This is a proper takeaway with enough words." in takeaways
        assert "Another valid takeaway that should be included." in takeaways
        assert "Valid takeaway here." in takeaways
        assert "Short." not in takeaways
        assert "No." not in takeaways

    def test_parse_takeaways_skips_header_phrases(self, agent: TakeawayExtractorAgent) -> None:
        """Test that parsing skips common header phrases."""
        response = """Key takeaways:
1. Important insight about technology
Summary:
2. Another valuable point about innovation
3. Final takeaway about future trends"""
        
        takeaways = agent._parse_takeaways(response)
        
        assert len(takeaways) == 3
        assert "Important insight about technology." in takeaways
        assert "Another valuable point about innovation." in takeaways
        assert "Final takeaway about future trends." in takeaways
        # Should not include header phrases
        for takeaway in takeaways:
            assert "key takeaways" not in takeaway.lower()
            assert "summary:" not in takeaway.lower()

    def test_parse_takeaways_adds_punctuation(self, agent: TakeawayExtractorAgent) -> None:
        """Test that parsing adds proper punctuation."""
        response = """1. This takeaway needs punctuation
2. This one already has punctuation.
3. This one has a question mark?
4. This one has an exclamation point!"""
        
        takeaways = agent._parse_takeaways(response)
        
        assert len(takeaways) == 4
        assert all(takeaway.endswith(('.', '!', '?')) for takeaway in takeaways)
        assert "This takeaway needs punctuation." in takeaways

    def test_parse_takeaways_capitalizes_first_letter(self, agent: TakeawayExtractorAgent) -> None:
        """Test that parsing capitalizes first letter."""
        response = """1. properly capitalized takeaway
2. Another uncapitalized takeaway
3. Already Capitalized Takeaway"""
        
        takeaways = agent._parse_takeaways(response)
        
        assert len(takeaways) == 3
        assert all(takeaway[0].isupper() for takeaway in takeaways)
        assert "Properly capitalized takeaway." in takeaways
        assert "Another uncapitalized takeaway." in takeaways

    def test_parse_takeaways_limits_to_ten(self, agent: TakeawayExtractorAgent) -> None:
        """Test that parsing limits results to 10 takeaways."""
        # Create response with many takeaways
        lines = [f"{i}. Takeaway number {i}" for i in range(1, 15)]
        response = "\n".join(lines)
        
        takeaways = agent._parse_takeaways(response)
        
        assert len(takeaways) <= 10

    def test_build_system_prompt(self, agent: TakeawayExtractorAgent) -> None:
        """Test system prompt building."""
        system_prompt = agent._build_system_prompt()
        
        assert "expert at identifying key insights" in system_prompt
        assert "actionable takeaways" in system_prompt
        assert "valuable" in system_prompt
        assert "numbered list" in system_prompt
        assert "substantive content" in system_prompt

    def test_call_claude_error_handling(self, agent: TakeawayExtractorAgent, sample_transcript: str) -> None:
        """Test error handling when Claude API fails."""
        # Mock API error - any exception in _call_claude will be caught
        agent.client.messages.create.side_effect = Exception("API Error")
        
        with pytest.raises(AgentProcessingError, match="Takeaway extraction failed"):
            agent.process(sample_transcript)

    def test_process_without_summary(self, agent: TakeawayExtractorAgent, sample_transcript: str) -> None:
        """Test processing without summary context."""
        mock_response = """1. First takeaway here
2. Second takeaway content
3. Third takeaway point"""
        
        agent.client.messages.create.return_value.content[0].text = mock_response
        
        # Process without summary parameter (should default to None)
        result = agent.process(sample_transcript)
        
        assert "takeaways" in result
        assert len(result["takeaways"]) == 3
        
        # Verify the API was called
        agent.client.messages.create.assert_called_once()

    def test_process_with_complex_transcript_formats(self, agent: TakeawayExtractorAgent) -> None:
        """Test processing transcripts with various formats."""
        complex_transcript = """Host: Welcome everyone to today's discussion.
[00:01:00] Expert: The key to success is consistency and dedication.
Guest Speaker: I completely agree. Here are three practical tips:
1. Set clear goals
2. Track your progress
3. Celebrate small wins
[00:02:30] Host: Excellent advice!"""
        
        mock_response = """1. Consistency and dedication are key to success
2. Set clear goals for better outcomes
3. Track your progress regularly
4. Celebrate small wins to maintain motivation"""
        
        agent.client.messages.create.return_value.content[0].text = mock_response
        
        result = agent.process(complex_transcript)
        
        assert "takeaways" in result
        assert len(result["takeaways"]) == 4

    def test_parse_takeaways_with_mixed_content(self, agent: TakeawayExtractorAgent) -> None:
        """Test parsing response with mixed content and formatting."""
        response = """Here are the key takeaways from the podcast:

1. Technology is rapidly changing the business landscape
2. Companies need to adapt or risk becoming obsolete

Additional insights:
3. Remote work is here to stay
4. Digital transformation is essential

- Investment in employee training is crucial
- Customer experience should be the top priority

Final thoughts: These changes require strong leadership."""
        
        takeaways = agent._parse_takeaways(response)
        
        # Should extract valid takeaways regardless of mixed formatting
        assert len(takeaways) >= 6
        assert any("Technology is rapidly changing" in takeaway for takeaway in takeaways)
        assert any("Remote work is here to stay" in takeaway for takeaway in takeaways)
        assert any("Investment in employee training" in takeaway for takeaway in takeaways)