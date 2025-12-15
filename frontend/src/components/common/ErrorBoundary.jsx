/**
 * Error boundary component for handling React errors gracefully.
 */

import React from 'react';
import PropTypes from 'prop-types';

class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false, error: null, errorInfo: null };
  }

  static getDerivedStateFromError(error) {
    // Update state so the next render will show the fallback UI
    return { hasError: true };
  }

  componentDidCatch(error, errorInfo) {
    // Log the error to console in development
    console.error('Error caught by boundary:', error, errorInfo);
    
    // Update state with error details
    this.setState({
      error,
      errorInfo
    });
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null, errorInfo: null });
  };

  render() {
    if (this.state.hasError) {
      const isDevelopment = import.meta.env.DEV;
      
      return (
        <div className="error-boundary">
          <div className="error-content">
            <h2>Something went wrong</h2>
            <p>
              An unexpected error occurred. Please refresh the page or try again later.
            </p>
            
            <div className="error-actions">
              <button onClick={this.handleRetry} className="btn btn-primary">
                Try Again
              </button>
              
              <button 
                onClick={() => window.location.reload()} 
                className="btn btn-secondary"
              >
                Refresh Page
              </button>
            </div>

            {isDevelopment && this.state.error && (
              <details className="error-details">
                <summary>Error Details (Development)</summary>
                <pre>{this.state.error.toString()}</pre>
                {this.state.errorInfo && (
                  <pre>{this.state.errorInfo.componentStack}</pre>
                )}
              </details>
            )}
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

ErrorBoundary.propTypes = {
  children: PropTypes.node.isRequired
};

export default ErrorBoundary;