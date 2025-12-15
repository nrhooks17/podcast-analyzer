/**
 * Home page with file upload functionality.
 */

import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import FileUpload from '../components/upload/FileUpload';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { startAnalysis } from '../services/api';
import ErrorMessage from '../components/common/ErrorMessage';

const HomePage = () => {
  const navigate = useNavigate();
  const [uploadedTranscript, setUploadedTranscript] = useState(null);
  const [isStartingAnalysis, setIsStartingAnalysis] = useState(false);
  const [analysisError, setAnalysisError] = useState(null);

  const handleUploadSuccess = (transcript) => {
    setUploadedTranscript(transcript);
    setAnalysisError(null);
  };

  const handleUploadError = (error) => {
    console.error('Upload error:', error);
    setUploadedTranscript(null);
  };

  const handleStartAnalysis = async () => {
    if (!uploadedTranscript) return;

    setIsStartingAnalysis(true);
    setAnalysisError(null);

    try {
      const analysisJob = await startAnalysis(uploadedTranscript.transcript_id);
      
      // Navigate to analysis page with job ID
      navigate(`/analysis?jobId=${analysisJob.job_id}`);
      
    } catch (error) {
      console.error('Failed to start analysis:', error);
      setAnalysisError(error);
    } finally {
      setIsStartingAnalysis(false);
    }
  };

  const handleUploadAnother = () => {
    setUploadedTranscript(null);
    setAnalysisError(null);
  };

  return (
    <div className="home-page">
      <div className="intro-section text-section">
        <Card>
          <h1>Podcast Analyzer</h1>
          <p>
            Upload your podcast transcript and get AI-powered analysis including summaries, 
            key takeaways, and fact-checking. Perfect for ad agencies and content creators 
            who need quick insights from podcast content.
          </p>
          
          <div className="features-list text-section">
            <h3>What you'll get:</h3>
            <ul>
              <li><strong>Summary:</strong> 200-300 word professional summary of the podcast</li>
              <li><strong>Key Takeaways:</strong> Important insights and actionable points</li>
              <li><strong>Fact Checking:</strong> Verification of claims made during the discussion</li>
            </ul>
          </div>
        </Card>
      </div>

      <div className="upload-container">
        {!uploadedTranscript ? (
          <FileUpload
            onUploadSuccess={handleUploadSuccess}
            onUploadError={handleUploadError}
          />
        ) : (
          <Card title="File Uploaded Successfully">
            <div className="upload-success">
              <div className="file-info">
                <h4>{uploadedTranscript.filename}</h4>
                <p>Word count: {uploadedTranscript.word_count.toLocaleString()}</p>
                <p>Transcript ID: {uploadedTranscript.transcript_id}</p>
              </div>

              <div className="text-section">
                <h4>Ready to analyze</h4>
                <p>
                  Your transcript has been uploaded successfully. Click "Start Analysis" 
                  to begin AI processing. This typically takes 2-5 minutes depending 
                  on transcript length.
                </p>

                <div className="button-group">
                  <Button
                    onClick={handleStartAnalysis}
                    loading={isStartingAnalysis}
                    variant="primary"
                    size="large"
                  >
                    {isStartingAnalysis ? 'Starting Analysis...' : 'Start Analysis'}
                  </Button>

                  <Button
                    onClick={handleUploadAnother}
                    variant="secondary"
                    disabled={isStartingAnalysis}
                  >
                    Upload Another File
                  </Button>
                </div>
              </div>

              {analysisError && (
                <ErrorMessage
                  error={analysisError}
                  onRetry={handleStartAnalysis}
                />
              )}
            </div>
          </Card>
        )}
      </div>

      <div className="supported-formats">
        <Card title="Supported Formats">
          <div className="format-info text-section">
            <div className="format-item text-section">
              <h4>.txt files</h4>
              <p>
                Plain text transcripts with optional timestamps and speaker labels:
              </p>
              <pre>{`[00:00:00] Host: Welcome to the show...
[00:00:15] Guest: Thanks for having me...`}</pre>
            </div>
            
            <div className="format-item text-section">
              <h4>.json files</h4>
              <p>
                Structured JSON format with metadata:
              </p>
              <pre>{`{
  "title": "Episode Title",
  "transcript": [
    {"timestamp": "00:00:00", "speaker": "Host", "text": "..."},
    {"timestamp": "00:15", "speaker": "Guest", "text": "..."}
  ]
}`}</pre>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
};

export default HomePage;