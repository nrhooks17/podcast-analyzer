/**
 * Analysis progress indicator showing processing stages.
 */

import React from 'react';
import PropTypes from 'prop-types';

const AnalysisProgress = ({ status, startTime }) => {
  const stages = [
    { id: 'pending', label: 'Queued', description: 'Waiting for processing' },
    { id: 'processing', label: 'Processing', description: 'Analyzing transcript' },
    { id: 'summarizing', label: 'Summarizing', description: 'Generating summary' },
    { id: 'extracting', label: 'Key Takeaways', description: 'Extracting insights' },
    { id: 'fact-checking', label: 'Fact Checking', description: 'Verifying claims' },
    { id: 'completed', label: 'Complete', description: 'Analysis finished' }
  ];

  // Determine current stage based on status and elapsed time
  const getCurrentStage = () => {
    if (status === 'pending') return 0;
    if (status === 'failed') return -1;
    if (status === 'completed') return stages.length - 1;
    
    // For processing status, estimate stage based on elapsed time
    if (status === 'processing' && startTime) {
      const elapsed = (Date.now() - new Date(startTime).getTime()) / 1000;
      
      // Rough timing estimates for each stage
      if (elapsed < 10) return 1; // Processing
      if (elapsed < 25) return 2; // Summarizing
      if (elapsed < 40) return 3; // Extracting
      if (elapsed < 60) return 4; // Fact-checking
      return 4; // Keep at fact-checking until complete
    }
    
    return 1; // Default to processing
  };

  const currentStage = getCurrentStage();
  
  if (status === 'failed') {
    return (
      <div className="analysis-progress failed">
        <div className="progress-header">
          <h4>Analysis Failed</h4>
          <p>An error occurred during processing</p>
        </div>
      </div>
    );
  }

  const getStageStatus = (stageIndex) => {
    if (currentStage === -1) return 'failed';
    if (stageIndex < currentStage) return 'completed';
    if (stageIndex === currentStage) return 'active';
    return 'pending';
  };

  const progressPercentage = status === 'completed' 
    ? 100 
    : Math.max(0, Math.min(95, (currentStage / (stages.length - 1)) * 100));

  return (
    <div className="analysis-progress">
      <div className="progress-header">
        <h4>Analysis in Progress</h4>
        <div className="progress-percentage">
          {Math.round(progressPercentage)}%
        </div>
      </div>
      
      <div className="progress-bar-container">
        <div className="progress-bar-track">
          <div 
            className="progress-bar-fill" 
            style={{ width: `${progressPercentage}%` }}
          />
        </div>
      </div>
      
      <div className="stages-list">
        {stages.map((stage, index) => {
          const stageStatus = getStageStatus(index);
          return (
            <div key={stage.id} className={`stage ${stageStatus}`}>
              <div className="stage-indicator">
                <div className="stage-circle">
                  {stageStatus === 'completed' && '✓'}
                  {stageStatus === 'active' && (
                    <div className="spinner-small" />
                  )}
                  {stageStatus === 'pending' && index + 1}
                </div>
              </div>
              <div className="stage-content">
                <div className="stage-label">{stage.label}</div>
                <div className="stage-description">{stage.description}</div>
              </div>
            </div>
          );
        })}
      </div>
      
      <div className="progress-footer">
        <p>This usually takes 2-5 minutes depending on transcript length.</p>
      </div>
    </div>
  );
};

AnalysisProgress.propTypes = {
  status: PropTypes.oneOf(['pending', 'processing', 'completed', 'failed']).isRequired,
  startTime: PropTypes.string
};

export default AnalysisProgress;