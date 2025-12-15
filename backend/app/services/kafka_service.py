"""
Service for Kafka message publishing and handling.
Manages job queue communication for analysis processing.
"""

import json
import uuid
from typing import Dict, Any, Optional
from kafka import KafkaProducer
from kafka.errors import KafkaError

from app.config import get_settings
from app.utils.logger import get_logger

logger = get_logger(__name__)
settings = get_settings()


class KafkaConnectionError(Exception):
    """Raised when Kafka connection fails."""
    pass


class KafkaService:
    """
    Service class for Kafka message publishing.
    
    Handles publishing analysis job messages to Kafka topics
    for asynchronous processing by workers.
    """

    def __init__(self) -> None:
        """Initialize Kafka producer."""
        self.producer: Optional[KafkaProducer] = None
        self._initialize_producer()

    def _initialize_producer(self) -> None:
        """
        Initialize Kafka producer with error handling.
        
        Raises:
            KafkaConnectionError: If unable to connect to Kafka
        """
        try:
            self.producer = KafkaProducer(
                bootstrap_servers=settings.kafka_bootstrap_servers.split(','),
                value_serializer=lambda v: json.dumps(v).encode('utf-8'),
                key_serializer=lambda k: str(k).encode('utf-8') if k else None,
                acks='all',  # Wait for all replicas to acknowledge
                retries=3,   # Retry failed sends
                retry_backoff_ms=100
            )
            
            logger.info("Kafka producer initialized", 
                       bootstrap_servers=settings.kafka_bootstrap_servers)

        except Exception as e:
            logger.error("Failed to initialize Kafka producer", error=str(e))
            raise KafkaConnectionError(f"Cannot connect to Kafka: {str(e)}")

    def publish_analysis_job(self, job_id: uuid.UUID, transcript_id: uuid.UUID) -> bool:
        """
        Publish an analysis job message to the Kafka topic.
        
        Args:
            job_id: UUID of the analysis job
            transcript_id: UUID of the transcript to analyze
            
        Returns:
            True if message was published successfully
            
        Raises:
            KafkaConnectionError: If publishing fails
        """
        if not self.producer:
            raise KafkaConnectionError("Kafka producer not initialized")

        message = {
            "job_id": str(job_id),
            "transcript_id": str(transcript_id),
            "timestamp": str(uuid.uuid4())  # For message deduplication
        }

        try:
            # Send message to the analysis topic
            future = self.producer.send(
                topic=settings.kafka_topic_analysis,
                key=str(job_id),
                value=message
            )
            
            # Wait for the message to be sent
            record_metadata = future.get(timeout=10)
            
            logger.info("Analysis job published to Kafka", 
                       job_id=job_id,
                       transcript_id=transcript_id,
                       topic=record_metadata.topic,
                       partition=record_metadata.partition,
                       offset=record_metadata.offset)

            return True

        except KafkaError as e:
            logger.error("Kafka publish error", 
                        job_id=job_id,
                        transcript_id=transcript_id,
                        error=str(e))
            raise KafkaConnectionError(f"Failed to publish message: {str(e)}")

        except Exception as e:
            logger.error("Unexpected error publishing to Kafka", 
                        job_id=job_id,
                        transcript_id=transcript_id,
                        error=str(e))
            raise KafkaConnectionError(f"Unexpected error: {str(e)}")

    def close(self) -> None:
        """Close the Kafka producer connection."""
        if self.producer:
            try:
                self.producer.close(timeout=5)
                logger.info("Kafka producer closed")
            except Exception as e:
                logger.error("Error closing Kafka producer", error=str(e))

    def __enter__(self) -> "KafkaService":
        """Context manager entry."""
        return self

    def __exit__(self, exc_type: Optional[type], exc_val: Optional[BaseException], exc_tb: Optional[object]) -> None:
        """Context manager exit."""
        self.close()