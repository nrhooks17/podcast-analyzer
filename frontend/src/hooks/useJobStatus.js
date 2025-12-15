/**
 * Custom hook for polling job status with automatic updates.
 */

import { useState, useEffect, useRef } from 'react';
import { getJobStatus } from '../services/api';
import { POLLING_INTERVAL } from '../utils/constants';

export const useJobStatus = (jobId, autoStart = true) => {
  const [status, setStatus] = useState(null);
  const [isPolling, setIsPolling] = useState(false);
  const [error, setError] = useState(null);
  const [lastUpdated, setLastUpdated] = useState(null);

  const intervalRef = useRef(null);
  const mountedRef = useRef(true);

  /**
   * Fetch current job status
   */
  const fetchStatus = async () => {
    if (!jobId) return;

    try {
      const result = await getJobStatus(jobId);
      
      if (mountedRef.current) {
        setStatus(result);
        setError(null);
        setLastUpdated(new Date());

        // Stop polling if job is completed or failed
        if (result.status === 'completed' || result.status === 'failed') {
          stopPolling();
        }
      }
    } catch (err) {
      console.error('Failed to fetch job status:', err);
      if (mountedRef.current) {
        setError(err);
        stopPolling();
      }
    }
  };

  /**
   * Start polling for status updates
   */
  const startPolling = () => {
    if (isPolling || !jobId) return;

    setIsPolling(true);
    setError(null);

    // Fetch immediately
    fetchStatus();

    // Set up interval for subsequent fetches
    intervalRef.current = setInterval(fetchStatus, POLLING_INTERVAL);
  };

  /**
   * Stop polling
   */
  const stopPolling = () => {
    setIsPolling(false);
    
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  };

  /**
   * Manually refresh status
   */
  const refresh = () => {
    fetchStatus();
  };

  // Auto-start polling if jobId is provided and autoStart is true
  useEffect(() => {
    if (jobId && autoStart) {
      startPolling();
    }

    return () => {
      stopPolling();
    };
  }, [jobId, autoStart]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      mountedRef.current = false;
      stopPolling();
    };
  }, []);

  return {
    status,
    isPolling,
    error,
    lastUpdated,
    startPolling,
    stopPolling,
    refresh
  };
};