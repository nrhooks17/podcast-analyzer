"""
Centralized logging configuration for the Podcast Analyzer application.
Provides structured JSON logging with correlation IDs for request tracing.
"""

import logging
import uuid
from contextvars import ContextVar
from typing import Any, Dict

import structlog
from structlog.types import FilteringBoundLogger

# Context variable to store correlation ID across async operations
correlation_id_ctx: ContextVar[str] = ContextVar("correlation_id", default="")


def get_correlation_id() -> str:
    """Get the current correlation ID from context."""
    return correlation_id_ctx.get()


def set_correlation_id(correlation_id: str = None) -> str:
    """Set correlation ID in context. Generates new UUID if none provided."""
    if correlation_id is None:
        correlation_id = str(uuid.uuid4())
    correlation_id_ctx.set(correlation_id)
    return correlation_id


def add_correlation_id(logger: FilteringBoundLogger, method_name: str, event_dict: Dict[str, Any]) -> Dict[str, Any]:
    """Add correlation ID to log events."""
    correlation_id = get_correlation_id()
    if correlation_id:
        event_dict["correlation_id"] = correlation_id
    return event_dict


def setup_logging(log_level: str = "INFO") -> FilteringBoundLogger:
    """
    Configure structured logging for the application.
    
    Args:
        log_level: Logging level (DEBUG, INFO, WARNING, ERROR)
        
    Returns:
        Configured logger instance
    """
    # Configure structlog
    structlog.configure(
        processors=[
            structlog.stdlib.filter_by_level,
            structlog.stdlib.add_logger_name,
            structlog.stdlib.add_log_level,
            structlog.stdlib.PositionalArgumentsFormatter(),
            add_correlation_id,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.StackInfoRenderer(),
            structlog.processors.format_exc_info,
            structlog.processors.UnicodeDecoder(),
            structlog.processors.JSONRenderer(),
        ],
        context_class=dict,
        logger_factory=structlog.stdlib.LoggerFactory(),
        wrapper_class=structlog.stdlib.BoundLogger,
        cache_logger_on_first_use=True,
    )

    # Configure standard library logging
    logging.basicConfig(
        format="%(message)s",
        level=getattr(logging, log_level.upper()),
    )

    return structlog.get_logger()


def get_logger(name: str = None) -> FilteringBoundLogger:
    """Get a logger instance with optional name."""
    if name:
        return structlog.get_logger(name)
    return structlog.get_logger()