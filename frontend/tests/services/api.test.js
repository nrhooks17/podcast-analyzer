/**
 * Tests for API service functions.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import axios from 'axios';
import {
  uploadTranscript,
  getTranscripts,
  getTranscript,
  deleteTranscript,
  startAnalysis,
  getJobStatus,
  getAnalysisResults
} from '../../src/services/api';

// Mock axios
vi.mock('axios');
const mockedAxios = vi.mocked(axios, true);

describe('API Service', () => {
  beforeEach(() => {
    mockedAxios.create.mockReturnValue(mockedAxios);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('uploadTranscript', () => {
    it('uploads file successfully', async () => {
      const file = new File(['test'], 'test.txt', { type: 'text/plain' });
      const responseData = {
        transcript_id: 'abc123',
        filename: 'test.txt',
        word_count: 100,
        message: 'Upload successful'
      };

      mockedAxios.post.mockResolvedValue({ data: responseData });

      const result = await uploadTranscript(file);

      expect(mockedAxios.post).toHaveBeenCalledWith(
        '/api/transcripts/',
        expect.any(FormData),
        expect.objectContaining({
          headers: {
            'Content-Type': 'multipart/form-data'
          }
        })
      );
      expect(result).toEqual(responseData);
    });

    it('calls progress callback during upload', async () => {
      const file = new File(['test'], 'test.txt', { type: 'text/plain' });
      const progressCallback = vi.fn();
      const responseData = { transcript_id: 'abc123' };

      mockedAxios.post.mockImplementation((url, data, config) => {
        // Simulate progress
        if (config.onUploadProgress) {
          config.onUploadProgress({ loaded: 50, total: 100 });
        }
        return Promise.resolve({ data: responseData });
      });

      await uploadTranscript(file, progressCallback);

      expect(progressCallback).toHaveBeenCalledWith(50);
    });
  });

  describe('getTranscripts', () => {
    it('fetches transcripts list', async () => {
      const responseData = {
        transcripts: [
          { id: 'abc123', filename: 'test.txt' },
          { id: 'def456', filename: 'test2.txt' }
        ],
        total: 2,
        page: 1,
        per_page: 20
      };

      mockedAxios.get.mockResolvedValue({ data: responseData });

      const result = await getTranscripts(1, 20);

      expect(mockedAxios.get).toHaveBeenCalledWith('/api/transcripts/', {
        params: { page: 1, per_page: 20 }
      });
      expect(result).toEqual(responseData);
    });

    it('uses default pagination parameters', async () => {
      const responseData = { transcripts: [], total: 0, page: 1, per_page: 20 };
      mockedAxios.get.mockResolvedValue({ data: responseData });

      await getTranscripts();

      expect(mockedAxios.get).toHaveBeenCalledWith('/api/transcripts/', {
        params: { page: 1, per_page: 20 }
      });
    });
  });

  describe('getTranscript', () => {
    it('fetches single transcript', async () => {
      const transcriptId = 'abc123';
      const responseData = {
        id: transcriptId,
        filename: 'test.txt',
        word_count: 100
      };

      mockedAxios.get.mockResolvedValue({ data: responseData });

      const result = await getTranscript(transcriptId);

      expect(mockedAxios.get).toHaveBeenCalledWith(`/api/transcripts/${transcriptId}`);
      expect(result).toEqual(responseData);
    });
  });

  describe('deleteTranscript', () => {
    it('deletes transcript', async () => {
      const transcriptId = 'abc123';
      const responseData = { message: 'Transcript deleted successfully' };

      mockedAxios.delete.mockResolvedValue({ data: responseData });

      const result = await deleteTranscript(transcriptId);

      expect(mockedAxios.delete).toHaveBeenCalledWith(`/api/transcripts/${transcriptId}`);
      expect(result).toEqual(responseData);
    });
  });

  describe('startAnalysis', () => {
    it('starts analysis job', async () => {
      const transcriptId = 'abc123';
      const responseData = {
        job_id: 'job456',
        transcript_id: transcriptId,
        status: 'pending',
        message: 'Analysis job created'
      };

      mockedAxios.post.mockResolvedValue({ data: responseData });

      const result = await startAnalysis(transcriptId);

      expect(mockedAxios.post).toHaveBeenCalledWith(`/api/analyze/${transcriptId}`);
      expect(result).toEqual(responseData);
    });
  });

  describe('getJobStatus', () => {
    it('fetches job status', async () => {
      const jobId = 'job456';
      const responseData = {
        job_id: jobId,
        status: 'processing',
        created_at: '2024-01-01T10:00:00Z'
      };

      mockedAxios.get.mockResolvedValue({ data: responseData });

      const result = await getJobStatus(jobId);

      expect(mockedAxios.get).toHaveBeenCalledWith(`/api/jobs/${jobId}/status`);
      expect(result).toEqual(responseData);
    });
  });

  describe('getAnalysisResults', () => {
    it('fetches analysis results', async () => {
      const analysisId = 'analysis789';
      const responseData = {
        id: analysisId,
        summary: 'Test summary',
        takeaways: ['Takeaway 1', 'Takeaway 2'],
        fact_checks: []
      };

      mockedAxios.get.mockResolvedValue({ data: responseData });

      const result = await getAnalysisResults(analysisId);

      expect(mockedAxios.get).toHaveBeenCalledWith(`/api/results/${analysisId}`);
      expect(result).toEqual(responseData);
    });
  });

  describe('Error handling', () => {
    it('transforms network errors', async () => {
      const networkError = new Error('Network Error');
      networkError.code = 'NETWORK_ERROR';
      mockedAxios.get.mockRejectedValue(networkError);

      await expect(getTranscripts()).rejects.toMatchObject({
        code: 'NETWORK_ERROR',
        message: expect.stringContaining('Unable to connect')
      });
    });

    it('transforms server errors with error response', async () => {
      const serverError = {
        response: {
          status: 400,
          data: {
            error: {
              code: 'FILE_VALIDATION_ERROR',
              message: 'Invalid file format',
              correlation_id: 'abc123'
            }
          }
        }
      };
      mockedAxios.post.mockRejectedValue(serverError);

      const file = new File(['test'], 'test.txt');
      await expect(uploadTranscript(file)).rejects.toMatchObject({
        code: 'FILE_VALIDATION_ERROR',
        message: 'Invalid file format',
        correlationId: 'abc123'
      });
    });

    it('transforms server errors without error details', async () => {
      const serverError = {
        response: {
          status: 500,
          data: {}
        }
      };
      mockedAxios.get.mockRejectedValue(serverError);

      await expect(getTranscripts()).rejects.toMatchObject({
        code: 'INTERNAL_ERROR',
        message: expect.stringContaining('Internal server error')
      });
    });
  });
});