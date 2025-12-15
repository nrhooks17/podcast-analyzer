/**
 * Error message component for displaying user-friendly errors.
 */

import React from 'react';
import PropTypes from 'prop-types';

const ErrorMessage = ({ 
  error, 
  onRetry, 
  className = '',
  showDetails = false 
}) => {
  if (!error) return null;

  const message = error.message || 'An unexpected error occurred';
  const code = error.code;
  const correlationId = error.correlationId;

  return (
    <div className={`error ${className}`}>
      <div className="error-content">
        <h4>Error</h4>
        <p>{message}</p>
        
        {showDetails && (
          <div className="error-details">
            {code && <p><strong>Code:</strong> {code}</p>}
            {correlationId && <p><strong>Reference:</strong> {correlationId}</p>}
          </div>
        )}
        
        {onRetry && (
          <button onClick={onRetry} className="btn btn-primary">
            Try Again
          </button>
        )}
      </div>
    </div>
  );
};

ErrorMessage.propTypes = {
  error: PropTypes.shape({
    message: PropTypes.string,
    code: PropTypes.string,
    correlationId: PropTypes.string,
    details: PropTypes.string
  }),
  onRetry: PropTypes.func,
  className: PropTypes.string,
  showDetails: PropTypes.bool
};

export default ErrorMessage;