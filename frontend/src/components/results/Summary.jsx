/**
 * Component for displaying podcast summary.
 */

import React from 'react';
import PropTypes from 'prop-types';
import Card from '../common/Card';

const Summary = ({ summary, className = '' }) => {
  if (!summary) return null;

  return (
    <Card title="Summary" className={className}>
      <div className="summary-content">
        <p>{summary}</p>
      </div>
    </Card>
  );
};

Summary.propTypes = {
  summary: PropTypes.string.isRequired,
  className: PropTypes.string
};

export default Summary;