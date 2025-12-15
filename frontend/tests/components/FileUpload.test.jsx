/**
 * Tests for FileUpload component.
 */

import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import FileUpload from '../../src/components/upload/FileUpload';

// Mock the useFileUpload hook
const mockUpload = vi.fn();
const mockResetUpload = vi.fn();

vi.mock('../../src/hooks/useFileUpload', () => ({
  useFileUpload: () => ({
    isUploading: false,
    uploadProgress: 0,
    uploadError: null,
    uploadSuccess: null,
    upload: mockUpload,
    resetUpload: mockResetUpload
  })
}));

describe('FileUpload', () => {
  const mockOnUploadSuccess = vi.fn();
  const mockOnUploadError = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders upload area with correct text', () => {
    render(<FileUpload onUploadSuccess={mockOnUploadSuccess} />);
    
    expect(screen.getByText('Upload Podcast Transcript')).toBeInTheDocument();
    expect(screen.getByText(/Supports .txt and .json files/)).toBeInTheDocument();
  });

  it('handles file selection', async () => {
    const user = userEvent.setup();
    render(<FileUpload onUploadSuccess={mockOnUploadSuccess} />);
    
    const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
    const input = screen.getByRole('button', { hidden: true });
    
    await user.upload(input, file);
    
    expect(screen.getByText('File Selected')).toBeInTheDocument();
    expect(screen.getByText('test.txt')).toBeInTheDocument();
  });

  it('shows upload and clear buttons when file is selected', async () => {
    const user = userEvent.setup();
    render(<FileUpload onUploadSuccess={mockOnUploadSuccess} />);
    
    const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
    const input = screen.getByRole('button', { hidden: true });
    
    await user.upload(input, file);
    
    expect(screen.getByRole('button', { name: /upload file/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /clear/i })).toBeInTheDocument();
  });

  it('calls upload function when upload button is clicked', async () => {
    const user = userEvent.setup();
    mockUpload.mockResolvedValue({ success: true, data: { transcript_id: '123' } });
    
    render(<FileUpload onUploadSuccess={mockOnUploadSuccess} />);
    
    const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
    const input = screen.getByRole('button', { hidden: true });
    
    await user.upload(input, file);
    await user.click(screen.getByRole('button', { name: /upload file/i }));
    
    expect(mockUpload).toHaveBeenCalledWith(file);
  });

  it('clears file when clear button is clicked', async () => {
    const user = userEvent.setup();
    render(<FileUpload onUploadSuccess={mockOnUploadSuccess} />);
    
    const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
    const input = screen.getByRole('button', { hidden: true });
    
    await user.upload(input, file);
    await user.click(screen.getByRole('button', { name: /clear/i }));
    
    expect(screen.queryByText('File Selected')).not.toBeInTheDocument();
    expect(mockResetUpload).toHaveBeenCalled();
  });

  it('handles drag and drop', async () => {
    render(<FileUpload onUploadSuccess={mockOnUploadSuccess} />);
    
    const uploadArea = screen.getByText(/drag and drop/i).closest('.upload-area');
    const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
    
    fireEvent.dragOver(uploadArea);
    fireEvent.drop(uploadArea, {
      dataTransfer: {
        files: [file]
      }
    });
    
    expect(screen.getByText('File Selected')).toBeInTheDocument();
    expect(screen.getByText('test.txt')).toBeInTheDocument();
  });

  it('adds dragover class during drag', () => {
    render(<FileUpload onUploadSuccess={mockOnUploadSuccess} />);
    
    const uploadArea = screen.getByText(/drag and drop/i).closest('.upload-area');
    
    fireEvent.dragOver(uploadArea);
    expect(uploadArea).toHaveClass('dragover');
    
    fireEvent.dragLeave(uploadArea);
    expect(uploadArea).not.toHaveClass('dragover');
  });

  it('calls onUploadSuccess when upload succeeds', async () => {
    const user = userEvent.setup();
    const uploadData = { transcript_id: '123', filename: 'test.txt' };
    mockUpload.mockResolvedValue({ success: true, data: uploadData });
    
    render(<FileUpload onUploadSuccess={mockOnUploadSuccess} />);
    
    const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
    const input = screen.getByRole('button', { hidden: true });
    
    await user.upload(input, file);
    await user.click(screen.getByRole('button', { name: /upload file/i }));
    
    await waitFor(() => {
      expect(mockOnUploadSuccess).toHaveBeenCalledWith(uploadData);
    });
  });

  it('calls onUploadError when upload fails', async () => {
    const user = userEvent.setup();
    const uploadError = { code: 'UPLOAD_FAILED', message: 'Upload failed' };
    mockUpload.mockResolvedValue({ success: false, error: uploadError });
    
    render(<FileUpload onUploadError={mockOnUploadError} />);
    
    const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
    const input = screen.getByRole('button', { hidden: true });
    
    await user.upload(input, file);
    await user.click(screen.getByRole('button', { name: /upload file/i }));
    
    await waitFor(() => {
      expect(mockOnUploadError).toHaveBeenCalledWith(uploadError);
    });
  });
});