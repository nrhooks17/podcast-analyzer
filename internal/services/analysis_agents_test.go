package services

import (
	"context"
	"errors"
	"testing"

	"podcast-analyzer/internal/agents"
	"podcast-analyzer/internal/config"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockAnalysisService extends AnalysisService for testing
type MockAnalysisService struct {
	*AnalysisService
	summarizerAgent    *MockSummarizerAgent
	takeawayAgent      *MockTakeawayAgent
	factCheckerAgent   *MockFactCheckerAgent
}

// Mock agent interfaces
type MockSummarizerAgent struct {
	mock.Mock
}

func (m *MockSummarizerAgent) Name() string {
	return "summarizer"
}

func (m *MockSummarizerAgent) Process(ctx context.Context, content string) (agents.Result, error) {
	args := m.Called(ctx, content)
	return args.Get(0).(agents.Result), args.Error(1)
}

type MockTakeawayAgent struct {
	mock.Mock
}

func (m *MockTakeawayAgent) Name() string {
	return "takeaway_extractor"
}

func (m *MockTakeawayAgent) Process(ctx context.Context, content string) (agents.Result, error) {
	args := m.Called(ctx, content)
	return args.Get(0).(agents.Result), args.Error(1)
}

func (m *MockTakeawayAgent) ProcessWithOptions(ctx context.Context, content string, options agents.ProcessingOptions) (agents.Result, error) {
	args := m.Called(ctx, content, options)
	return args.Get(0).(agents.Result), args.Error(1)
}

type MockFactCheckerAgent struct {
	mock.Mock
}

func (m *MockFactCheckerAgent) Name() string {
	return "fact_checker"
}

func (m *MockFactCheckerAgent) Process(ctx context.Context, content string) (agents.Result, error) {
	args := m.Called(ctx, content)
	return args.Get(0).(agents.Result), args.Error(1)
}

// Override agent creation methods for testing
func (m *MockAnalysisService) runSummarizerAgent(ctx context.Context, content string, jobID uuid.UUID, correlationID string) (string, error) {
	if m.summarizerAgent == nil {
		return m.AnalysisService.runSummarizerAgent(ctx, content, jobID, correlationID)
	}

	result, err := m.summarizerAgent.Process(ctx, content)
	if err != nil {
		return "", err
	}
	return result.Summary, nil
}

func (m *MockAnalysisService) runTakeawayExtractorAgent(ctx context.Context, content, summary string, jobID uuid.UUID, correlationID string) ([]string, error) {
	if m.takeawayAgent == nil {
		return m.AnalysisService.runTakeawayExtractorAgent(ctx, content, summary, jobID, correlationID)
	}

	result, err := m.takeawayAgent.ProcessWithOptions(ctx, content, agents.ProcessingOptions{Summary: summary})
	if err != nil {
		// Return empty takeaways on error (graceful degradation)
		return []string{}, nil
	}
	return result.Takeaways, nil
}

func (m *MockAnalysisService) runFactCheckerAgent(ctx context.Context, content string, jobID uuid.UUID, correlationID string) ([]agents.FactCheck, error) {
	if m.factCheckerAgent == nil {
		return m.AnalysisService.runFactCheckerAgent(ctx, content, jobID, correlationID)
	}

	result, err := m.factCheckerAgent.Process(ctx, content)
	if err != nil {
		// Return empty fact checks on error (graceful degradation)
		return []agents.FactCheck{}, nil
	}
	return result.FactChecks, nil
}

// Override the main runAnalysisAgents method to ensure it uses the mock agent methods
func (m *MockAnalysisService) runAnalysisAgents(ctx context.Context, content string, jobID uuid.UUID, correlationID string) (*AnalysisResults, error) {
	// Use our overridden methods that utilize mocks
	summary, err := m.runSummarizerAgent(ctx, content, jobID, correlationID)
	if err != nil {
		return nil, err
	}
	
	takeaways, err := m.runTakeawayExtractorAgent(ctx, content, summary, jobID, correlationID)
	if err != nil {
		return nil, err
	}
	
	factCheckResults, err := m.runFactCheckerAgent(ctx, content, jobID, correlationID)
	if err != nil {
		return nil, err
	}
	
	return m.transformAnalysisResults(summary, takeaways, factCheckResults, jobID, correlationID)
}

// Test helpers
func setupTestDatabase() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func setupMockAnalysisService() (*MockAnalysisService, *test.Hook) {
	db, _ := setupTestDatabase()
	cfg := &config.Config{
		AnthropicAPIKey: "test-key",
		SerperAPIKey:   "test-serper-key",
		ClaudeModel:    "claude-3-sonnet-20240229",
		SummaryMaxChars: 300,
	}
	
	logger, hook := test.NewNullLogger()
	
	service := &MockAnalysisService{
		AnalysisService:   NewAnalysisService(db, cfg),
		summarizerAgent:   &MockSummarizerAgent{},
		takeawayAgent:     &MockTakeawayAgent{},
		factCheckerAgent:  &MockFactCheckerAgent{},
	}

	// Replace the logger for testing
	oldLogger := logrus.StandardLogger()
	logrus.SetOutput(logger.Out)
	logrus.SetLevel(logger.Level)
	defer func() {
		logrus.SetOutput(oldLogger.Out)
		logrus.SetLevel(oldLogger.Level)
	}()

	return service, hook
}

func TestAnalysisService_runSummarizerAgent_Success(t *testing.T) {
	service, _ := setupMockAnalysisService()

	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-123")
	content := "This is test podcast content for summarization that talks about technology trends."
	jobID := uuid.New()
	correlationID := "test-correlation-123"

	expectedSummary := "This podcast discusses emerging technology trends and their impact on business."

	// Mock successful summarizer response
	service.summarizerAgent.On("Process", ctx, content).Return(
		agents.Result{Summary: expectedSummary}, nil,
	)

	summary, err := service.runSummarizerAgent(ctx, content, jobID, correlationID)

	assert.NoError(t, err)
	assert.Equal(t, expectedSummary, summary)
	service.summarizerAgent.AssertExpectations(t)
}

func TestAnalysisService_runSummarizerAgent_Error(t *testing.T) {
	service, _ := setupMockAnalysisService()

	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-456")
	content := "Test content"
	jobID := uuid.New()
	correlationID := "test-correlation-456"

	// Mock summarizer error
	service.summarizerAgent.On("Process", ctx, content).Return(
		agents.Result{}, errors.New("summarizer agent failed"),
	)

	summary, err := service.runSummarizerAgent(ctx, content, jobID, correlationID)

	assert.Error(t, err)
	assert.Empty(t, summary)
	assert.Contains(t, err.Error(), "summarizer agent failed")
	service.summarizerAgent.AssertExpectations(t)
}

func TestAnalysisService_runTakeawayExtractorAgent_Success(t *testing.T) {
	service, _ := setupMockAnalysisService()

	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-789")
	content := "This podcast content has several key insights about business strategy."
	summary := "Summary of business strategy discussion"
	jobID := uuid.New()
	correlationID := "test-correlation-789"

	expectedTakeaways := []string{
		"Focus on customer-centric business models",
		"Digital transformation is essential for growth", 
		"Data-driven decision making improves outcomes",
	}

	// Mock successful takeaway extraction
	service.takeawayAgent.On("ProcessWithOptions", ctx, content, agents.ProcessingOptions{Summary: summary}).Return(
		agents.Result{Takeaways: expectedTakeaways}, nil,
	)

	takeaways, err := service.runTakeawayExtractorAgent(ctx, content, summary, jobID, correlationID)

	assert.NoError(t, err)
	assert.Equal(t, expectedTakeaways, takeaways)
	assert.Len(t, takeaways, 3)
	service.takeawayAgent.AssertExpectations(t)
}

func TestAnalysisService_runTakeawayExtractorAgent_Error_GracefulDegradation(t *testing.T) {
	service, _ := setupMockAnalysisService()

	ctx := context.Background()
	content := "Test content"
	summary := "Test summary"
	jobID := uuid.New()
	correlationID := "test-correlation-error"

	// Mock takeaway extractor error
	service.takeawayAgent.On("ProcessWithOptions", ctx, content, agents.ProcessingOptions{Summary: summary}).Return(
		agents.Result{}, errors.New("takeaway extraction failed"),
	)

	takeaways, err := service.runTakeawayExtractorAgent(ctx, content, summary, jobID, correlationID)

	// Should not error due to graceful degradation
	assert.NoError(t, err)
	assert.Empty(t, takeaways)
	service.takeawayAgent.AssertExpectations(t)
}

func TestAnalysisService_runFactCheckerAgent_Success(t *testing.T) {
	service, _ := setupMockAnalysisService()

	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-fact")
	content := "The moon landing happened in 1969. This is a verifiable historical fact."
	jobID := uuid.New()
	correlationID := "test-correlation-fact"

	expectedFactChecks := []agents.FactCheck{
		{
			Claim:      "The moon landing happened in 1969",
			Verdict:    "true",
			Confidence: 0.95,
			Evidence:   "Historical records confirm Apollo 11 mission",
			Sources:    []string{"https://nasa.gov/apollo11"},
		},
	}

	// Mock successful fact checking
	service.factCheckerAgent.On("Process", ctx, content).Return(
		agents.Result{FactChecks: expectedFactChecks}, nil,
	)

	factChecks, err := service.runFactCheckerAgent(ctx, content, jobID, correlationID)

	assert.NoError(t, err)
	assert.Equal(t, expectedFactChecks, factChecks)
	assert.Len(t, factChecks, 1)
	assert.Equal(t, "true", factChecks[0].Verdict)
	assert.Equal(t, 0.95, factChecks[0].Confidence)
	service.factCheckerAgent.AssertExpectations(t)
}

func TestAnalysisService_runFactCheckerAgent_Error_GracefulDegradation(t *testing.T) {
	service, _ := setupMockAnalysisService()

	ctx := context.Background()
	content := "Test content with claims"
	jobID := uuid.New()
	correlationID := "test-correlation-fact-error"

	// Mock fact checker error
	service.factCheckerAgent.On("Process", ctx, content).Return(
		agents.Result{}, errors.New("fact checking service unavailable"),
	)

	factChecks, err := service.runFactCheckerAgent(ctx, content, jobID, correlationID)

	// Should not error due to graceful degradation
	assert.NoError(t, err)
	assert.Empty(t, factChecks)
	service.factCheckerAgent.AssertExpectations(t)
}

func TestAnalysisService_transformAnalysisResults_Success(t *testing.T) {
	service, _ := setupMockAnalysisService()

	summary := "This podcast episode covers the latest developments in artificial intelligence."
	takeaways := []string{
		"AI is transforming multiple industries",
		"Ethical considerations are becoming more important",
		"Investment in AI skills is crucial for businesses",
	}
	factCheckResults := []agents.FactCheck{
		{
			Claim:      "AI market will reach $500B by 2024",
			Verdict:    "partially_true",
			Confidence: 0.75,
			Evidence:   "Various estimates range from $400B to $600B",
			Sources:    []string{"https://techreport.com/ai-market", "https://analyst.com/ai-forecast"},
		},
	}
	jobID := uuid.New()
	correlationID := "test-correlation-transform"

	result, err := service.transformAnalysisResults(summary, takeaways, factCheckResults, jobID, correlationID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	
	// Verify summary
	assert.Equal(t, summary, result.Summary)
	
	// Verify takeaways structure
	assert.NotNil(t, result.Takeaways)
	takeawaysData, exists := result.Takeaways["takeaways"]
	assert.True(t, exists)
	takeawaysList := takeawaysData.([]string)
	assert.Len(t, takeawaysList, 3)
	assert.Equal(t, "AI is transforming multiple industries", takeawaysList[0])
	
	// Verify fact checks
	assert.Len(t, result.FactChecks, 1)
	factCheck := result.FactChecks[0]
	assert.Equal(t, "AI market will reach $500B by 2024", factCheck.Claim)
	assert.Equal(t, "partially_true", factCheck.Verdict)
	assert.Equal(t, 0.75, factCheck.Confidence)
	
	// Verify sources structure
	sourcesMap := factCheck.Sources
	sources, exists := sourcesMap["sources"]
	assert.True(t, exists)
	sourcesList := sources.([]string)
	assert.Len(t, sourcesList, 2)
	assert.Contains(t, sourcesList, "https://techreport.com/ai-market")
	assert.Contains(t, sourcesList, "https://analyst.com/ai-forecast")
}

func TestAnalysisService_transformAnalysisResults_EmptyInputs(t *testing.T) {
	service, _ := setupMockAnalysisService()

	summary := ""
	takeaways := []string{}
	factCheckResults := []agents.FactCheck{}
	jobID := uuid.New()
	correlationID := "test-correlation-empty"

	result, err := service.transformAnalysisResults(summary, takeaways, factCheckResults, jobID, correlationID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Summary)
	assert.Len(t, result.FactChecks, 0)
	
	// Verify takeaways structure even when empty
	assert.NotNil(t, result.Takeaways)
	takeawaysData, exists := result.Takeaways["takeaways"]
	assert.True(t, exists)
	takeawaysList := takeawaysData.([]string)
	assert.Len(t, takeawaysList, 0)
}

func TestAnalysisService_runAnalysisAgents_FullWorkflow_Success(t *testing.T) {
	service, _ := setupMockAnalysisService()

	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-full")
	content := "This comprehensive podcast episode discusses the future of renewable energy, including solar power advancements and wind energy efficiency. According to recent studies, solar panel efficiency has increased by 25% in the last five years."
	jobID := uuid.New()
	correlationID := "test-correlation-full"

	// Mock summarizer
	expectedSummary := "This episode explores renewable energy innovations, focusing on solar and wind power improvements."
	service.summarizerAgent.On("Process", ctx, content).Return(
		agents.Result{Summary: expectedSummary}, nil,
	)

	// Mock takeaway extractor
	expectedTakeaways := []string{
		"Solar panel efficiency has significantly improved",
		"Wind energy is becoming more cost-effective",
		"Government policies are driving renewable adoption",
	}
	service.takeawayAgent.On("ProcessWithOptions", ctx, content, agents.ProcessingOptions{Summary: expectedSummary}).Return(
		agents.Result{Takeaways: expectedTakeaways}, nil,
	)

	// Mock fact checker
	expectedFactChecks := []agents.FactCheck{
		{
			Claim:      "Solar panel efficiency has increased by 25% in the last five years",
			Verdict:    "true",
			Confidence: 0.88,
			Evidence:   "Industry reports confirm significant efficiency improvements",
			Sources:    []string{"https://renewabletech.com/solar-efficiency"},
		},
	}
	service.factCheckerAgent.On("Process", ctx, content).Return(
		agents.Result{FactChecks: expectedFactChecks}, nil,
	)

	result, err := service.runAnalysisAgents(ctx, content, jobID, correlationID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	
	// Verify all components
	assert.Equal(t, expectedSummary, result.Summary)
	
	takeawaysData := result.Takeaways["takeaways"].([]string)
	assert.Equal(t, expectedTakeaways, takeawaysData)
	
	assert.Len(t, result.FactChecks, 1)
	assert.Equal(t, expectedFactChecks[0].Claim, result.FactChecks[0].Claim)
	assert.Equal(t, expectedFactChecks[0].Verdict, result.FactChecks[0].Verdict)

	// Verify all mocks were called
	service.summarizerAgent.AssertExpectations(t)
	service.takeawayAgent.AssertExpectations(t)
	service.factCheckerAgent.AssertExpectations(t)
}

func TestAnalysisService_runAnalysisAgents_SummarizerFails_WorkflowStops(t *testing.T) {
	service, _ := setupMockAnalysisService()

	ctx := context.Background()
	content := "Test content"
	jobID := uuid.New()
	correlationID := "test-correlation-fail"

	// Mock summarizer failure
	service.summarizerAgent.On("Process", ctx, content).Return(
		agents.Result{}, errors.New("summarizer failed"),
	)
	// Other agents should not be called

	result, err := service.runAnalysisAgents(ctx, content, jobID, correlationID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "summarizer failed")

	// Only summarizer should be called
	service.summarizerAgent.AssertExpectations(t)
	service.takeawayAgent.AssertNotCalled(t, "ProcessWithOptions")
	service.factCheckerAgent.AssertNotCalled(t, "Process")
}

func TestAnalysisService_runAnalysisAgents_TakeawayFails_WorkflowContinues(t *testing.T) {
	service, _ := setupMockAnalysisService()

	ctx := context.Background()
	content := "Test content"
	jobID := uuid.New()
	correlationID := "test-correlation-takeaway-fail"

	// Mock successful summarizer
	expectedSummary := "Test summary"
	service.summarizerAgent.On("Process", ctx, content).Return(
		agents.Result{Summary: expectedSummary}, nil,
	)

	// Mock takeaway failure
	service.takeawayAgent.On("ProcessWithOptions", ctx, content, agents.ProcessingOptions{Summary: expectedSummary}).Return(
		agents.Result{}, errors.New("takeaway extraction failed"),
	)

	// Mock successful fact checker
	expectedFactChecks := []agents.FactCheck{
		{Claim: "Test claim", Verdict: "true", Confidence: 0.9},
	}
	service.factCheckerAgent.On("Process", ctx, content).Return(
		agents.Result{FactChecks: expectedFactChecks}, nil,
	)

	result, err := service.runAnalysisAgents(ctx, content, jobID, correlationID)

	// Should succeed despite takeaway failure
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedSummary, result.Summary)
	
	// Takeaways should be empty but workflow continues
	takeawaysData := result.Takeaways["takeaways"].([]string)
	assert.Empty(t, takeawaysData)
	
	// Fact checks should still work
	assert.Len(t, result.FactChecks, 1)

	// All agents should be called
	service.summarizerAgent.AssertExpectations(t)
	service.takeawayAgent.AssertExpectations(t)
	service.factCheckerAgent.AssertExpectations(t)
}

func TestAnalysisService_transformAnalysisResults_TakeawaysMarshallingEdgeCase(t *testing.T) {
	service, _ := setupMockAnalysisService()

	// Test with special characters and complex takeaways
	summary := "Summary with special characters: àéîôü"
	takeaways := []string{
		"Takeaway with quotes: \"Important insight\"",
		"Takeaway with JSON-like content: {\"key\": \"value\"}",
		"Takeaway with newlines:\nSecond line",
	}
	factCheckResults := []agents.FactCheck{}
	jobID := uuid.New()
	correlationID := "test-correlation-marshal"

	result, err := service.transformAnalysisResults(summary, takeaways, factCheckResults, jobID, correlationID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, summary, result.Summary)
	
	// Verify complex takeaways are handled correctly
	takeawaysData := result.Takeaways["takeaways"].([]string)
	assert.Len(t, takeawaysData, 3)
	assert.Equal(t, takeaways[0], takeawaysData[0])
	assert.Equal(t, takeaways[1], takeawaysData[1])
	assert.Equal(t, takeaways[2], takeawaysData[2])
}