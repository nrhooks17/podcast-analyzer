"""
AI agent for generating podcast summaries.
Creates concise 200-300 word summaries of podcast content.
"""

import re
from typing import Dict, Any

from app.agents.base_agent import BaseAgent, AgentProcessingError
from app.config import get_settings
from app.utils.logger import get_logger

logger = get_logger(__name__)
settings = get_settings()


class SummarizerAgent(BaseAgent):
    """
    AI agent that generates concise summaries of podcast transcripts.
    
    Creates professional, informative summaries between 200-300 words
    that capture the main topics, key points, and overall theme of the podcast.
    """

    def process(self, transcript_content: str, **kwargs) -> Dict[str, Any]:
        """
        Generate a summary of the podcast transcript.
        
        Args:
            transcript_content: The raw transcript text to summarize
            **kwargs: Additional parameters (unused)
            
        Returns:
            Dictionary with 'summary' key containing the generated summary
            
        Raises:
            AgentProcessingError: If summarization fails
        """
        logger.info(f"[{self.agent_name}] Starting summarization of transcript",
                   transcript_length=len(transcript_content),
                   word_count=len(transcript_content.split()))

        if not transcript_content.strip():
            raise AgentProcessingError("Cannot summarize empty transcript")

        # Create the summarization prompt
        prompt = self._build_summarization_prompt(transcript_content)
        
        # Create system prompt for the summarizer
        system_prompt = self._build_system_prompt()

        try:
            # Get summary from Claude
            raw_summary = self._call_claude(prompt, system_prompt)
            
            # Clean and validate the summary
            summary = self._clean_summary(raw_summary)
            
            # Validate word count
            word_count = len(summary.split())
            if word_count < settings.summary_min_words or word_count > settings.summary_max_words:
                logger.warning(f"[{self.agent_name}] Summary word count outside target range",
                              word_count=word_count,
                              min_words=settings.summary_min_words,
                              max_words=settings.summary_max_words)

            logger.info(f"[{self.agent_name}] Generated summary successfully",
                       summary_word_count=word_count,
                       summary_preview=self._truncate_for_log(summary, 100))

            return {"summary": summary}

        except Exception as e:
            logger.error(f"[{self.agent_name}] Failed to generate summary", error=str(e))
            raise AgentProcessingError(f"Summarization failed: {str(e)}")

    def _build_system_prompt(self) -> str:
        """Build the system prompt for Claude."""
        return f"""You are an expert at creating concise, professional summaries of podcast content for business audiences.

Your task is to create a summary that:
- Is between {settings.summary_min_words} and {settings.summary_max_words} words
- Captures the main topics and themes discussed
- Highlights key insights and important points
- Is written in a professional, business-appropriate tone
- Focuses on factual content rather than opinions
- Does not include filler words or transcription artifacts

The summary should be useful for someone who wants to quickly understand what the podcast covered without listening to it."""

    def _build_summarization_prompt(self, transcript_content: str) -> str:
        """
        Build the prompt for summarizing the transcript.
        
        Args:
            transcript_content: The transcript to summarize
            
        Returns:
            Formatted prompt string
        """
        # Truncate very long transcripts for the prompt
        max_transcript_length = 15000  # Reasonable limit for Claude context
        if len(transcript_content) > max_transcript_length:
            transcript_content = transcript_content[:max_transcript_length] + "\n[...transcript truncated...]"
            logger.info(f"[{self.agent_name}] Truncated long transcript for processing")

        return f"""Please create a professional summary of the following podcast transcript.

The summary should be {settings.summary_min_words}-{settings.summary_max_words} words and cover:
- Main topics and themes discussed
- Key insights and important points made by speakers
- Notable facts or information shared
- Overall context and purpose of the discussion

Focus on the substantive content and avoid including filler words, "ums", or transcription artifacts.

TRANSCRIPT:
{transcript_content}

SUMMARY:"""

    def _clean_summary(self, raw_summary: str) -> str:
        """
        Clean and format the generated summary.
        
        Args:
            raw_summary: Raw summary text from Claude
            
        Returns:
            Cleaned summary text
        """
        # Remove any leading/trailing whitespace
        summary = raw_summary.strip()
        
        # Remove common prefixes that might be added
        prefixes_to_remove = [
            "Summary:",
            "SUMMARY:",
            "Podcast Summary:",
            "This podcast discusses",
            "In this podcast",
            "The podcast covers"
        ]
        
        for prefix in prefixes_to_remove:
            if summary.startswith(prefix):
                summary = summary[len(prefix):].strip()
                break
        
        # Ensure it starts with a capital letter
        if summary and not summary[0].isupper():
            summary = summary[0].upper() + summary[1:]
        
        # Remove extra whitespace and normalize spacing
        summary = re.sub(r'\s+', ' ', summary)
        
        return summary