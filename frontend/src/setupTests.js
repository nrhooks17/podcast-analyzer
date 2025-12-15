/**
 * Test setup for React components using Vitest and Testing Library.
 */

import '@testing-library/jest-dom';

// Mock environment variables for tests
import.meta.env.VITE_API_BASE_URL = 'http://localhost:8000';