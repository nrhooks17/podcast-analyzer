/**
 * Utility functions for formatting data display.
 */

/**
 * Format date for display
 * @param {string} dateString - ISO date string
 * @returns {string} Formatted date
 */
export const formatDate = (dateString) => {
  if (!dateString) return 'N/A';
  
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
};

/**
 * Format file size in human readable format
 * @param {number} bytes - File size in bytes
 * @returns {string} Formatted file size
 */
export const formatFileSize = (bytes) => {
  if (!bytes) return '0 B';
  
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

/**
 * Format confidence score as percentage
 * @param {number} confidence - Confidence score (0-1)
 * @returns {string} Formatted percentage
 */
export const formatConfidence = (confidence) => {
  if (confidence === undefined || confidence === null) return '0%';
  return Math.round(confidence * 100) + '%';
};

/**
 * Format word count with commas
 * @param {number} count - Word count
 * @returns {string} Formatted word count
 */
export const formatWordCount = (count) => {
  if (!count) return '0';
  return count.toLocaleString();
};

/**
 * Truncate text to specified length
 * @param {string} text - Text to truncate
 * @param {number} maxLength - Maximum length
 * @returns {string} Truncated text
 */
export const truncateText = (text, maxLength = 100) => {
  if (!text || text.length <= maxLength) return text;
  return text.substr(0, maxLength) + '...';
};

/**
 * Get CSS class for status
 * @param {string} status - Status value
 * @returns {string} CSS class name
 */
export const getStatusClass = (status) => {
  const classes = {
    pending: 'status-pending',
    processing: 'status-processing',
    completed: 'status-completed',
    failed: 'status-failed'
  };
  return classes[status] || 'status-pending';
};

/**
 * Get CSS class for verdict
 * @param {string} verdict - Verdict value
 * @returns {string} CSS class name
 */
export const getVerdictClass = (verdict) => {
  const classes = {
    true: 'verdict-true',
    false: 'verdict-false',
    partially_true: 'verdict-partially-true',
    unverifiable: 'verdict-unverifiable'
  };
  return classes[verdict] || 'verdict-unverifiable';
};