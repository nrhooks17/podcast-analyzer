/**
 * Component for displaying key takeaways from the podcast.
 */

import React from 'react';
import PropTypes from 'prop-types';
import Card from '../common/Card';

const KeyTakeaways = ({ takeaways, className = '' }) => {
  if (!takeaways || takeaways.length === 0) {
    return (
      <Card title="Key Takeaways" className={className}>
        <p>No key takeaways available for this analysis.</p>
      </Card>
    );
  }

  return (
    <Card title="Key Takeaways" className={className}>
      <ul className="takeaways-list">
        {takeaways.map((takeaway, index) => (
          <li key={index}>
            {takeaway}
          </li>
        ))}
      </ul>
    </Card>
  );
};

KeyTakeaways.propTypes = {
  takeaways: PropTypes.arrayOf(PropTypes.string),
  className: PropTypes.string
};

export default KeyTakeaways;