"""
FastAPI application entry point for Podcast Analyzer.
Configures the application, middleware, routes, and error handlers.
"""

import uuid
from typing import Dict, Union, AsyncGenerator
from fastapi import FastAPI, Request, HTTPException, Response
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from contextlib import asynccontextmanager

from app.config import get_settings
from app.db.database import init_db
from app.routers import transcripts, analysis
from app.utils.logger import setup_logging, get_logger, set_correlation_id
from app.utils.file_handler import FileValidationError
from app.services.transcript_service import TranscriptNotFoundError
from app.services.analysis_service import AnalysisNotFoundError
from app.services.kafka_service import KafkaConnectionError

# Initialize settings and logging
settings = get_settings()
logger = setup_logging(settings.log_level)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Application lifespan manager for startup and shutdown events."""
    # Startup
    logger.info("Starting Podcast Analyzer API", version="1.0.0")
    
    try:
        # Initialize database
        init_db()
        logger.info("Database initialization completed")
    except Exception as e:
        logger.error("Database initialization failed", error=str(e))
        raise
    
    yield
    
    # Shutdown
    logger.info("Shutting down Podcast Analyzer API")


# Create FastAPI application
app = FastAPI(
    title="Podcast Analyzer API",
    description="AI-powered podcast transcript analysis for ad agencies",
    version="1.0.0",
    docs_url="/docs",
    redoc_url="/redoc",
    lifespan=lifespan
)

# Configure CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origins,
    allow_credentials=True,
    allow_methods=["GET", "POST", "PUT", "DELETE"],
    allow_headers=["*"],
)


@app.middleware("http")
async def correlation_id_middleware(request: Request, call_next) -> Response:
    """Add correlation ID to all requests for tracing."""
    correlation_id = set_correlation_id()
    
    # Log request start
    logger.info("Request started",
               method=request.method,
               url=str(request.url),
               client_ip=request.client.host if request.client else "unknown")
    
    response = await call_next(request)
    
    # Add correlation ID to response headers
    response.headers["X-Correlation-ID"] = correlation_id
    
    # Log request completion
    logger.info("Request completed",
               method=request.method,
               url=str(request.url),
               status_code=response.status_code)
    
    return response


# Custom exception handlers
@app.exception_handler(FileValidationError)
async def file_validation_exception_handler(request: Request, exc: FileValidationError) -> JSONResponse:
    """Handle file validation errors."""
    correlation_id = set_correlation_id()
    logger.warning("File validation error", error=str(exc), url=str(request.url))
    
    return JSONResponse(
        status_code=400,
        content={
            "error": {
                "code": "FILE_VALIDATION_ERROR",
                "message": str(exc),
                "correlation_id": correlation_id
            }
        }
    )


@app.exception_handler(TranscriptNotFoundError)
async def transcript_not_found_handler(request: Request, exc: TranscriptNotFoundError) -> JSONResponse:
    """Handle transcript not found errors."""
    correlation_id = set_correlation_id()
    logger.warning("Transcript not found", error=str(exc), url=str(request.url))
    
    return JSONResponse(
        status_code=404,
        content={
            "error": {
                "code": "TRANSCRIPT_NOT_FOUND",
                "message": "The requested transcript does not exist",
                "correlation_id": correlation_id
            }
        }
    )


@app.exception_handler(AnalysisNotFoundError)
async def analysis_not_found_handler(request: Request, exc: AnalysisNotFoundError) -> JSONResponse:
    """Handle analysis not found errors."""
    correlation_id = set_correlation_id()
    logger.warning("Analysis not found", error=str(exc), url=str(request.url))
    
    return JSONResponse(
        status_code=404,
        content={
            "error": {
                "code": "ANALYSIS_NOT_FOUND",
                "message": "The requested analysis does not exist",
                "correlation_id": correlation_id
            }
        }
    )



@app.exception_handler(KafkaConnectionError)
async def kafka_connection_handler(request: Request, exc: KafkaConnectionError) -> JSONResponse:
    """Handle Kafka connection errors."""
    correlation_id = set_correlation_id()
    logger.error("Kafka connection error", error=str(exc), url=str(request.url))
    
    return JSONResponse(
        status_code=503,
        content={
            "error": {
                "code": "SERVICE_UNAVAILABLE",
                "message": "Analysis service temporarily unavailable",
                "correlation_id": correlation_id
            }
        }
    )


@app.exception_handler(Exception)
async def general_exception_handler(request: Request, exc: Exception) -> JSONResponse:
    """Handle all other unhandled exceptions."""
    correlation_id = set_correlation_id()
    logger.error("Unhandled exception", 
                error=str(exc), 
                error_type=type(exc).__name__,
                url=str(request.url))
    
    return JSONResponse(
        status_code=500,
        content={
            "error": {
                "code": "INTERNAL_ERROR",
                "message": "An unexpected error occurred",
                "correlation_id": correlation_id
            }
        }
    )


# Include routers
app.include_router(transcripts.router)
app.include_router(analysis.router)


@app.get("/health")
async def health_check() -> Dict[str, str]:
    """Health check endpoint for monitoring."""
    return {
        "status": "healthy",
        "service": "podcast-analyzer-api",
        "version": "1.0.0"
    }


@app.get("/")
async def root() -> Dict[str, str]:
    """Root endpoint with API information."""
    return {
        "service": "Podcast Analyzer API",
        "version": "1.0.0",
        "description": "AI-powered podcast transcript analysis for ad agencies",
        "docs": "/docs",
        "health": "/health"
    }


if __name__ == "__main__":
    import uvicorn
    
    # Run the application
    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=8000,
        reload=True,
        log_level=settings.log_level.lower()
    )