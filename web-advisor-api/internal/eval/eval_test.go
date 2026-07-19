package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/amin/web-based-ai-project-advisor/internal/services/requirements"
)

type evalCase struct {
	ID                    string   `json:"id"`
	Idea                  string   `json:"idea"`
	ExpectedDomain        string   `json:"expected_domain"`
	ExpectedTaskContains  string   `json:"expected_task_contains"`
	ExpectedRepoKeywords  []string `json:"expected_repo_keywords"`
	ExpectedModelTasks    []string `json:"expected_model_tasks"`
}

func TestRequirementExtractionBenchmark(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "eval_cases.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Cases []evalCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}

	ex := requirements.New(nil)
	for _, c := range payload.Cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			req, err := ex.Extract(context.Background(), c.Idea, nil)
			if err != nil {
				t.Fatal(err)
			}
			if c.ExpectedDomain != "general" && req.Domain != c.ExpectedDomain {
				t.Fatalf("domain: got %q want %q", req.Domain, c.ExpectedDomain)
			}
			if !strings.Contains(strings.ToLower(req.Task), strings.ToLower(c.ExpectedTaskContains)) {
				t.Fatalf("task %q missing %q", req.Task, c.ExpectedTaskContains)
			}
		})
	}
}
