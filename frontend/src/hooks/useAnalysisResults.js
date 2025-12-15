/**
 * Custom hook for managing analysis results data.
 */

import { useState, useEffect, useCallback } from 'react';
import { getAnalysisResults, getAnalysisResultsList } from '../services/api';

export const useAnalysisResults = () => {
  const [results, setResults] = useState(null);
  const [resultsList, setResultsList] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [pagination, setPagination] = useState({
    page: 1,
    perPage: 20,
    total: 0
  });

  /**
   * Fetch analysis results by ID
   * @param {string} analysisId - Analysis ID
   */
  const fetchResults = useCallback(async (analysisId) => {
    if (!analysisId) return;

    setLoading(true);
    setError(null);

    try {
      const data = await getAnalysisResults(analysisId);
      setResults(data);
    } catch (err) {
      console.error('Failed to fetch analysis results:', err);
      
      // If analysis is not completed (409 error), don't treat as error
      if (err.status === 409 || err.code === 'ANALYSIS_NOT_COMPLETED') {
        setResults(null);
        setError(null);
      } else {
        setError(err);
        setResults(null);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  /**
   * Fetch list of analysis results
   * @param {number} page - Page number
   * @param {number} perPage - Items per page
   */
  const fetchResultsList = useCallback(async (page = 1, perPage = 20) => {
    setLoading(true);
    setError(null);

    try {
      const data = await getAnalysisResultsList(page, perPage);
      setResultsList(data.results);
      setPagination({
        page: data.page,
        perPage: data.per_page,
        total: data.total
      });
    } catch (err) {
      console.error('Failed to fetch analysis results list:', err);
      setError(err);
      setResultsList([]);
    } finally {
      setLoading(false);
    }
  }, []);

  /**
   * Clear current results
   */
  const clearResults = () => {
    setResults(null);
    setError(null);
  };

  /**
   * Clear results list
   */
  const clearResultsList = () => {
    setResultsList([]);
    setError(null);
    setPagination({ page: 1, perPage: 20, total: 0 });
  };

  return {
    // Single result
    results,
    fetchResults,
    clearResults,

    // Results list
    resultsList,
    fetchResultsList,
    clearResultsList,
    pagination,

    // Common state
    loading,
    error
  };
};