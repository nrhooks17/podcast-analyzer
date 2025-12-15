/**
 * Custom hook for handling file uploads with progress tracking and validation.
 */

import { useState } from 'react';
import { uploadTranscript } from '../services/api';
import { MAX_FILE_SIZE, ALLOWED_EXTENSIONS } from '../utils/constants';

export const useFileUpload = () => {
  const [isUploading, setIsUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploadError, setUploadError] = useState(null);
  const [uploadSuccess, setUploadSuccess] = useState(null);

  /**
   * Validate file before upload
   * @param {File} file - File to validate
   * @returns {Object} Validation result
   */
  const validateFile = (file) => {
    const errors = [];

    if (!file) {
      errors.push('Please select a file to upload');
      return { valid: false, errors };
    }

    // Check file size
    if (file.size > MAX_FILE_SIZE) {
      const maxMB = MAX_FILE_SIZE / (1024 * 1024);
      errors.push(`File size must be less than ${maxMB}MB`);
    }

    // Check file extension
    const fileExtension = '.' + file.name.split('.').pop().toLowerCase();
    if (!ALLOWED_EXTENSIONS.includes(fileExtension)) {
      errors.push(`File type not supported. Please upload ${ALLOWED_EXTENSIONS.join(' or ')} files`);
    }

    return {
      valid: errors.length === 0,
      errors
    };
  };

  /**
   * Upload file with progress tracking
   * @param {File} file - File to upload
   * @returns {Promise} Upload result
   */
  const upload = async (file) => {
    // Reset states
    setUploadError(null);
    setUploadSuccess(null);
    setUploadProgress(0);

    // Validate file
    const validation = validateFile(file);
    if (!validation.valid) {
      const error = {
        code: 'VALIDATION_ERROR',
        message: validation.errors.join('. ')
      };
      setUploadError(error);
      return { success: false, error };
    }

    setIsUploading(true);

    try {
      const result = await uploadTranscript(file, (progress) => {
        setUploadProgress(progress);
      });

      setUploadSuccess({
        message: result.message,
        transcript: result
      });

      return { success: true, data: result };

    } catch (error) {
      console.error('Upload failed:', error);
      setUploadError(error);
      return { success: false, error };

    } finally {
      setIsUploading(false);
      // Keep progress at 100% briefly if successful
      setTimeout(() => {
        setUploadProgress(0);
      }, 2000);
    }
  };

  /**
   * Reset upload state
   */
  const resetUpload = () => {
    setIsUploading(false);
    setUploadProgress(0);
    setUploadError(null);
    setUploadSuccess(null);
  };

  return {
    isUploading,
    uploadProgress,
    uploadError,
    uploadSuccess,
    upload,
    validateFile,
    resetUpload
  };
};