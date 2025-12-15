"""
AI agent for fact-checking claims in podcast transcripts.
Extracts factual claims and verifies them using web search.
"""

import json
import re
from typing import Dict, Any, List, Tuple

from app.agents.base_agent import BaseAgent, AgentProcessingError
from app.services.serper_service import SerperService, SerperSearchError
from app.utils.logger import get_logger

logger = get_logger(__name__)


class FactCheckerAgent(BaseAgent):
    """
    AI agent that extracts and verifies factual claims from podcast transcripts.
    
    Uses Serper API for web search and Claude for analysis to identify factual claims
    and verify their accuracy against reliable sources.
    """
    
    def __init__(self) -> None:
        """Initialize the FactChecker agent with Serper service."""
        super().__init__()
        self.serper_service = SerperService()

    def process(self, transcript_content: str, **kwargs) -> Dict[str, Any]:
        """
        Extract and verify factual claims from the transcript.
        
        Args:
            transcript_content: The raw transcript text to fact-check
            **kwargs: Additional parameters (unused)
            
        Returns:
            Dictionary with 'fact_checks' key containing list of verification results
            
        Raises:
            AgentProcessingError: If fact-checking fails
        """
        logger.info(f"[{self.agent_name}] Starting fact verification",
                   transcript_length=len(transcript_content))

        if not transcript_content.strip():
            raise AgentProcessingError("Cannot fact-check empty transcript")

        try:
            # Step 1: Extract factual claims from transcript
            claims = self._extract_claims(transcript_content)
            
            if not claims:
                logger.info(f"[{self.agent_name}] No factual claims found in transcript")
                return {"fact_checks": []}

            logger.info(f"[{self.agent_name}] Extracted {len(claims)} factual claims from transcript")

            # Step 2: Verify each claim with rate limiting
            fact_checks = []
            for i, claim in enumerate(claims, 1):
                logger.info(f"[{self.agent_name}] Checking claim {i} of {len(claims)}: {self._truncate_for_log(claim, 100)}")
                
                try:
                    verification_result = self._verify_claim(claim)
                    fact_checks.append(verification_result)
                    
                    logger.info(f"[{self.agent_name}] Claim {i} result: {verification_result['verdict'].upper()} (confidence: {verification_result['confidence']:.2f}) - {self._truncate_for_log(verification_result.get('evidence', 'No evidence provided'), 100)}")
                    
                    # Add extra delay between claims to avoid hitting rate limits
                    if i < len(claims):  # Don't delay after the last claim
                        import time
                        time.sleep(3)  # 3 second delay between claims
                    
                except Exception as e:
                    logger.error(f"[{self.agent_name}] Failed to verify claim {i}", claim=claim, error=str(e))
                    # Continue with other claims instead of failing completely
                    fact_checks.append({
                        "claim": claim,
                        "verdict": "unverifiable",
                        "confidence": 0.0,
                        "evidence": f"Verification failed: {str(e)}",
                        "sources": []
                    })

            logger.info(f"[{self.agent_name}] Complete: {len(fact_checks)} claims verified ({sum(1 for fc in fact_checks if fc['verdict'] == 'true')} true, {sum(1 for fc in fact_checks if fc['verdict'] == 'partially_true')} partial, {sum(1 for fc in fact_checks if fc['verdict'] == 'unverifiable')} unverifiable)")

            return {"fact_checks": fact_checks}

        except Exception as e:
            logger.error(f"[{self.agent_name}] Fact-checking failed", error=str(e))
            raise AgentProcessingError(f"Fact-checking failed: {str(e)}")

    def _extract_claims(self, transcript_content: str) -> List[str]:
        """
        Extract factual claims from the transcript that can be verified.
        
        Args:
            transcript_content: The transcript to analyze
            
        Returns:
            List of factual claim strings
        """
        # Truncate very long transcripts
        max_transcript_length = 10000
        if len(transcript_content) > max_transcript_length:
            transcript_content = transcript_content[:max_transcript_length] + "\n[...transcript truncated...]"

        prompt = f"""Analyze the following podcast transcript and extract factual claims that can be verified.

Look for statements that:
- Make specific factual assertions about events, dates, numbers, or statistics
- Reference real people, companies, organizations, or places
- Mention scientific findings, research results, or studies
- Claim specific achievements, milestones, or historical events
- Make predictions with specific timelines or targets

Ignore:
- Opinions, beliefs, or personal views
- General statements without specific details
- Hypothetical scenarios
- Common knowledge facts
- Vague or ambiguous statements

TRANSCRIPT:
{transcript_content}

Extract 2-3 specific factual claims that can be verified. Format as a simple numbered list:

1. [First specific factual claim]
2. [Second specific factual claim]
etc.

FACTUAL CLAIMS:"""

        system_prompt = """You are an expert at identifying specific, verifiable factual claims in text. Focus on concrete statements that make specific assertions about real-world facts, events, dates, numbers, or entities that can be checked against reliable sources."""

        try:
            response = self._call_claude(prompt, system_prompt)
            return self._parse_claims(response)
        except Exception as e:
            logger.error(f"[{self.agent_name}] Failed to extract claims", error=str(e))
            return []

    def _parse_claims(self, raw_response: str) -> List[str]:
        """
        Parse claims from Claude's response.
        
        Args:
            raw_response: Raw response text from Claude
            
        Returns:
            List of claim strings
        """
        claims = []
        lines = raw_response.strip().split('\n')
        
        for line in lines:
            line = line.strip()
            if not line:
                continue
            
            # Remove list markers
            patterns = [
                r'^\d+\.\s*',     # 1. 
                r'^\d+\)\s*',     # 1) 
                r'^-\s*',         # - 
                r'^•\s*',         # • 
                r'^\*\s*',        # * 
            ]
            
            cleaned_line = line
            for pattern in patterns:
                cleaned_line = re.sub(pattern, '', cleaned_line)
            
            # Skip if too short
            if len(cleaned_line.split()) < 4:
                continue
            
            claims.append(cleaned_line)
        
        return claims[:3]  # Limit to 3 claims to reduce token usage

    def _verify_claim(self, claim: str) -> Dict[str, Any]:
        """
        Verify a single factual claim using Serper web search and Claude analysis.
        
        Args:
            claim: The claim to verify
            
        Returns:
            Dictionary with verification results
        """
        try:
            # Step 1: Use Serper to search for the claim
            logger.info(f"[{self.agent_name}] Searching for claim with Serper API")
            search_context = self.serper_service.search_for_claim(claim)
            
            if not search_context["snippets"]:
                logger.warning(f"[{self.agent_name}] No search results found for claim")
                return {
                    "claim": claim,
                    "verdict": "unverifiable",
                    "confidence": 0.0,
                    "evidence": "No search results found",
                    "sources": []
                }
            
            # Step 2: Use Claude to analyze the search results
            logger.info(f"[{self.agent_name}] Analyzing search results with Claude")
            analysis_result = self._analyze_search_results(claim, search_context)
            return analysis_result
            
        except SerperSearchError as e:
            logger.error(f"[{self.agent_name}] Serper search failed", 
                        claim=claim, error=str(e))
            return {
                "claim": claim,
                "verdict": "unverifiable",
                "confidence": 0.0,
                "evidence": "Web search failed",
                "sources": []
            }
        except Exception as e:
            logger.error(f"[{self.agent_name}] Claim verification failed", 
                        claim=claim, error=str(e))
            return {
                "claim": claim,
                "verdict": "unverifiable",
                "confidence": 0.0,
                "evidence": f"Verification failed: {str(e)}",
                "sources": []
            }
    
    def _analyze_search_results(self, claim: str, search_context: Dict[str, Any]) -> Dict[str, Any]:
        """
        Use Claude to analyze search results and determine claim validity.
        
        Args:
            claim: The original claim to verify
            search_context: Search results from Serper
            
        Returns:
            Dictionary with verification results
        """
        # Format search results for Claude
        formatted_results = self._format_search_results_for_analysis(search_context)
        
        prompt = f"""Analyze the following search results to verify this claim:

CLAIM: {claim}

SEARCH RESULTS:
{formatted_results}

Based on these search results, provide your assessment:

VERDICT: [true/false/partially_true/unverifiable]
CONFIDENCE: [0.0-1.0]
EVIDENCE: [Brief explanation in 1-2 sentences max]
SOURCES: [List the most relevant source URLs from the search results]

Guidelines:
- true: Claim is fully supported by reliable sources
- false: Claim is contradicted by reliable sources  
- partially_true: Claim has some truth but lacks important context/nuance
- unverifiable: Insufficient or unreliable sources to make determination

Be concise and focus on the most relevant evidence."""

        system_prompt = """You are a professional fact-checker analyzing web search results. Evaluate claims objectively based on source quality and evidence strength. Be precise and concise in your assessment."""
        
        try:
            # No web search tools needed - we already have the search results
            response = self._call_claude(prompt, system_prompt, max_retries=1)
            return self._parse_verification_result(claim, response, search_context["sources"])
            
        except Exception as e:
            logger.error(f"[{self.agent_name}] Claude analysis failed", error=str(e))
            return {
                "claim": claim,
                "verdict": "unverifiable",
                "confidence": 0.0,
                "evidence": "Analysis failed",
                "sources": search_context.get("sources", [])
            }
    
    def _format_search_results_for_analysis(self, search_context: Dict[str, Any]) -> str:
        """
        Format search results into a readable text for Claude analysis.
        
        Args:
            search_context: Search results from Serper
            
        Returns:
            Formatted string of search results
        """
        formatted_results = []
        
        for i, snippet_data in enumerate(search_context["snippets"][:3], 1):  # Limit to top 3 results
            title = snippet_data.get("title", "")
            snippet = snippet_data.get("snippet", "")
            url = snippet_data.get("url", "")
            
            result_text = f"Result {i}:\nTitle: {title}\nSnippet: {snippet}"
            if url:
                result_text += f"\nSource: {url}"
            
            formatted_results.append(result_text)
        
        return "\n\n".join(formatted_results)


    def _parse_verification_result(self, claim: str, response: str, available_sources: List[str] = None) -> Dict[str, Any]:
        """
        Parse the verification result from Claude's response.
        
        Args:
            claim: Original claim being verified
            response: Claude's verification response
            available_sources: List of source URLs from search results
            
        Returns:
            Dictionary with parsed verification results
        """
        # Extract verdict
        verdict_match = re.search(r'VERDICT:\s*(\w+)', response, re.IGNORECASE)
        verdict = verdict_match.group(1).lower() if verdict_match else "unverifiable"
        
        # Ensure valid verdict
        valid_verdicts = ["true", "false", "partially_true", "unverifiable"]
        if verdict not in valid_verdicts:
            verdict = "unverifiable"
        
        # Extract confidence
        confidence_match = re.search(r'CONFIDENCE:\s*([\d.]+)', response, re.IGNORECASE)
        try:
            confidence = float(confidence_match.group(1)) if confidence_match else 0.5
            confidence = max(0.0, min(1.0, confidence))  # Clamp to valid range
        except ValueError:
            confidence = 0.5
        
        # Extract evidence
        evidence_match = re.search(r'EVIDENCE:\s*(.+?)(?=SOURCES:|$)', response, re.IGNORECASE | re.DOTALL)
        evidence = evidence_match.group(1).strip() if evidence_match else "No evidence provided"
        
        # Extract sources
        sources_match = re.search(r'SOURCES:\s*(.+?)$', response, re.IGNORECASE | re.DOTALL)
        sources_text = sources_match.group(1).strip() if sources_match else ""
        
        # Parse URLs from sources text
        sources = []
        if sources_text and sources_text != "[]":
            # Extract URLs using regex
            url_pattern = r'https?://[^\s\],]+'
            found_urls = re.findall(url_pattern, sources_text)
            
            # Validate against available sources if provided
            if available_sources:
                sources = [url for url in found_urls if url in available_sources]
            else:
                sources = found_urls
        
        # If no sources found but we have available sources, use them as fallback
        if not sources and available_sources:
            sources = available_sources[:2]  # Limit to first 2 sources
        
        return {
            "claim": claim,
            "verdict": verdict,
            "confidence": confidence,
            "evidence": evidence,
            "sources": sources
        }