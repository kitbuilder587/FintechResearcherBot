package service

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

const defaultEvalUserID int64 = 1

type EvalHandler struct {
	query   QueryService
	sources SourceService
	users   UserService
	userID  int64
}

type EvalHandlerConfig struct {
	UserID int64
}

type EvalQueryRequest struct {
	Question string `json:"question"`
	Strategy string `json:"strategy"`
}

type EvalSource struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type EvalQueryResponse struct {
	Question string       `json:"question"`
	Answer   string       `json:"answer"`
	Sources  []EvalSource `json:"sources"`
	Usage    EvalUsage    `json:"usage"`
	Runtime  EvalRuntime  `json:"runtime"`
}

type EvalUsage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	LLMRequests  int     `json:"llm_requests"`
	CostUSD      float64 `json:"cost_usd"`
}

type EvalRuntime struct {
	LLMProvider      string `json:"llm_provider"`
	OpenRouterModel  string `json:"openrouter_model,omitempty"`
	MandatesEnabled  string `json:"agent_mandates_enabled,omitempty"`
	PlannerEnabled   string `json:"agent_planner_enabled,omitempty"`
	StructuredOutput string `json:"agent_structured_output_enabled,omitempty"`
	DisableCritic    string `json:"eval_disable_critic,omitempty"`
	MaxQueries       string `json:"eval_max_queries,omitempty"`
	MaxResults       string `json:"eval_max_results,omitempty"`
	DisableDomains   string `json:"search_domain_filter_disabled,omitempty"`
	DisableCoord     string `json:"agent_coordinator_disabled,omitempty"`
}

func NewEvalHandler(query QueryService, sources SourceService, users UserService, cfg EvalHandlerConfig) *EvalHandler {
	userID := cfg.UserID
	if userID == 0 {
		userID = defaultEvalUserID
	}
	return &EvalHandler{
		query:   query,
		sources: sources,
		users:   users,
		userID:  userID,
	}
}

func (h *EvalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EvalQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if h.users != nil {
		if _, err := h.users.GetOrCreate(r.Context(), h.userID, "eval"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	strategy, ok := evalStrategy(req.Strategy)
	if !ok {
		http.Error(w, "invalid strategy", http.StatusBadRequest)
		return
	}
	if timeout := getEnvInt("EVAL_QUERY_TIMEOUT_SECONDS"); timeout > 0 {
		strategy.TimeoutSeconds = timeout
	}
	if maxQueries := getEnvInt("EVAL_MAX_QUERIES"); maxQueries > 0 {
		strategy.MaxQueries = maxQueries
	}
	if maxResults := getEnvInt("EVAL_MAX_RESULTS"); maxResults > 0 {
		strategy.MaxResults = maxResults
	}
	if envBool("EVAL_DISABLE_CRITIC") {
		strategy.UseCritic = false
	}

	if h.sources != nil {
		if _, err := h.sources.ImportSeed(r.Context(), h.userID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	resp, err := h.query.Process(r.Context(), &domain.QueryRequest{
		UserID:   h.userID,
		Text:     req.Question,
		Strategy: strategy,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := EvalQueryResponse{
		Question: req.Question,
		Answer:   resp.Text,
		Sources:  make([]EvalSource, 0, len(resp.Sources)),
		Usage: EvalUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.TotalTokens,
			LLMRequests:  resp.Usage.LLMRequests,
			CostUSD:      resp.Usage.CostUSD,
		},
		Runtime: EvalRuntime{
			LLMProvider:      os.Getenv("LLM_PROVIDER"),
			OpenRouterModel:  os.Getenv("OPENROUTER_MODEL"),
			MandatesEnabled:  os.Getenv("AGENT_MANDATES_ENABLED"),
			PlannerEnabled:   os.Getenv("AGENT_PLANNER_ENABLED"),
			StructuredOutput: os.Getenv("AGENT_STRUCTURED_OUTPUT_ENABLED"),
			DisableCritic:    os.Getenv("EVAL_DISABLE_CRITIC"),
			MaxQueries:       os.Getenv("EVAL_MAX_QUERIES"),
			MaxResults:       os.Getenv("EVAL_MAX_RESULTS"),
			DisableDomains:   os.Getenv("SEARCH_DOMAIN_FILTER_DISABLED"),
			DisableCoord:     os.Getenv("AGENT_COORDINATOR_DISABLED"),
		},
	}
	for _, src := range resp.Sources {
		out.Sources = append(out.Sources, EvalSource{
			Title:   src.Title,
			URL:     src.URL,
			Content: src.Content,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func evalStrategy(name string) (domain.Strategy, bool) {
	switch domain.StrategyType(name) {
	case domain.StrategyQuick:
		return domain.QuickStrategy(), true
	case "", domain.StrategyStandard:
		return domain.StandardStrategy(), true
	case domain.StrategyDeep:
		return domain.DeepStrategy(), true
	default:
		return domain.Strategy{}, false
	}
}

func getEnvInt(key string) int {
	value := os.Getenv(key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func envBool(key string) bool {
	value := os.Getenv(key)
	return value == "1" || value == "true" || value == "yes"
}
