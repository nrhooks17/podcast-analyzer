/**
 * Progress bar component for showing upload/processing progress.
 */

import React from 'react';
import PropTypes from 'prop-types';

const ProgressBar = ({ 
  progress, 
  showLabel = true, 
  className = '',
  variant = 'primary'
}) => {
  const percentage = Math.max(0, Math.min(100, progress));
  
  return (
    <div className={`progress ${className}`}>
      <div 
        className={`progress-bar progress-bar-${variant}`}
        style={{ width: `${percentage}%` }}
        role="progressbar"
        aria-valuenow={percentage}
        aria-valuemin="0"
        aria-valuemax="100"
      >
        {showLabel && <span>{percentage}%</span>}
      </div>
    </div>
  );
};

ProgressBar.propTypes = {
  progress: PropTypes.number.isRequired,
  showLabel: PropTypes.bool,
  className: PropTypes.string,
  variant: PropTypes.oneOf(['primary', 'success', 'warning', 'danger'])
};

export default ProgressBar;