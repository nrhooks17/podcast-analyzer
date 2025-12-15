/**
 * Component for displaying fact-check results in a table.
 */

import React from 'react';
import PropTypes from 'prop-types';
import Card from '../common/Card';
import { getVerdictClass, formatConfidence } from '../../utils/formatters';
import { VERDICT_LABELS } from '../../utils/constants';

const FactCheckTable = ({ factChecks, className = '' }) => {
  if (!factChecks || factChecks.length === 0) {
    return (
      <Card title="Fact Check Results" className={className}>
        <p>No factual claims were identified for verification in this transcript.</p>
      </Card>
    );
  }

  return (
    <Card title="Fact Check Results" className={className}>
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Claim</th>
              <th>Verification</th>
              <th>Confidence</th>
              <th>Evidence</th>
            </tr>
          </thead>
          <tbody>
            {factChecks.map((factCheck, index) => (
              <tr key={factCheck.id || index}>
                <td>
                  <div className="claim-text">
                    {factCheck.claim}
                  </div>
                </td>
                <td>
                  <span className={`verdict ${getVerdictClass(factCheck.verdict)}`}>
                    {VERDICT_LABELS[factCheck.verdict] || factCheck.verdict}
                  </span>
                </td>
                <td>
                  <div className="confidence-bar">
                    <div className="confidence-progress">
                      <div 
                        className="confidence-fill"
                        style={{ width: `${factCheck.confidence * 100}%` }}
                      />
                    </div>
                    <span className="confidence-text">
                      {formatConfidence(factCheck.confidence)}
                    </span>
                  </div>
                </td>
                <td>
                  <div className="evidence-text">
                    {factCheck.evidence || 'No evidence provided'}
                    {factCheck.sources && factCheck.sources.length > 0 && (
                      <div className="sources">
                        <strong>Sources:</strong>
                        <ul>
                          {factCheck.sources.map((source, sourceIndex) => (
                            <li key={sourceIndex}>
                              <a 
                                href={source} 
                                target="_blank" 
                                rel="noopener noreferrer"
                                className="source-link"
                              >
                                {source}
                              </a>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      
      <div className="fact-check-summary">
        <p>
          <strong>Summary:</strong> {factChecks.length} claim{factChecks.length !== 1 ? 's' : ''} verified
          ({factChecks.filter(fc => fc.verdict === 'true').length} true, 
          {factChecks.filter(fc => fc.verdict === 'partially_true').length} partial, 
          {factChecks.filter(fc => fc.verdict === 'unverifiable').length} unverifiable)
        </p>
      </div>
    </Card>
  );
};

FactCheckTable.propTypes = {
  factChecks: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string,
      claim: PropTypes.string.isRequired,
      verdict: PropTypes.string.isRequired,
      confidence: PropTypes.number.isRequired,
      evidence: PropTypes.string,
      sources: PropTypes.arrayOf(PropTypes.string)
    })
  ),
  className: PropTypes.string
};

export default FactCheckTable;