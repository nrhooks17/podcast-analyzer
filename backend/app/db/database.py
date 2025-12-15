"""
Database configuration and session management for PostgreSQL.
"""

from sqlalchemy import create_engine
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, Session
from typing import Generator

from app.config import get_settings
from app.utils.logger import get_logger

logger = get_logger(__name__)
settings = get_settings()

# Create SQLAlchemy engine
engine = create_engine(
    settings.database_url,
    echo=False,  # Set to True for SQL query logging in development
    pool_pre_ping=True,  # Verify connections before use
    pool_recycle=300,  # Recycle connections every 5 minutes
)

# Create session factory
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

# Base class for ORM models
Base = declarative_base()


def get_db() -> Generator[Session, None, None]:
    """
    Dependency to get database session.
    
    Yields:
        Database session that is automatically closed after use
    """
    db = SessionLocal()
    try:
        yield db
    except Exception as e:
        logger.error("Database session error", error=str(e))
        db.rollback()
        raise
    finally:
        db.close()


def init_db() -> None:
    """
    Initialize the database by creating all tables.
    This should be called during application startup.
    """
    try:
        logger.info("Initializing database tables")
        Base.metadata.create_all(bind=engine)
        logger.info("Database tables initialized successfully")
    except Exception as e:
        logger.error("Failed to initialize database", error=str(e))
        raise