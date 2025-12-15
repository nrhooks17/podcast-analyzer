/**
 * API client for communicating with the backend.
 * Handles all HTTP requests with error handling and logging.
 */

import axios from 'axios';
import { API_BASE_URL, ERROR_CODES } from '../utils/constants';

// Create axios instance with default config
const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000, // 30 seconds
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor
api.interceptors.request.use(
  (config) => {
    console.log('API Request:', config.method?.toUpperCase(), config.url);
    return config;
  },
  (error) => {
    console.error('API Request Error:', error);
    return Promise.reject(error);
  }
);

// Response interceptor
api.interceptors.response.use(
  (response) => {
    console.log('API Response:', response.status, response.config.url);
    return response;
  },
  (error) => {
    console.error('API Response Error:', error.response?.status, error.response?.data);
    
    // Transform error to standard format
    const transformedError = transformError(error);
    return Promise.reject(transformedError);
  }
);

/**
 * Transform axios error to application error format
 * @param {Object} error - Axios error object
 * @returns {Object} Transformed error
 */
const transformError = (error) => {
  if (!error.response) {
    // Network error
    return {
      code: ERROR_CODES.NETWORK_ERROR,
      message: 'Unable to connect to the server. Please check your internet connection.',
      details: error.message
    };
  }

  const { status, data } = error.response;
  
  // Server returned error response
  if (data && data.error) {
    return {
      code: data.error.code || ERROR_CODES.INTERNAL_ERROR,
      message: data.error.message || 'An unexpected error occurred',
      correlationId: data.error.correlation_id,
      details: data.error.details
    };
  }

  // Default error handling based on status code
  const statusMessages = {
    400: 'Invalid request. Please check your input.',
    401: 'Authentication required.',
    403: 'Access denied.',
    404: 'Resource not found.',
    409: 'Analysis not yet completed.',
    422: 'Invalid data provided.',
    500: 'Internal server error. Please try again later.',
    503: 'Service temporarily unavailable.'
  };

  return {
    code: ERROR_CODES.INTERNAL_ERROR,
    message: statusMessages[status] || 'An unexpected error occurred',
    status
  };
};

/**
 * Upload a transcript file
 * @param {File} file - File to upload
 * @param {Function} onProgress - Progress callback
 * @returns {Promise} Upload response
 */
export const uploadTranscript = async (file, onProgress = null) => {
  const formData = new FormData();
  formData.append('file', file);

  const config = {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  };

  if (onProgress) {
    config.onUploadProgress = (progressEvent) => {
      const percentCompleted = Math.round(
        (progressEvent.loaded * 100) / progressEvent.total
      );
      onProgress(percentCompleted);
    };
  }

  const response = await api.post('/api/transcripts/', formData, config);
  return response.data;
};

/**
 * Get list of transcripts
 * @param {number} page - Page number
 * @param {number} perPage - Items per page
 * @returns {Promise} Transcripts list response
 */
export const getTranscripts = async (page = 1, perPage = 20) => {
  const response = await api.get('/api/transcripts/', {
    params: { page, per_page: perPage }
  });
  return response.data;
};

/**
 * Get transcript by ID
 * @param {string} transcriptId - Transcript ID
 * @returns {Promise} Transcript data
 */
export const getTranscript = async (transcriptId) => {
  const response = await api.get(`/api/transcripts/${transcriptId}`);
  return response.data;
};

/**
 * Delete transcript
 * @param {string} transcriptId - Transcript ID
 * @returns {Promise} Delete response
 */
export const deleteTranscript = async (transcriptId) => {
  const response = await api.delete(`/api/transcripts/${transcriptId}`);
  return response.data;
};

/**
 * Start analysis for a transcript
 * @param {string} transcriptId - Transcript ID
 * @returns {Promise} Analysis job response
 */
export const startAnalysis = async (transcriptId) => {
  const response = await api.post(`/api/analyze/${transcriptId}`);
  return response.data;
};

/**
 * Get job status
 * @param {string} jobId - Job ID
 * @returns {Promise} Job status response
 */
export const getJobStatus = async (jobId) => {
  const response = await api.get(`/api/jobs/${jobId}/status`);
  return response.data;
};

/**
 * Get analysis results
 * @param {string} analysisId - Analysis ID
 * @returns {Promise} Analysis results
 */
export const getAnalysisResults = async (analysisId) => {
  const response = await api.get(`/api/results/${analysisId}`);
  return response.data;
};

/**
 * Get list of analysis results
 * @param {number} page - Page number
 * @param {number} perPage - Items per page
 * @returns {Promise} Analysis results list
 */
export const getAnalysisResultsList = async (page = 1, perPage = 20) => {
  const response = await api.get('/api/results', {
    params: { page, per_page: perPage }
  });
  return response.data;
};

/**
 * Health check
 * @returns {Promise} Health status
 */
export const healthCheck = async () => {
  const response = await api.get('/health');
  return response.data;
};

export default api;