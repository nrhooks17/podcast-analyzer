"""
Tests for the FactCheckerAgent AI agent.
"""

import pytest
from typing import Dict, Any, List
from unittest.mock import Mock, patch

from app.agents.fact_checker import FactCheckerAgent
from app.agents.base_agent import AgentProcessingError
from app.services.serper_service import SerperSearchError


class TestFactCheckerAgent:
    """Test the FactCheckerAgent class."""

    @pytest.fixture
    def agent(self, mock_anthropic_client: Mock) -> FactCheckerAgent:
        """Create FactCheckerAgent instance with mocked client and Serper service."""
        agent = FactCheckerAgent()
        agent.client = mock_anthropic_client
        # Mock the Serper service
        agent.serper_service = Mock()
        return agent

    @pytest.fixture
    def sample_transcript(self) -> str:
        """Sample transcript with factual claims for testing."""
        return """[00:00:00] Host: Welcome to the show. Today we're discussing space exploration with our guest.
[00:00:15] Guest: Thanks for having me. NASA's Mars mission is launching in 2026, which is incredibly exciting.
[00:01:02] Host: That's fascinating. What about SpaceX's role in this?
[00:01:30] Guest: SpaceX is collaborating with NASA on several Mars projects. They're planning to send humans to Mars by 2030.
[00:02:15] Host: Interesting. I've also heard Bitcoin will reach $200,000 by 2025.
[00:02:45] Guest: That's pure speculation. Let's stick to the science."""

    def test_process_success_with_claims(self, agent: FactCheckerAgent, sample_transcript: str) -> None:
        """Test successful fact-checking with extractable claims."""
        # Mock the claim extraction response
        agent.client.messages.create.return_value.content[0].text = """
1. NASA's Mars mission is launching in 2026
2. SpaceX is collaborating with NASA on Mars projects
3. Bitcoin will reach $200,000 by 2025
"""
        
        # Mock Serper search results for each claim
        search_results = [
            {
                "original_claim": "NASA's Mars mission is launching in 2026",
                "search_query": "NASA Mars mission launching 2026",
                "snippets": [
                    {
                        "title": "NASA Mars Mission 2026",
                        "snippet": "NASA has confirmed Mars mission timeline for 2026 based on official announcements",
                        "url": "https://www.nasa.gov/mars-mission"
                    }
                ],
                "sources": ["https://www.nasa.gov/mars-mission"],
                "total_results": 1
            },
            {
                "original_claim": "SpaceX is collaborating with NASA on Mars projects",
                "search_query": "SpaceX NASA Mars collaboration projects",
                "snippets": [
                    {
                        "title": "SpaceX NASA Partnership",
                        "snippet": "SpaceX partnership with NASA on Mars exploration officially announced",
                        "url": "https://www.spacex.com/mars"
                    }
                ],
                "sources": ["https://www.spacex.com/mars", "https://www.nasa.gov/partnerships"],
                "total_results": 2
            },
            {
                "original_claim": "Bitcoin will reach $200,000 by 2025",
                "search_query": "Bitcoin price prediction 200000 2025",
                "snippets": [],
                "sources": [],
                "total_results": 0
            }
        ]
        
        # Mock verification responses from Claude analyzing search results
        verification_responses = [
            """VERDICT: true
CONFIDENCE: 0.85
EVIDENCE: NASA has confirmed Mars mission timeline for 2026 based on official announcements and mission planning documents.
SOURCES: https://www.nasa.gov/mars-mission""",
            """VERDICT: true
CONFIDENCE: 0.90
EVIDENCE: SpaceX partnership with NASA on Mars exploration has been officially announced and documented.
SOURCES: https://www.spacex.com/mars""",
            """VERDICT: unverifiable
CONFIDENCE: 0.10
EVIDENCE: Cryptocurrency price predictions are speculative and cannot be verified against authoritative sources.
SOURCES: []"""
        ]
        
        # Configure mocks
        agent.serper_service.search_for_claim.side_effect = search_results
        agent.client.messages.create.side_effect = [
            Mock(content=[Mock(text=agent.client.messages.create.return_value.content[0].text)]),  # Claim extraction
            *[Mock(content=[Mock(text=resp)]) for resp in verification_responses]  # Claude analysis
        ]
        
        result = agent.process(sample_transcript)
        
        assert "fact_checks" in result
        assert len(result["fact_checks"]) == 3
        
        # Check first fact check
        first_check = result["fact_checks"][0]
        assert first_check["claim"] == "NASA's Mars mission is launching in 2026"
        assert first_check["verdict"] == "true"
        assert first_check["confidence"] == 0.85
        assert "NASA has confirmed" in first_check["evidence"]
        assert "https://www.nasa.gov/mars-mission" in first_check["sources"]
        
        # Check unverifiable claim
        crypto_check = result["fact_checks"][2]
        assert crypto_check["verdict"] == "unverifiable"
        assert crypto_check["confidence"] == 0.0

    def test_process_no_claims_found(self, agent: FactCheckerAgent) -> None:
        """Test processing transcript with no factual claims."""
        transcript = "This is just an opinion-based discussion without specific facts."
        
        # Mock the _extract_claims method to return empty list (which is what happens when no claims found)
        with patch.object(agent, '_extract_claims', return_value=[]):
            result = agent.process(transcript)
        
        assert "fact_checks" in result
        assert result["fact_checks"] == []

    def test_process_empty_transcript(self, agent: FactCheckerAgent) -> None:
        """Test processing empty transcript."""
        with pytest.raises(AgentProcessingError, match="Cannot fact-check empty transcript"):
            agent.process("")

    def test_process_whitespace_transcript(self, agent: FactCheckerAgent) -> None:
        """Test processing transcript with only whitespace."""
        with pytest.raises(AgentProcessingError, match="Cannot fact-check empty transcript"):
            agent.process("   \n\t  ")

    def test_extract_claims_success(self, agent: FactCheckerAgent, sample_transcript: str) -> None:
        """Test successful claim extraction."""
        # Mock Claude response for claim extraction
        agent.client.messages.create.return_value.content[0].text = """
Here are the factual claims I found:

1. NASA's Mars mission is launching in 2026
2. SpaceX is collaborating with NASA on Mars projects  
3. Humans will be sent to Mars by 2030
4. Bitcoin will reach $200,000 by 2025
"""
        
        claims = agent._extract_claims(sample_transcript)
        
        # The actual implementation limits to 3 claims and filters by word count
        assert len(claims) >= 1  # At least some claims should be extracted
        
        # Check that valid claims with 4+ words are included
        for claim in claims:
            assert len(claim.split()) >= 4

    def test_extract_claims_limits_to_three(self, agent: FactCheckerAgent) -> None:
        """Test that claim extraction limits results to 3 claims."""
        # Mock response with many claims (need at least 4 words each to pass filtering)
        many_claims = "\n".join([f"{i}. This is claim number {i}" for i in range(1, 10)])
        agent.client.messages.create.return_value.content[0].text = many_claims
        
        claims = agent._extract_claims("Long transcript with many claims")
        
        # The actual implementation limits to 3 claims
        assert len(claims) == 3

    def test_parse_claims_various_formats(self, agent: FactCheckerAgent) -> None:
        """Test parsing claims from different list formats."""
        response = """
1. First claim with number
2) Second claim with parenthesis  
- Third claim with dash
• Fourth claim with bullet
* Fifth claim with asterisk
Regular sentence without marker
"""
        
        claims = agent._parse_claims(response)
        
        # Check that various format markers are handled
        assert len(claims) >= 3  # At least some claims should be extracted
        
        # Check that markers are removed from claims
        for claim in claims:
            assert not claim.startswith(("1.", "2)", "-", "•", "*"))

    def test_parse_claims_filters_short_lines(self, agent: FactCheckerAgent) -> None:
        """Test that parsing filters out very short lines."""
        response = """
1. This is a proper claim with enough words
2. Short
3. Another valid claim that should be included
4. No
"""
        
        claims = agent._parse_claims(response)
        
        assert len(claims) == 2
        assert "This is a proper claim with enough words" in claims
        assert "Another valid claim that should be included" in claims
        assert "Short" not in claims
        assert "No" not in claims

    def test_verify_claim_web_search_failure(self, agent: FactCheckerAgent) -> None:
        """Test claim verification when Serper search fails."""
        claim = "Test claim that cannot be verified"
        
        # Mock Serper search failure
        agent.serper_service.search_for_claim.side_effect = SerperSearchError("API Error")
        
        result = agent._verify_claim(claim)
        
        assert result["claim"] == claim
        assert result["verdict"] == "unverifiable"
        assert result["confidence"] == 0.0
        assert "Web search failed" in result["evidence"]
        assert result["sources"] == []

    def test_verify_claim_success(self, agent: FactCheckerAgent) -> None:
        """Test successful claim verification with Serper search and Claude analysis."""
        claim = "NASA launched a mission to Mars in 2020"
        
        # Mock Serper search results
        search_context = {
            "original_claim": claim,
            "search_query": "NASA mission Mars 2020",
            "snippets": [
                {
                    "title": "NASA Mars 2020 Mission",
                    "snippet": "NASA's Perseverance rover was successfully launched to Mars in July 2020",
                    "url": "https://www.nasa.gov/mars2020"
                }
            ],
            "sources": ["https://www.nasa.gov/mars2020", "https://mars.nasa.gov/mars2020/"],
            "total_results": 1
        }
        
        agent.serper_service.search_for_claim.return_value = search_context
        
        # Mock successful Claude analysis response
        agent.client.messages.create.return_value.content[0].text = """
VERDICT: true
CONFIDENCE: 0.92
EVIDENCE: NASA's Perseverance rover was successfully launched to Mars in July 2020 as part of the Mars 2020 mission.
SOURCES: https://www.nasa.gov/mars2020, https://mars.nasa.gov/mars2020/
"""
        
        result = agent._verify_claim(claim)
        
        assert result["claim"] == claim
        assert result["verdict"] == "true"
        assert result["confidence"] == 0.92
        assert "Perseverance rover" in result["evidence"]
        assert len(result["sources"]) == 2
        assert "https://www.nasa.gov/mars2020" in result["sources"]

    def test_parse_verification_result_all_fields(self, agent: FactCheckerAgent) -> None:
        """Test parsing verification result with all fields present."""
        claim = "Test claim"
        response = """
VERDICT: partially_true
CONFIDENCE: 0.65
EVIDENCE: The claim is mostly accurate but lacks important context about timing and specific details.
SOURCES: https://example.com/source1, https://example.com/source2
"""
        
        result = agent._parse_verification_result(claim, response)
        
        assert result["claim"] == claim
        assert result["verdict"] == "partially_true"
        assert result["confidence"] == 0.65
        assert "mostly accurate" in result["evidence"]
        assert len(result["sources"]) == 2

    def test_parse_verification_result_missing_fields(self, agent: FactCheckerAgent) -> None:
        """Test parsing verification result with missing fields."""
        claim = "Test claim"
        response = "Some response without proper formatting"
        
        result = agent._parse_verification_result(claim, response)
        
        assert result["claim"] == claim
        assert result["verdict"] == "unverifiable"  # Default value
        assert result["confidence"] == 0.5  # Default value
        assert result["evidence"] == "No evidence provided"  # Default value
        assert result["sources"] == []  # Default value

    def test_parse_verification_result_invalid_verdict(self, agent: FactCheckerAgent) -> None:
        """Test parsing with invalid verdict values."""
        claim = "Test claim"
        response = """
VERDICT: invalid_verdict_type
CONFIDENCE: 0.75
EVIDENCE: Some evidence
"""
        
        result = agent._parse_verification_result(claim, response)
        
        assert result["verdict"] == "unverifiable"  # Should default to unverifiable
        assert result["confidence"] == 0.75

    def test_parse_verification_result_invalid_confidence(self, agent: FactCheckerAgent) -> None:
        """Test parsing with invalid confidence values."""
        claim = "Test claim"
        response = """
VERDICT: true
CONFIDENCE: invalid_number
EVIDENCE: Some evidence
"""
        
        result = agent._parse_verification_result(claim, response)
        
        assert result["confidence"] == 0.5  # Should default to 0.5

    def test_parse_verification_result_confidence_clamping(self, agent: FactCheckerAgent) -> None:
        """Test that confidence values are clamped to valid range."""
        claim = "Test claim"
        
        # Test upper bound
        response_high = """
VERDICT: true
CONFIDENCE: 1.5
EVIDENCE: Some evidence
"""
        result_high = agent._parse_verification_result(claim, response_high)
        assert result_high["confidence"] <= 1.0
        
        # Test lower bound
        response_low = """
VERDICT: true  
CONFIDENCE: -0.5
EVIDENCE: Some evidence
"""
        result_low = agent._parse_verification_result(claim, response_low)
        assert result_low["confidence"] >= 0.0

    def test_process_handles_individual_claim_failures(self, agent: FactCheckerAgent, sample_transcript: str) -> None:
        """Test that process continues even if individual claims fail to verify."""
        # Mock the _extract_claims to return specific claims
        with patch.object(agent, '_extract_claims', return_value=["First claim", "Second claim"]):
            # Mock Serper to succeed for first, fail for second
            def mock_search_for_claim(claim):
                if "First" in claim:
                    return {
                        "original_claim": claim,
                        "search_query": claim,
                        "snippets": [{"title": "Test", "snippet": "Good evidence", "url": "http://example.com"}],
                        "sources": ["http://example.com"],
                        "total_results": 1
                    }
                else:
                    # This will trigger the exception handling in the main process loop
                    raise SerperSearchError("Search failed")
            
            agent.serper_service.search_for_claim.side_effect = mock_search_for_claim
            
            # Mock Claude for successful analysis of first claim
            agent.client.messages.create.return_value.content[0].text = """VERDICT: true
CONFIDENCE: 0.8
EVIDENCE: Good evidence
SOURCES: http://example.com"""
            
            result = agent.process(sample_transcript)
        
        assert "fact_checks" in result
        assert len(result["fact_checks"]) == 2
        
        # First should succeed
        assert result["fact_checks"][0]["verdict"] == "true"
        
        # Second should be marked as unverifiable due to failure (as per the code logic)
        assert result["fact_checks"][1]["verdict"] == "unverifiable"
        assert "Web search failed" in result["fact_checks"][1]["evidence"]