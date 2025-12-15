"""
Abstract base class for all AI agents in the podcast analysis system.
Provides common functionality for Claude API interactions and error handling.
"""

import time
from abc import ABC, abstractmethod
from typing import Any, Dict, Optional

import anthropic
from anthropic import Anthropic

from app.config import get_settings
from app.utils.logger import get_logger

logger = get_logger(__name__)
settings = get_settings()


class AgentProcessingError(Exception):
    """Raised when an agent fails to process content."""
    pass


class BaseAgent(ABC):
    """
    Abstract base class for AI agents that process podcast transcripts.
    
    Provides common Claude API interaction patterns, error handling,
    and logging functionality for all specialized agents.
    """

    def __init__(self) -> None:
        """Initialize the agent with Claude API client."""
        if not settings.anthropic_api_key:
            raise ValueError("ANTHROPIC_API_KEY environment variable is required")
        
        self.client: Anthropic = Anthropic(api_key=settings.anthropic_api_key)
        self.model: str = settings.claude_model
        self.agent_name: str = self.__class__.__name__

    @abstractmethod
    def process(self, transcript_content: str, **kwargs) -> Dict[str, Any]:
        """
        Process transcript content and return results.
        
        Args:
            transcript_content: The raw transcript text to analyze
            **kwargs: Additional agent-specific parameters
            
        Returns:
            Dictionary containing the agent's analysis results
            
        Raises:
            AgentProcessingError: If processing fails
        """
        pass

    def _call_claude(self, prompt: str, system_prompt: str = None, tools: list = None, max_retries: int = 3) -> str:
        """
        Make a call to Claude API with error handling and logging.
        
        Args:
            prompt: The user prompt to send to Claude
            system_prompt: Optional system prompt for Claude
            tools: Optional list of tools to enable (e.g., web search)
            
        Returns:
            Claude's text response
            
        Raises:
            AgentProcessingError: If the API call fails
        """
        start_time = time.time()
        retry_count = 0
        
        while retry_count <= max_retries:
            try:
                logger.info(f"[{self.agent_name}] Sending prompt to Claude", 
                           prompt_length=len(prompt), 
                           has_system=bool(system_prompt),
                           tools_enabled=bool(tools))

                # Prepare message parameters
                message_params = {
                    "model": self.model,
                    "max_tokens": 4000,
                    "temperature": 0.1,
                    "messages": [{"role": "user", "content": prompt}]
                }
                
                # Add beta header for web search if tools are provided
                extra_headers = {}
                if tools:
                    extra_headers["anthropic-beta"] = "web-search-2025-03-05"

                # Add system prompt if provided
                if system_prompt:
                    message_params["system"] = system_prompt

                # Add tools if provided (for fact-checking with web search)
                if tools:
                    message_params["tools"] = tools

                # Make the API call
                if extra_headers:
                    response = self.client.messages.create(**message_params, extra_headers=extra_headers)
                else:
                    response = self.client.messages.create(**message_params)
                
                # Extract text from response
                if response.content and len(response.content) > 0:
                    response_text = response.content[0].text
                else:
                    raise AgentProcessingError("Empty response from Claude API")

                # Calculate duration and log success
                duration = time.time() - start_time
                
                logger.info(f"[{self.agent_name}] Claude response received",
                           duration_seconds=round(duration, 2),
                           response_length=len(response_text),
                           input_tokens=response.usage.input_tokens if hasattr(response, 'usage') else 0,
                           output_tokens=response.usage.output_tokens if hasattr(response, 'usage') else 0)

                return response_text

            except anthropic.RateLimitError as e:
                retry_count += 1
                if retry_count <= max_retries:
                    # Exponential backoff: wait 2^retry_count * 10 seconds
                    wait_time = (2 ** retry_count) * 10
                    logger.warning(f"[{self.agent_name}] Rate limit hit, waiting {wait_time}s (attempt {retry_count}/{max_retries})")
                    time.sleep(wait_time)
                    continue
                else:
                    duration = time.time() - start_time
                    logger.error(f"[{self.agent_name}] Rate limit exceeded after {max_retries} retries", 
                                error=str(e), 
                                duration_seconds=round(duration, 2))
                    raise AgentProcessingError(f"Rate limit exceeded: {str(e)}")
                    
            except anthropic.APIError as e:
                duration = time.time() - start_time
                logger.error(f"[{self.agent_name}] Claude API error", 
                            error=str(e), 
                            duration_seconds=round(duration, 2))
                raise AgentProcessingError(f"Claude API error: {str(e)}")

            except Exception as e:
                duration = time.time() - start_time
                logger.error(f"[{self.agent_name}] Unexpected error calling Claude",
                            error=str(e),
                            duration_seconds=round(duration, 2))
                raise AgentProcessingError(f"Unexpected error: {str(e)}")

    def _truncate_for_log(self, text: str, max_length: int = 200) -> str:
        """
        Truncate text for logging to avoid overly long log messages.
        
        Args:
            text: Text to truncate
            max_length: Maximum length to keep
            
        Returns:
            Truncated text with ellipsis if needed
        """
        if len(text) <= max_length:
            return text
        return text[:max_length] + "..."