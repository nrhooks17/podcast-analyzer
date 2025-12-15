/**
 * Reusable Card component for content layout.
 */

import React from 'react';
import PropTypes from 'prop-types';

const Card = ({ 
  children, 
  title, 
  subtitle,
  className = '',
  ...props 
}) => {
  const classes = ['card', className].filter(Boolean).join(' ');

  return (
    <div className={classes} {...props}>
      {title && (
        <div className="card-header">
          <h2>{title}</h2>
          {subtitle && <p>{subtitle}</p>}
        </div>
      )}
      <div className="card-content">
        {children}
      </div>
    </div>
  );
};

Card.propTypes = {
  children: PropTypes.node.isRequired,
  title: PropTypes.string,
  subtitle: PropTypes.string,
  className: PropTypes.string
};

export default Card;