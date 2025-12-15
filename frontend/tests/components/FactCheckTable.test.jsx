/**
 * Tests for FactCheckTable component.
 */

import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import FactCheckTable from '../../src/components/results/FactCheckTable';

describe('FactCheckTable', () => {
  const mockFactChecks = [
    {
      id: '1',
      claim: 'NASA Mars mission launches in 2026',
      verdict: 'true',
      confidence: 0.92,
      evidence: 'Confirmed by NASA official announcements',
      sources: ['https://nasa.gov/mars-mission']
    },
    {
      id: '2',
      claim: 'Bitcoin will reach $200K by 2025',
      verdict: 'unverifiable',
      confidence: 0.40,
      evidence: 'Future prediction, no factual basis',
      sources: []
    },
    {
      id: '3',
      claim: 'Humans will be on Mars by 2035',
      verdict: 'partially_true',
      confidence: 0.65,
      evidence: 'Timeline varies by source, 2030-2040 range cited',
      sources: ['https://example.com/mars-timeline']
    }
  ];

  it('renders fact check table with data', () => {
    render(<FactCheckTable factChecks={mockFactChecks} />);
    
    expect(screen.getByText('Fact Check Results')).toBeInTheDocument();
    expect(screen.getByRole('table')).toBeInTheDocument();
    
    // Check table headers
    expect(screen.getByText('Claim')).toBeInTheDocument();
    expect(screen.getByText('Verification')).toBeInTheDocument();
    expect(screen.getByText('Confidence')).toBeInTheDocument();
    expect(screen.getByText('Evidence')).toBeInTheDocument();
  });

  it('displays all fact check claims', () => {
    render(<FactCheckTable factChecks={mockFactChecks} />);
    
    expect(screen.getByText('NASA Mars mission launches in 2026')).toBeInTheDocument();
    expect(screen.getByText('Bitcoin will reach $200K by 2025')).toBeInTheDocument();
    expect(screen.getByText('Humans will be on Mars by 2035')).toBeInTheDocument();
  });

  it('displays verdict labels correctly', () => {
    render(<FactCheckTable factChecks={mockFactChecks} />);
    
    expect(screen.getByText('True')).toBeInTheDocument();
    expect(screen.getByText('Unverifiable')).toBeInTheDocument();
    expect(screen.getByText('Partially True')).toBeInTheDocument();
  });

  it('displays confidence percentages', () => {
    render(<FactCheckTable factChecks={mockFactChecks} />);
    
    expect(screen.getByText('92%')).toBeInTheDocument();
    expect(screen.getByText('40%')).toBeInTheDocument();
    expect(screen.getByText('65%')).toBeInTheDocument();
  });

  it('displays evidence text', () => {
    render(<FactCheckTable factChecks={mockFactChecks} />);
    
    expect(screen.getByText('Confirmed by NASA official announcements')).toBeInTheDocument();
    expect(screen.getByText('Future prediction, no factual basis')).toBeInTheDocument();
    expect(screen.getByText('Timeline varies by source, 2030-2040 range cited')).toBeInTheDocument();
  });

  it('displays sources when available', () => {
    render(<FactCheckTable factChecks={mockFactChecks} />);
    
    expect(screen.getByRole('link', { name: 'https://nasa.gov/mars-mission' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'https://example.com/mars-timeline' })).toBeInTheDocument();
  });

  it('applies correct CSS classes for verdicts', () => {
    const { container } = render(<FactCheckTable factChecks={mockFactChecks} />);
    
    const trueVerdict = screen.getByText('True');
    const unverifiableVerdict = screen.getByText('Unverifiable');
    const partialVerdict = screen.getByText('Partially True');
    
    expect(trueVerdict).toHaveClass('verdict-true');
    expect(unverifiableVerdict).toHaveClass('verdict-unverifiable');
    expect(partialVerdict).toHaveClass('verdict-partially-true');
  });

  it('displays summary with correct counts', () => {
    render(<FactCheckTable factChecks={mockFactChecks} />);
    
    expect(screen.getByText(/3 claims verified/)).toBeInTheDocument();
    expect(screen.getByText(/1 true/)).toBeInTheDocument();
    expect(screen.getByText(/1 partial/)).toBeInTheDocument();
    expect(screen.getByText(/1 unverifiable/)).toBeInTheDocument();
  });

  it('renders empty state when no fact checks provided', () => {
    render(<FactCheckTable factChecks={[]} />);
    
    expect(screen.getByText('Fact Check Results')).toBeInTheDocument();
    expect(screen.getByText(/No factual claims were identified/)).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('handles null fact checks', () => {
    render(<FactCheckTable factChecks={null} />);
    
    expect(screen.getByText('Fact Check Results')).toBeInTheDocument();
    expect(screen.getByText(/No factual claims were identified/)).toBeInTheDocument();
  });

  it('handles fact checks without sources', () => {
    const factCheckNoSources = [{
      id: '1',
      claim: 'Test claim',
      verdict: 'true',
      confidence: 0.8,
      evidence: 'Test evidence',
      sources: null
    }];
    
    render(<FactCheckTable factChecks={factCheckNoSources} />);
    
    expect(screen.getByText('Test claim')).toBeInTheDocument();
    expect(screen.getByText('Test evidence')).toBeInTheDocument();
    expect(screen.queryByText('Sources:')).not.toBeInTheDocument();
  });

  it('displays confidence bars with correct width', () => {
    const { container } = render(<FactCheckTable factChecks={mockFactChecks} />);
    
    const confidenceFills = container.querySelectorAll('.confidence-fill');
    
    expect(confidenceFills[0]).toHaveStyle('width: 92%');
    expect(confidenceFills[1]).toHaveStyle('width: 40%');
    expect(confidenceFills[2]).toHaveStyle('width: 65%');
  });
});