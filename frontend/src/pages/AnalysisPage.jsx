/**
 * Analysis page showing job status and results.
 */

import React, { useEffect } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import AnalysisResults from '../components/results/AnalysisResults';
import Button from '../components/common/Button';
import ErrorMessage from '../components/common/ErrorMessage';

const AnalysisPage = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  
  const jobId = searchParams.get('jobId');
  const analysisId = searchParams.get('analysisId');

  useEffect(() => {
    if (!jobId && !analysisId) {
      // Redirect to home if no job or analysis ID provided
      navigate('/', { replace: true });
    }
  }, [jobId, analysisId, navigate]);

  if (!jobId && !analysisId) {
    return (
      <ErrorMessage
        error={{
          message: 'No analysis job or results specified',
          code: 'MISSING_PARAMETERS'
        }}
        onRetry={() => navigate('/')}
      />
    );
  }

  return (
    <div className="analysis-page">
      <div className="page-header">
        <div className="header-content">
          <h1>Analysis Results</h1>
          <div className="header-actions">
            <Button
              onClick={() => navigate('/')}
              variant="secondary"
            >
              New Analysis
            </Button>
            
            <Button
              onClick={() => navigate('/history')}
              variant="secondary"
            >
              View History
            </Button>
          </div>
        </div>
        
        {jobId && (
          <div className="job-info">
            <p>Job ID: <code>{jobId}</code></p>
          </div>
        )}
        
        {analysisId && (
          <div className="analysis-info">
            <p>Analysis ID: <code>{analysisId}</code></p>
          </div>
        )}
      </div>

      <div className="results-container">
        <AnalysisResults 
          jobId={jobId} 
          analysisId={analysisId}
          autoRefresh={true}
        />
      </div>
    </div>
  );
};

export default AnalysisPage;