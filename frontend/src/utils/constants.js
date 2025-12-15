/**
 * Application constants and configuration values.
 */

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8000';

export const STATUS_LABELS = {
  pending: 'Pending',
  processing: 'Processing',
  completed: 'Completed',
  failed: 'Failed'
};

export const VERDICT_LABELS = {
  true: 'True',
  false: 'False',
  partially_true: 'Partially True',
  unverifiable: 'Unverifiable'
};

export const POLLING_INTERVAL = 3000; // 3 seconds

export const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB

export const ALLOWED_EXTENSIONS = ['.txt', '.json'];

export const ERROR_CODES = {
  FILE_VALIDATION_ERROR: 'FILE_VALIDATION_ERROR',
  DUPLICATE_TRANSCRIPT: 'DUPLICATE_TRANSCRIPT',
  TRANSCRIPT_NOT_FOUND: 'TRANSCRIPT_NOT_FOUND',
  ANALYSIS_NOT_FOUND: 'ANALYSIS_NOT_FOUND',
  JOB_NOT_FOUND: 'JOB_NOT_FOUND',
  INTERNAL_ERROR: 'INTERNAL_ERROR',
  NETWORK_ERROR: 'NETWORK_ERROR'
};