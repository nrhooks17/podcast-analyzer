/**
 * Tests for AnalysisResults component.
 */

import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import AnalysisResults from '../../src/components/results/AnalysisResults';

// Mock the hooks
const mockUseJobStatus = vi.fn();
const mockUseAnalysisResults = vi.fn();

vi.mock('../../src/hooks/useJobStatus', () => ({
  useJobStatus: mockUseJobStatus
}));

vi.mock('../../src/hooks/useAnalysisResults', () => ({
  useAnalysisResults: mockUseAnalysisResults
}));

// Mock markdown exporter
const mockGenerateMarkdown = vi.fn();
const mockDownloadMarkdown = vi.fn();
const mockGenerateFilename = vi.fn();

vi.mock('../../src/utils/markdownExporter', () => ({
  generateMarkdown: mockGenerateMarkdown,
  downloadMarkdown: mockDownloadMarkdown,
  generateFilename: mockGenerateFilename
}));

// Mock child components
vi.mock('../../src/components/results/Summary', () => ({
  default: ({ summary }) => <div data-testid="summary">{summary}</div>
}));

vi.mock('../../src/components/results/KeyTakeaways', () => ({
  default: ({ takeaways }) => (
    <div data-testid="takeaways">
      {takeaways?.map((takeaway, idx) => <div key={idx}>{takeaway}</div>)}
    </div>
  )
}));

vi.mock('../../src/components/results/FactCheckTable', () => ({
  default: ({ factChecks }) => (
    <div data-testid="fact-checks">
      {factChecks?.length || 0} fact checks
    </div>
  )
}));

describe('AnalysisResults', () => {
  const mockResults = {
    id: 'test-analysis-id',
    status: 'completed',
    completed_at: '2024-01-01T12:00:00Z',
    summary: 'This is a test summary of the analysis results.',
    takeaways: [
      'First key takeaway',
      'Second key takeaway'
    ],
    fact_checks: [
      {
        claim: 'Test claim',
        verdict: 'true',
        confidence: 0.9,
        evidence: 'Strong evidence',
        sources: ['https://example.com']
      }
    ]
  };

  beforeEach(() => {
    vi.clearAllMocks();
    
    // Default mock implementations
    mockUseJobStatus.mockReturnValue({
      status: null,
      isPolling: false,
      error: null,
      refresh: vi.fn()
    });
    
    mockUseAnalysisResults.mockReturnValue({
      results: null,
      fetchResults: vi.fn(),
      loading: false,
      error: null
    });
  });

  describe('with completed analysis results', () => {
    beforeEach(() => {
      mockUseAnalysisResults.mockReturnValue({
        results: mockResults,
        fetchResults: vi.fn(),
        loading: false,
        error: null
      });
    });

    it('renders complete analysis results', () => {
      render(<AnalysisResults analysisId="test-id" />);
      
      expect(screen.getByText('Analysis Complete')).toBeInTheDocument();
      expect(screen.getByText('test-analysis-id')).toBeInTheDocument();
      expect(screen.getByText('completed')).toBeInTheDocument();
      expect(screen.getByTestId('summary')).toBeInTheDocument();
      expect(screen.getByTestId('takeaways')).toBeInTheDocument();
      expect(screen.getByTestId('fact-checks')).toBeInTheDocument();
    });

    it('displays download markdown button', () => {
      render(<AnalysisResults analysisId="test-id" />);
      
      const downloadButton = screen.getByRole('button', { name: /download markdown/i });
      expect(downloadButton).toBeInTheDocument();
      expect(downloadButton).toHaveAttribute('title', 'Download results as a Markdown file');
    });

    it('handles markdown download when button is clicked', async () => {
      const mockMarkdown = '# Test Markdown Content';
      const mockFilename = 'analysis-test-2024-01-01.md';
      
      mockGenerateMarkdown.mockReturnValue(mockMarkdown);
      mockGenerateFilename.mockReturnValue(mockFilename);
      
      render(<AnalysisResults analysisId="test-id" />);
      
      const downloadButton = screen.getByRole('button', { name: /download markdown/i });
      fireEvent.click(downloadButton);
      
      await waitFor(() => {
        expect(mockGenerateMarkdown).toHaveBeenCalledWith(mockResults);
        expect(mockGenerateFilename).toHaveBeenCalledWith(mockResults);
        expect(mockDownloadMarkdown).toHaveBeenCalledWith(mockMarkdown, mockFilename);
      });
    });

    it('does not attempt download when no results available', () => {
      mockUseAnalysisResults.mockReturnValue({
        results: null,
        fetchResults: vi.fn(),
        loading: false,
        error: null
      });
      
      render(<AnalysisResults analysisId="test-id" />);
      
      // Should not render download button when no results
      expect(screen.queryByRole('button', { name: /download markdown/i })).not.toBeInTheDocument();
    });

    it('renders analysis info in correct layout', () => {
      render(<AnalysisResults analysisId="test-id" />);
      
      // Check for the new layout structure
      const analysisInfo = screen.getByText('test-analysis-id').closest('.analysis-info');
      expect(analysisInfo).toBeInTheDocument();
      
      // Should contain both info section and actions
      expect(analysisInfo?.querySelector('.info-section')).toBeInTheDocument();
      expect(analysisInfo?.querySelector('.analysis-actions')).toBeInTheDocument();
    });

    it('displays completion date correctly', () => {
      render(<AnalysisResults analysisId="test-id" />);
      
      // Should format the date
      expect(screen.getByText(/1\/1\/2024/)).toBeInTheDocument();
    });
  });

  describe('with processing job status', () => {
    beforeEach(() => {
      mockUseJobStatus.mockReturnValue({
        status: {
          job_id: 'test-job-id',
          status: 'processing',
          created_at: '2024-01-01T11:00:00Z'
        },
        isPolling: true,
        error: null,
        refresh: vi.fn()
      });
    });

    it('shows processing status', () => {
      render(<AnalysisResults jobId="test-job-id" />);
      
      expect(screen.getByText('Analysis Status')).toBeInTheDocument();
      expect(screen.getByText('processing')).toBeInTheDocument();
      expect(screen.getByText('Analysis in progress...')).toBeInTheDocument();
    });

    it('does not show download button during processing', () => {
      render(<AnalysisResults jobId="test-job-id" />);
      
      expect(screen.queryByRole('button', { name: /download markdown/i })).not.toBeInTheDocument();
    });
  });

  describe('with failed job status', () => {
    beforeEach(() => {
      mockUseJobStatus.mockReturnValue({
        status: {
          job_id: 'test-job-id',
          status: 'failed',
          error_message: 'Analysis failed due to an error',
          created_at: '2024-01-01T11:00:00Z'
        },
        isPolling: false,
        error: null,
        refresh: vi.fn()
      });
    });

    it('shows failed status with error message', () => {
      render(<AnalysisResults jobId="test-job-id" />);
      
      expect(screen.getByText('Analysis Status')).toBeInTheDocument();
      expect(screen.getByText('failed')).toBeInTheDocument();
      expect(screen.getByText('Analysis failed due to an error')).toBeInTheDocument();
    });

    it('does not show download button for failed analysis', () => {
      render(<AnalysisResults jobId="test-job-id" />);
      
      expect(screen.queryByRole('button', { name: /download markdown/i })).not.toBeInTheDocument();
    });
  });

  describe('with loading state', () => {
    beforeEach(() => {
      mockUseAnalysisResults.mockReturnValue({
        results: null,
        fetchResults: vi.fn(),
        loading: true,
        error: null
      });
    });

    it('shows loading spinner', () => {
      render(<AnalysisResults analysisId="test-id" />);
      
      expect(screen.getByText('Loading analysis results...')).toBeInTheDocument();
    });

    it('does not show download button while loading', () => {
      render(<AnalysisResults analysisId="test-id" />);
      
      expect(screen.queryByRole('button', { name: /download markdown/i })).not.toBeInTheDocument();
    });
  });

  describe('with error state', () => {
    beforeEach(() => {
      mockUseAnalysisResults.mockReturnValue({
        results: null,
        fetchResults: vi.fn(),
        loading: false,
        error: {
          message: 'Failed to load results',
          code: 'ANALYSIS_ERROR'
        }
      });
    });

    it('shows error message', () => {
      render(<AnalysisResults analysisId="test-id" />);
      
      expect(screen.getByText('Failed to load results')).toBeInTheDocument();
    });

    it('does not show download button when error occurs', () => {
      render(<AnalysisResults analysisId="test-id" />);
      
      expect(screen.queryByRole('button', { name: /download markdown/i })).not.toBeInTheDocument();
    });
  });

  describe('fetching results behavior', () => {
    it('fetches results when analysisId is provided', () => {
      const mockFetchResults = vi.fn();
      mockUseAnalysisResults.mockReturnValue({
        results: null,
        fetchResults: mockFetchResults,
        loading: false,
        error: null
      });
      
      render(<AnalysisResults analysisId="test-analysis-id" />);
      
      expect(mockFetchResults).toHaveBeenCalledWith('test-analysis-id');
    });

    it('fetches results when job status becomes completed', () => {
      const mockFetchResults = vi.fn();
      mockUseAnalysisResults.mockReturnValue({
        results: null,
        fetchResults: mockFetchResults,
        loading: false,
        error: null
      });
      
      mockUseJobStatus.mockReturnValue({
        status: {
          id: 'completed-job-id',
          status: 'completed'
        },
        isPolling: false,
        error: null,
        refresh: vi.fn()
      });
      
      render(<AnalysisResults jobId="test-job-id" />);
      
      expect(mockFetchResults).toHaveBeenCalledWith('completed-job-id');
    });
  });
});