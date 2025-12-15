/**
 * File upload component with drag-and-drop support.
 */

import React, { useState, useRef } from 'react';
import PropTypes from 'prop-types';
import { useFileUpload } from '../../hooks/useFileUpload';
import ProgressBar from '../common/ProgressBar';
import ErrorMessage from '../common/ErrorMessage';
import Button from '../common/Button';

const FileUpload = ({ onUploadSuccess, onUploadError }) => {
  const [dragOver, setDragOver] = useState(false);
  const [selectedFile, setSelectedFile] = useState(null);
  const fileInputRef = useRef(null);

  const {
    isUploading,
    uploadProgress,
    uploadError,
    uploadSuccess,
    upload,
    resetUpload
  } = useFileUpload();

  const handleDragOver = (e) => {
    e.preventDefault();
    setDragOver(true);
  };

  const handleDragLeave = (e) => {
    e.preventDefault();
    setDragOver(false);
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setDragOver(false);
    
    const files = e.dataTransfer.files;
    if (files.length > 0) {
      handleFileSelect(files[0]);
    }
  };

  const handleFileInputChange = (e) => {
    const files = e.target.files;
    if (files.length > 0) {
      handleFileSelect(files[0]);
    }
  };

  const handleFileSelect = (file) => {
    setSelectedFile(file);
    resetUpload();
  };

  const handleUpload = async () => {
    if (!selectedFile) return;

    const result = await upload(selectedFile);
    
    if (result.success) {
      if (onUploadSuccess) {
        onUploadSuccess(result.data);
      }
      setSelectedFile(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    } else {
      if (onUploadError) {
        onUploadError(result.error);
      }
    }
  };

  const handleClear = () => {
    setSelectedFile(null);
    resetUpload();
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const openFileDialog = () => {
    if (fileInputRef.current) {
      fileInputRef.current.click();
    }
  };

  return (
    <div className="file-upload">
      {/* Drag and Drop Area */}
      <div
        className={`upload-area ${dragOver ? 'dragover' : ''}`}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={openFileDialog}
      >
        <input
          ref={fileInputRef}
          type="file"
          accept=".txt,.json"
          onChange={handleFileInputChange}
          disabled={isUploading}
        />
        
        {!selectedFile ? (
          <>
            <h3>Upload Podcast Transcript</h3>
            <p>Drag and drop your transcript file here, or click to select</p>
            <p className="file-info">
              Supports .txt and .json files up to 10MB
            </p>
          </>
        ) : (
          <>
            <h3>File Selected</h3>
            <p><strong>{selectedFile.name}</strong></p>
            <p>Size: {(selectedFile.size / 1024 / 1024).toFixed(2)} MB</p>
          </>
        )}
      </div>

      {/* File Actions */}
      {selectedFile && (
        <div className="file-actions button-group">
          <Button
            onClick={handleUpload}
            loading={isUploading}
            disabled={isUploading}
            variant="primary"
          >
            {isUploading ? 'Uploading...' : 'Upload File'}
          </Button>
          
          <Button
            onClick={handleClear}
            disabled={isUploading}
            variant="secondary"
          >
            Clear
          </Button>
        </div>
      )}

      {/* Upload Progress */}
      {isUploading && (
        <div className="upload-progress">
          <ProgressBar progress={uploadProgress} />
          <p>Uploading: {uploadProgress}%</p>
        </div>
      )}

      {/* Success Message */}
      {uploadSuccess && (
        <div className="success">
          <h4>Upload Successful</h4>
          <p>{uploadSuccess.message}</p>
          <p>
            <strong>Word Count:</strong> {uploadSuccess.transcript.word_count.toLocaleString()}
          </p>
        </div>
      )}

      {/* Error Message */}
      {uploadError && (
        <ErrorMessage
          error={uploadError}
          onRetry={selectedFile ? handleUpload : null}
        />
      )}
    </div>
  );
};

FileUpload.propTypes = {
  onUploadSuccess: PropTypes.func,
  onUploadError: PropTypes.func
};

export default FileUpload;