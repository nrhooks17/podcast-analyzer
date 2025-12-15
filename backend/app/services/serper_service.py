"""
Serper API web search service for fact verification.
"""

import http.client
import json
import logging
from typing import Dict, List, Optional, Any

from app.config import get_settings

logger = logging.getLogger(__name__)


class SerperSearchError(Exception):
    """Exception raised when Serper search fails."""
    pass


class SerperService:
    """Service for performing web searches using Serper API."""
    
    def __init__(self) -> None:
        """Initialize the Serper service."""
        self.settings = get_settings()
        self.base_url = "google.serper.dev"
        self.search_endpoint = "/search"
    
    def search(self, query: str, num_results: int = 5) -> Dict[str, Any]:
        """
        Perform a web search using Serper API.
        
        Args:
            query: The search query string
            num_results: Number of results to return (default: 5)
            
        Returns:
            Dict containing search results with organic results, snippets, and sources
            
        Raises:
            SerperSearchError: If search fails or API key is missing
        """
        if not self.settings.serper_api_key:
            raise SerperSearchError("Serper API key not configured")
        
        logger.info(f"Performing web search for query: {query}")
        
        try:
            # Prepare the request
            conn = http.client.HTTPSConnection(self.base_url)
            
            payload = json.dumps({
                "q": query,
                "num": num_results
            })
            
            headers = {
                'X-API-KEY': self.settings.serper_api_key,
                'Content-Type': 'application/json'
            }
            
            # Make the request
            conn.request("POST", self.search_endpoint, payload, headers)
            response = conn.getresponse()
            
            if response.status != 200:
                error_msg = f"Serper API returned status {response.status}: {response.reason}"
                logger.error(error_msg)
                raise SerperSearchError(error_msg)
            
            # Parse the response
            data = response.read()
            result = json.loads(data.decode("utf-8"))
            
            logger.info(f"Search completed. Found {len(result.get('organic', []))} organic results")
            return result
            
        except json.JSONDecodeError as e:
            error_msg = f"Failed to parse Serper API response: {e}"
            logger.error(error_msg)
            raise SerperSearchError(error_msg)
        except Exception as e:
            error_msg = f"Serper API request failed: {e}"
            logger.error(error_msg)
            raise SerperSearchError(error_msg)
        finally:
            conn.close()
    
    def extract_search_context(self, search_results: Dict[str, Any]) -> Dict[str, Any]:
        """
        Extract relevant context from Serper search results for fact verification.
        
        Args:
            search_results: Raw search results from Serper API
            
        Returns:
            Dict with formatted search context including snippets and sources
        """
        context = {
            "snippets": [],
            "sources": [],
            "total_results": 0
        }
        
        # Extract organic search results
        organic_results = search_results.get("organic", [])
        context["total_results"] = len(organic_results)
        
        for result in organic_results:
            # Extract snippet/description
            snippet = result.get("snippet", "")
            if snippet:
                context["snippets"].append({
                    "title": result.get("title", ""),
                    "snippet": snippet,
                    "url": result.get("link", "")
                })
                
            # Extract source URL
            link = result.get("link", "")
            if link:
                context["sources"].append(link)
        
        # Extract answer box if available
        answer_box = search_results.get("answerBox", {})
        if answer_box:
            answer_snippet = answer_box.get("answer", "") or answer_box.get("snippet", "")
            if answer_snippet:
                context["snippets"].insert(0, {
                    "title": answer_box.get("title", "Answer Box"),
                    "snippet": answer_snippet,
                    "url": answer_box.get("link", "")
                })
        
        # Extract knowledge graph if available
        knowledge_graph = search_results.get("knowledgeGraph", {})
        if knowledge_graph:
            kg_description = knowledge_graph.get("description", "")
            if kg_description:
                context["snippets"].insert(0, {
                    "title": f"Knowledge Graph: {knowledge_graph.get('title', '')}",
                    "snippet": kg_description,
                    "url": knowledge_graph.get("website", "")
                })
        
        logger.debug(f"Extracted {len(context['snippets'])} snippets and {len(context['sources'])} sources")
        return context
    
    def search_for_claim(self, claim: str) -> Dict[str, Any]:
        """
        Perform a targeted search for a specific factual claim.
        
        Args:
            claim: The factual claim to verify
            
        Returns:
            Dict with search context optimized for fact verification
        """
        try:
            # Clean up the claim for better search results
            search_query = self._optimize_claim_query(claim)
            
            # Perform the search
            search_results = self.search(search_query, num_results=5)
            
            # Extract and format context
            context = self.extract_search_context(search_results)
            
            # Add the original claim and search query for reference
            context["original_claim"] = claim
            context["search_query"] = search_query
            
            return context
            
        except SerperSearchError:
            # Re-raise Serper errors
            raise
        except Exception as e:
            error_msg = f"Failed to search for claim: {e}"
            logger.error(error_msg)
            raise SerperSearchError(error_msg)
    
    def _optimize_claim_query(self, claim: str) -> str:
        """
        Optimize a factual claim for web search.
        
        Args:
            claim: The original claim
            
        Returns:
            Optimized search query
        """
        # Remove common prefixes/suffixes that might hurt search quality
        query = claim.strip()
        
        # Remove quotation marks that might be too restrictive
        query = query.replace('"', '')
        
        # Limit query length for better results
        words = query.split()
        if len(words) > 10:
            query = ' '.join(words[:10])
        
        return query