"""
AI agent for extracting key takeaways from podcast transcripts.
Identifies and formats the most important insights and actionable points.
"""

import json
from typing import Dict, Any, List

from app.agents.base_agent import BaseAgent, AgentProcessingError
from app.utils.logger import get_logger

logger = get_logger(__name__)


class TakeawayExtractorAgent(BaseAgent):
    """
    AI agent that extracts key takeaways and insights from podcast transcripts.
    
    Identifies the most important, actionable, and memorable points from the
    discussion that would be valuable for someone to remember or act upon.
    """

    def process(self, transcript_content: str, summary: str = None, **kwargs) -> Dict[str, Any]:
        """
        Extract key takeaways from the podcast transcript.
        
        Args:
            transcript_content: The raw transcript text to analyze
            summary: Optional summary to provide context (from summarizer agent)
            **kwargs: Additional parameters (unused)
            
        Returns:
            Dictionary with 'takeaways' key containing list of key insights
            
        Raises:
            AgentProcessingError: If takeaway extraction fails
        """
        logger.info(f"[{self.agent_name}] Starting extraction of key points",
                   transcript_length=len(transcript_content),
                   has_summary=bool(summary))

        if not transcript_content.strip():
            raise AgentProcessingError("Cannot extract takeaways from empty transcript")

        # Create the extraction prompt
        prompt = self._build_extraction_prompt(transcript_content, summary)
        
        # Create system prompt for the extractor
        system_prompt = self._build_system_prompt()

        try:
            # Get takeaways from Claude
            raw_response = self._call_claude(prompt, system_prompt)
            
            # Parse and validate the takeaways
            takeaways = self._parse_takeaways(raw_response)
            
            if not takeaways:
                raise AgentProcessingError("No takeaways extracted from transcript")

            logger.info(f"[{self.agent_name}] Found {len(takeaways)} key takeaways")
            
            # Log the takeaways for visibility
            for i, takeaway in enumerate(takeaways, 1):
                logger.info(f"[{self.agent_name}]   {i}. {self._truncate_for_log(takeaway, 150)}")

            logger.info(f"[{self.agent_name}] Complete")

            return {"takeaways": takeaways}

        except Exception as e:
            logger.error(f"[{self.agent_name}] Failed to extract takeaways", error=str(e))
            raise AgentProcessingError(f"Takeaway extraction failed: {str(e)}")

    def _build_system_prompt(self) -> str:
        """Build the system prompt for Claude."""
        return """You are an expert at identifying key insights and actionable takeaways from podcast discussions.

Your task is to extract the most important, valuable, and memorable points that:
- Represent key insights or learnings shared during the discussion
- Are actionable or applicable to the audience
- Capture important facts, statistics, or expert opinions
- Highlight notable quotes or profound statements
- Include practical advice or recommendations mentioned
- Cover significant predictions or future outlook discussed

Focus on substantive content that would be valuable for someone to remember or act upon. Avoid:
- Basic introductory statements
- Small talk or casual conversation
- Obvious or common knowledge points
- Repetitive information

Return your response as a simple numbered list, with each takeaway as a complete, clear sentence."""

    def _build_extraction_prompt(self, transcript_content: str, summary: str = None) -> str:
        """
        Build the prompt for extracting takeaways.
        
        Args:
            transcript_content: The transcript to analyze
            summary: Optional summary for context
            
        Returns:
            Formatted prompt string
        """
        # Truncate very long transcripts for the prompt
        max_transcript_length = 12000  # Reasonable limit for Claude context
        if len(transcript_content) > max_transcript_length:
            transcript_content = transcript_content[:max_transcript_length] + "\n[...transcript truncated...]"
            logger.info(f"[{self.agent_name}] Truncated long transcript for processing")

        prompt = f"""Analyze the following podcast transcript and extract the key takeaways and insights.

Focus on identifying:
- Important facts, statistics, or expert insights
- Actionable advice or recommendations
- Significant predictions or future outlook
- Notable quotes or profound statements
- Key lessons learned or wisdom shared
- Practical tips mentioned

"""

        # Add summary context if available
        if summary:
            prompt += f"""CONTEXT SUMMARY:
{summary}

"""

        prompt += f"""TRANSCRIPT:
{transcript_content}

Please extract 4-8 key takeaways from this podcast. Format your response as a simple numbered list:

1. [First key takeaway]
2. [Second key takeaway]
3. [Third key takeaway]
etc.

KEY TAKEAWAYS:"""

        return prompt

    def _parse_takeaways(self, raw_response: str) -> List[str]:
        """
        Parse the takeaways from Claude's response.
        
        Args:
            raw_response: Raw response text from Claude
            
        Returns:
            List of cleaned takeaway strings
        """
        takeaways = []
        
        # Split response into lines
        lines = raw_response.strip().split('\n')
        
        for line in lines:
            line = line.strip()
            if not line:
                continue
            
            # Look for numbered list items
            # Patterns: "1. text", "1) text", "- text", "• text"
            import re
            
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
            
            # Skip if line is too short or doesn't look like a takeaway
            if len(cleaned_line.split()) < 3:
                continue
            
            # Skip common non-takeaway phrases
            skip_phrases = [
                "key takeaways",
                "takeaways:",
                "summary:",
                "in conclusion",
                "to summarize"
            ]
            
            if any(phrase in cleaned_line.lower() for phrase in skip_phrases):
                continue
            
            # Ensure it ends with proper punctuation
            if cleaned_line and not cleaned_line.endswith(('.', '!', '?')):
                cleaned_line += '.'
            
            # Ensure it starts with capital letter
            if cleaned_line and not cleaned_line[0].isupper():
                cleaned_line = cleaned_line[0].upper() + cleaned_line[1:]
            
            if cleaned_line:
                takeaways.append(cleaned_line)
        
        # Limit to reasonable number of takeaways
        if len(takeaways) > 10:
            takeaways = takeaways[:10]
            logger.warning(f"[{self.agent_name}] Truncated takeaways list to 10 items")
        
        return takeaways