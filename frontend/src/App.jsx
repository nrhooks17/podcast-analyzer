/**
 * Main App component with routing and error boundaries.
 */

import React from 'react';
import { Routes, Route, Link, useLocation } from 'react-router-dom';
import HomePage from './pages/HomePage';
import AnalysisPage from './pages/AnalysisPage';
import HistoryPage from './pages/HistoryPage';
import ErrorBoundary from './components/common/ErrorBoundary';

const App = () => {
  const location = useLocation();

  const isActiveRoute = (path) => {
    if (path === '/' && location.pathname === '/') return true;
    if (path !== '/' && location.pathname.startsWith(path)) return true;
    return false;
  };

  return (
    <div className="app">
      <header className="header">
        <div className="container">
          <div className="header-content">
            <div className="logo-section">
              <h1>Podcast Analyzer</h1>
              <p>AI-powered transcript analysis for ad agencies</p>
            </div>
            
            <nav className="nav">
              <Link 
                to="/" 
                className={`nav-link ${isActiveRoute('/') ? 'active' : ''}`}
              >
                Upload
              </Link>
              <Link 
                to="/history" 
                className={`nav-link ${isActiveRoute('/history') ? 'active' : ''}`}
              >
                History
              </Link>
            </nav>
          </div>
        </div>
      </header>

      <main className="main">
        <div className="container">
          <ErrorBoundary>
            <Routes>
              <Route path="/" element={<HomePage />} />
              <Route path="/analysis" element={<AnalysisPage />} />
              <Route path="/history" element={<HistoryPage />} />
              <Route path="*" element={<NotFoundPage />} />
            </Routes>
          </ErrorBoundary>
        </div>
      </main>

      <footer className="footer">
        <div className="container">
          <p>&copy; 2025 Podcast Analyzer. Built with FastAPI and React.</p>
        </div>
      </footer>
    </div>
  );
};

// Simple 404 page
const NotFoundPage = () => (
  <div className="not-found">
    <h2>Page Not Found</h2>
    <p>The page you're looking for doesn't exist.</p>
    <Link to="/" className="btn btn-primary">
      Go Home
    </Link>
  </div>
);

export default App;