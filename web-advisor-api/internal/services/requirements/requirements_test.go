package requirements_test

import (
	"context"
	"testing"

	"github.com/amin/web-based-ai-project-advisor/internal/services/requirements"
)

func TestHeuristicExtractMedicalChatbot(t *testing.T) {
	ex := requirements.New(nil)
	req, err := ex.Extract(context.Background(), "I want to build a medical chatbot", nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.Domain != "healthcare" {
		t.Fatalf("expected healthcare domain, got %s", req.Domain)
	}
	if req.Task != "question answering" {
		t.Fatalf("expected question answering, got %s", req.Task)
	}
}

func TestHeuristicExtractAgent(t *testing.T) {
	ex := requirements.New(nil)
	req, err := ex.Extract(context.Background(), "I want to build autonomous coding agent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.Task != "agentic workflow" {
		t.Fatalf("expected agentic workflow, got %s", req.Task)
	}
}
