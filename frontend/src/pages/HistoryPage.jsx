/**
 * History page showing past analysis results.
 */

import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAnalysisResults } from '../hooks/useAnalysisResults';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import LoadingSpinner from '../components/common/LoadingSpinner';
import ErrorMessage from '../components/common/ErrorMessage';
import { formatDate, getStatusClass, truncateText } from '../utils/formatters';
import { STATUS_LABELS } from '../utils/constants';

const HistoryPage = () => {
  const navigate = useNavigate();
  const [currentPage, setCurrentPage] = useState(1);
  const perPage = 10;

  const {
    resultsList,
    fetchResultsList,
    pagination,
    loading,
    error
  } = useAnalysisResults();

  useEffect(() => {
    fetchResultsList(currentPage, perPage);
  }, [currentPage, fetchResultsList]);

  const handleViewResults = (analysis) => {
    navigate(`/analysis?analysisId=${analysis.id}`);
  };

  const handlePageChange = (newPage) => {
    setCurrentPage(newPage);
  };

  const totalPages = Math.ceil(pagination.total / perPage);

  if (loading && resultsList.length === 0) {
    return <LoadingSpinner message="Loading analysis history..." />;
  }

  if (error && resultsList.length === 0) {
    return (
      <ErrorMessage
        error={error}
        onRetry={() => fetchResultsList(currentPage, perPage)}
      />
    );
  }

  return (
    <div className="history-page">
      <div className="page-header history-header">
        <div className="header-content">
          <h1>Analysis History</h1>
          <div className="header-actions">
            <Button
              onClick={() => navigate('/')}
              variant="primary"
            >
              New Analysis
            </Button>
          </div>
        </div>
        
        {pagination.total > 0 && (
          <div className="results-summary text-section">
            <p>
              Showing {((currentPage - 1) * perPage) + 1} to {Math.min(currentPage * perPage, pagination.total)} of {pagination.total} results
            </p>
          </div>
        )}
      </div>

      {resultsList.length === 0 && !loading ? (
        <Card>
          <div className="empty-state text-section">
            <h3>No Analysis History</h3>
            <p>You haven't run any analyses yet. Upload a transcript to get started.</p>
            <div className="button-group">
              <Button
                onClick={() => navigate('/')}
                variant="primary"
              >
                Upload Transcript
              </Button>
            </div>
          </div>
        </Card>
      ) : (
        <>
          <div className="results-list">
            {resultsList.map((analysis) => (
              <div key={analysis.id} className="result-item">
                <div className="result-header">
                  <div className="title-section">
                    <h3>{analysis.transcript_title || analysis.transcript_filename || `Analysis ${analysis.id.substring(0, 8)}...`}</h3>
                    <span className={`status ${getStatusClass(analysis.status)}`}>
                      {STATUS_LABELS[analysis.status] || analysis.status}
                    </span>
                  </div>
                  <div className="date-section">
                    <span>{formatDate(analysis.created_at)}</span>
                  </div>
                </div>

                <div className="text-section">
                  {analysis.summary ? (
                    <p className="summary-text">
                      {truncateText(analysis.summary, 200)}
                    </p>
                  ) : (
                    <p className="no-summary">
                      {analysis.status === 'completed' ? 'No summary available' : 'Analysis not yet completed'}
                    </p>
                  )}
                </div>

                <div className="result-meta">
                  {analysis.takeaways && (
                    <span className="stat">
                      <strong>{analysis.takeaways.length}</strong> takeaways
                    </span>
                  )}
                  
                  {analysis.fact_checks && (
                    <span className="stat">
                      <strong>{analysis.fact_checks.length}</strong> fact checks
                    </span>
                  )}
                  
                  {analysis.completed_at && (
                    <span className="stat">
                      Completed {formatDate(analysis.completed_at)}
                    </span>
                  )}
                </div>

                <div className="result-actions">
                  <Button
                    onClick={() => handleViewResults(analysis)}
                    variant={analysis.status === 'completed' ? 'primary' : 'secondary'}
                    disabled={analysis.status === 'failed'}
                  >
                    {analysis.status === 'completed' ? 'View Results' : 
                     analysis.status === 'failed' ? 'Failed' : 'View Status'}
                  </Button>
                </div>
              </div>
            ))}
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="pagination button-group">
              <Button
                onClick={() => handlePageChange(currentPage - 1)}
                disabled={currentPage <= 1 || loading}
                variant="secondary"
              >
                Previous
              </Button>

              <div className="page-info">
                <span>
                  Page {currentPage} of {totalPages}
                </span>
              </div>

              <Button
                onClick={() => handlePageChange(currentPage + 1)}
                disabled={currentPage >= totalPages || loading}
                variant="secondary"
              >
                Next
              </Button>
            </div>
          )}

          {loading && (
            <div className="loading-overlay">
              <LoadingSpinner message="Loading..." />
            </div>
          )}
        </>
      )}
    </div>
  );
};

export default HistoryPage;