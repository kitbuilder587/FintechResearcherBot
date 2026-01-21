package service

import (
	"context"

	"github.com/kitbuilder587/fintech-bot/internal/agent"
)

type CoordinatorAdapter struct {
	coordinator *agent.Coordinator
}

func NewCoordinatorAdapter(coordinator *agent.Coordinator) *CoordinatorAdapter {
	return &CoordinatorAdapter{
		coordinator: coordinator,
	}
}

func (a *CoordinatorAdapter) Process(ctx context.Context, req AgentCoordinatorRequest) (*CoordinatorResponse, error) {
	agentReq := agent.AgentRequest{
		Question:      req.Question,
		SearchResults: req.SearchResults,
		Context:       req.Context,
		Strategy:      req.Strategy,
	}

	resp, err := a.coordinator.Process(ctx, agentReq)
	if err != nil {
		return nil, err
	}

	return &CoordinatorResponse{
		FinalAnswer: resp.FinalAnswer,
		AgentsUsed:  resp.AgentsUsed,
	}, nil
}
