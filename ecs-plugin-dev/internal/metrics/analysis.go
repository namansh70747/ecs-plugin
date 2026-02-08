package metrics

import (
	"sync"
	"time"
)

type DeploymentAnalysis struct {
	TotalDeployments   int64
	SuccessfulDeploys  int64
	FailedDeploys      int64
	CancelledDeploys   int64
	SuccessRate        float64
	AverageDuration    time.Duration
	StrategyBreakdown  map[string]int64
	ErrorBreakdown     map[string]int64
	LastDeploymentTime time.Time
	FastestDeployment  time.Duration
	SlowestDeployment  time.Duration
}

type DeploymentInsight struct {
	DeploymentID string
	Strategy     string
	Duration     time.Duration
	Status       string
	Error        string
	StartTime    time.Time
	EndTime      time.Time
}

type AnalysisEngine struct {
	mu          sync.RWMutex
	insights    []DeploymentInsight
	maxInsights int
}

func NewAnalysisEngine() *AnalysisEngine {
	return &AnalysisEngine{
		insights:    []DeploymentInsight{},
		maxInsights: 1000,
	}
}

func (ae *AnalysisEngine) RecordDeployment(deploymentID, strategy, status, errorMsg string, duration time.Duration, startTime time.Time) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	insight := DeploymentInsight{
		DeploymentID: deploymentID,
		Strategy:     strategy,
		Duration:     duration,
		Status:       status,
		Error:        errorMsg,
		StartTime:    startTime,
		EndTime:      startTime.Add(duration),
	}

	ae.insights = append(ae.insights, insight)

	// Keep only last N insights
	if len(ae.insights) > ae.maxInsights {
		ae.insights = ae.insights[len(ae.insights)-ae.maxInsights:]
	}
}

func (ae *AnalysisEngine) GetAnalysis() *DeploymentAnalysis {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	analysis := &DeploymentAnalysis{
		StrategyBreakdown: make(map[string]int64),
		ErrorBreakdown:    make(map[string]int64),
	}

	if len(ae.insights) == 0 {
		return analysis
	}

	var totalDuration time.Duration
	analysis.FastestDeployment = time.Hour * 24
	analysis.SlowestDeployment = 0

	for _, insight := range ae.insights {
		analysis.TotalDeployments++

		// Status breakdown
		switch insight.Status {
		case "success":
			analysis.SuccessfulDeploys++
		case "failed":
			analysis.FailedDeploys++
		case "cancelled":
			analysis.CancelledDeploys++
		}

		// Strategy breakdown
		analysis.StrategyBreakdown[insight.Strategy]++

		// Error breakdown
		if insight.Error != "" {
			analysis.ErrorBreakdown[insight.Error]++
		}

		// Duration stats
		totalDuration += insight.Duration
		if insight.Duration < analysis.FastestDeployment {
			analysis.FastestDeployment = insight.Duration
		}
		if insight.Duration > analysis.SlowestDeployment {
			analysis.SlowestDeployment = insight.Duration
		}

		// Last deployment time
		if insight.EndTime.After(analysis.LastDeploymentTime) {
			analysis.LastDeploymentTime = insight.EndTime
		}
	}

	// Calculate success rate
	if analysis.TotalDeployments > 0 {
		analysis.SuccessRate = float64(analysis.SuccessfulDeploys) / float64(analysis.TotalDeployments) * 100
		analysis.AverageDuration = totalDuration / time.Duration(analysis.TotalDeployments)
	}

	return analysis
}

func (ae *AnalysisEngine) GetRecentInsights(limit int) []DeploymentInsight {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	if limit <= 0 || limit > len(ae.insights) {
		limit = len(ae.insights)
	}

	start := len(ae.insights) - limit
	if start < 0 {
		start = 0
	}

	result := make([]DeploymentInsight, limit)
	copy(result, ae.insights[start:])
	return result
}

func (ae *AnalysisEngine) GetInsightsByStrategy(strategy string) []DeploymentInsight {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	var result []DeploymentInsight
	for _, insight := range ae.insights {
		if insight.Strategy == strategy {
			result = append(result, insight)
		}
	}
	return result
}

func (ae *AnalysisEngine) GetFailedDeployments() []DeploymentInsight {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	var result []DeploymentInsight
	for _, insight := range ae.insights {
		if insight.Status == "failed" {
			result = append(result, insight)
		}
	}
	return result
}

var globalAnalysisEngine = NewAnalysisEngine()

func GetGlobalAnalysisEngine() *AnalysisEngine {
	return globalAnalysisEngine
}
