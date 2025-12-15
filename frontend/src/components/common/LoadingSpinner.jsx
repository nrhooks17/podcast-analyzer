/**
 * Loading spinner component.
 */

import React from 'react';
import PropTypes from 'prop-types';

const LoadingSpinner = ({ message = 'Loading...', className = '' }) => {
  return (
    <div className={`loading-container ${className}`}>
      <div className="spinner"></div>
      {message && <p>{message}</p>}
    </div>
  );
};

LoadingSpinner.propTypes = {
  message: PropTypes.string,
  className: PropTypes.string
};

export default LoadingSpinner;