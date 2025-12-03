package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/utils"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// MIRARCAHandler handles MIRA-powered RCA explanation endpoints.
type MIRARCAHandler struct {
	miraService services.MIRAService
	logger      logging.Logger
	config      config.MIRAConfig
}

// NewMIRARCAHandler creates a new MIRA RCA handler.
func NewMIRARCAHandler(miraService services.MIRAService, miraCfg config.MIRAConfig, logger corelogger.Logger) *MIRARCAHandler {
	return &MIRARCAHandler{
		miraService: miraService,
		logger:      logging.FromCoreLogger(logger),
		config:      miraCfg,
	}
}

// MIRARCARequest represents the request payload for MIRA RCA analysis.
// It expects the full RCA response from /api/v1/unified/rca.
type MIRARCARequest struct {
	RCAData models.RCAResponse `json:"rcaData" binding:"required"`
}

// HandleMIRARCAAnalyze handles POST /api/v1/mira/rca_analyze.
// It takes an RCA response, converts it to TOON format, and generates
// a non-technical explanation using MIRA.
//
// @Summary Generate non-technical RCA explanation
// @Description Translates technical RCA output into non-technical narrative using AI (MIRA - Mirador Intelligent Research Assistant). Supports multiple providers: OpenAI, Anthropic, vLLM, Ollama. Responses are cached for cost optimization.
// @Tags mira
// @Accept json
// @Produce json
// @Param rcaData body MIRARCARequest true "RCA response payload from /api/v1/unified/rca"
// @Success 200 {object} map[string]interface{} "status: success, data: {explanation, tokensUsed, provider, model, generatedAt, cached, generationTimeMs}"
// @Failure 400 {object} map[string]interface{} "status: error, error: invalid_json_payload | invalid_rca_data"
// @Failure 500 {object} map[string]interface{} "status: error, error: toon_conversion_failed | prompt_rendering_failed | mira_generation_failed"
// @Failure 429 {object} map[string]interface{} "status: error, error: rate_limit_exceeded"
// @Router /api/v1/mira/rca_analyze [post]
func (h *MIRARCAHandler) HandleMIRARCAAnalyze(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.config.Timeout)
	defer cancel()

	// Read request body
	bodyData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "failed_to_read_body",
		})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyData))

	// Parse request
	var req MIRARCARequest
	if err := json.Unmarshal(bodyData, &req); err != nil {
		h.logger.Error("Failed to parse MIRA RCA request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "invalid_json_payload",
		})
		return
	}

	// Validate RCA response structure
	if err := utils.ValidateRCAResponse(&req.RCAData); err != nil {
		h.logger.Error("RCA response validation failed", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("invalid_rca_data: %v", err),
		})
		return
	}

	// Convert RCA to TOON format
	toonData, err := utils.ConvertRCAToTOON(&req.RCAData)
	if err != nil {
		h.logger.Error("Failed to convert RCA to TOON", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "toon_conversion_failed",
		})
		return
	}

	// Extract key information for prompt template
	promptData := h.ExtractPromptData(&req.RCAData, toonData)

	// Render base prompt from template
	basePrompt, err := h.RenderPrompt(promptData)
	if err != nil {
		h.logger.Error("Failed to render prompt", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "prompt_rendering_failed",
		})
		return
	}

	h.logger.Info("Generating MIRA explanation with chunked prompts",
		"provider", h.miraService.GetProviderName(),
		"model", h.miraService.GetModelName(),
		"toon_length", len(toonData),
		"prompt_length", len(basePrompt))

	// Split RCA data into chunks for small context models (llama3.2:1b = 4096 tokens)
	// NOTE(MIRA-CHUNKING): Break large RCA responses into multiple prompts to stay within
	// model context limits. Each chunk is processed separately and responses are cached
	// in Valkey, then stitched together for final response.
	startTime := time.Now()
	explanation, totalTokens, cached, err := h.GenerateChunkedExplanation(ctx, &req.RCAData, basePrompt)
	if err != nil {
		h.logger.Error("MIRA chunked explanation generation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "mira_generation_failed",
		})
		return
	}
	generationTime := time.Since(startTime)

	h.logger.Info("MIRA explanation generated successfully",
		"provider", h.miraService.GetProviderName(),
		"model", h.miraService.GetModelName(),
		"tokens_used", totalTokens,
		"cached", cached,
		"duration_ms", generationTime.Milliseconds())

	// Return response
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"explanation":      explanation,
			"tokensUsed":       totalTokens,
			"provider":         h.miraService.GetProviderName(),
			"model":            h.miraService.GetModelName(),
			"generatedAt":      time.Now().Format(time.RFC3339),
			"cached":           cached,
			"generationTimeMs": generationTime.Milliseconds(),
		},
	})
}

// ExtractPromptData extracts key information from RCA response for prompt template.
func (h *MIRARCAHandler) ExtractPromptData(rca *models.RCAResponse, toonData string) map[string]interface{} {
	data := map[string]interface{}{
		"TOONData": toonData,
	}

	if rca.Data != nil {
		// Extract impact KPI info
		if rca.Data.Impact != nil {
			data["ImpactService"] = rca.Data.Impact.ImpactService
			data["MetricName"] = rca.Data.Impact.MetricName
			data["Severity"] = fmt.Sprintf("%.2f", rca.Data.Impact.Severity)
			data["AnomalyScore"] = "N/A" // Not available in IncidentContextDTO
		}

		// Extract root cause info
		if rca.Data.RootCause != nil {
			data["RootCauseService"] = rca.Data.RootCause.Service
			data["RootCauseComponent"] = rca.Data.RootCause.Component

			// Build evidence string from EvidenceRefDTO
			var evidenceStrs []string
			for _, ev := range rca.Data.RootCause.Evidence {
				evidenceStrs = append(evidenceStrs, fmt.Sprintf("%s: %s", ev.Type, ev.Details))
			}
			data["RootCauseEvidence"] = strings.Join(evidenceStrs, "; ")
		}

		// Extract chain count
		data["ChainCount"] = len(rca.Data.Chains)

		// Extract top chain
		if len(rca.Data.Chains) > 0 {
			chain := rca.Data.Chains[0]
			data["TopChainScore"] = fmt.Sprintf("%.2f", chain.Score)
			data["TopChainHops"] = chain.DurationHops

			// Build readable chain path from steps
			var chainPath []string
			for _, step := range chain.Steps {
				chainPath = append(chainPath, fmt.Sprintf("%s (%s)", step.KPIName, step.Service))
			}
			data["TopChainPath"] = strings.Join(chainPath, " → ")
		}

		// Extract time window from Impact
		if rca.Data.Impact != nil {
			data["TimeWindowStart"] = rca.Data.Impact.TimeStartStr
			data["TimeWindowEnd"] = rca.Data.Impact.TimeEndStr
		}
	}

	return data
}

// RenderPrompt renders the prompt template with extracted data.
func (h *MIRARCAHandler) RenderPrompt(data map[string]interface{}) (string, error) {
	tmpl, err := template.New("mira_prompt").Parse(h.config.PromptTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse prompt template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute prompt template: %w", err)
	}

	return buf.String(), nil
}

// GenerateChunkedExplanation breaks large RCA data into chunks, processes each chunk
// separately to stay within model context limits, caches intermediate results in Valkey,
// and stitches the final explanation together. Each chunk includes context from previous
// responses to maintain coherence.
func (h *MIRARCAHandler) GenerateChunkedExplanation(ctx context.Context, rca *models.RCAResponse, basePrompt string) (string, int, bool, error) {
	const maxTokensPerChunk = 3000 // Leave buffer for response tokens (4096 - 1096 = 3000)

	// Step 1: Split RCA into logical chunks
	chunks := h.splitRCAIntoChunks(rca, maxTokensPerChunk)

	h.logger.Info("Split RCA into chunks",
		"total_chunks", len(chunks),
		"max_tokens_per_chunk", maxTokensPerChunk)

	// Step 2: Process each chunk sequentially, passing context from previous responses
	var explanations []string
	var conversationContext strings.Builder
	totalTokens := 0
	allCached := true

	for i, chunk := range chunks {
		// Build prompt with previous context
		chunkPrompt := h.buildChunkPrompt(basePrompt, chunk, i+1, len(chunks), conversationContext.String())

		h.logger.Debug("Processing chunk",
			"chunk_number", i+1,
			"total_chunks", len(chunks),
			"prompt_length", len(chunkPrompt),
			"has_context", len(conversationContext.String()) > 0)

		// Generate explanation for this chunk
		miraResponse, err := h.miraService.GenerateExplanation(ctx, chunkPrompt)
		if err != nil {
			return "", 0, false, fmt.Errorf("failed to generate explanation for chunk %d: %w", i+1, err)
		}

		explanations = append(explanations, miraResponse.Explanation)
		totalTokens += miraResponse.TokensUsed
		allCached = allCached && miraResponse.Cached

		// Add this response to conversation context for next chunk
		conversationContext.WriteString(fmt.Sprintf("\n[Previous Analysis Part %d]: %s\n", i+1, miraResponse.Explanation))

		h.logger.Debug("Chunk processed",
			"chunk_number", i+1,
			"tokens_used", miraResponse.TokensUsed,
			"cached", miraResponse.Cached,
			"response_length", len(miraResponse.Explanation))
	}

	// Step 3: Stitch explanations together
	finalExplanation := h.stitchExplanations(explanations, len(chunks))

	return finalExplanation, totalTokens, allCached, nil
}

// splitRCAIntoChunks divides the RCA response into smaller chunks that fit within token limits.
func (h *MIRARCAHandler) splitRCAIntoChunks(rca *models.RCAResponse, maxTokensPerChunk int) []map[string]interface{} {
	var chunks []map[string]interface{}

	// Always include impact in first chunk
	chunk1 := map[string]interface{}{
		"type":      "impact_and_root_cause",
		"impact":    rca.Data.Impact,
		"rootCause": rca.Data.RootCause,
		"timeRings": rca.Data.TimeRings,
	}
	chunks = append(chunks, chunk1)

	// Split chains into separate chunks (each chain can be explained independently)
	if len(rca.Data.Chains) > 0 {
		// Dynamically calculate chains per chunk based on total chains
		// Strategy: Minimize chunks while staying within model context limits
		// For 1-5 chains: Include all in one chunk (better context)
		// For 6-10 chains: 5 per chunk (2 chunks total)
		// For 11+ chains: 5 per chunk (multiple chunks)
		totalChains := len(rca.Data.Chains)
		var chainsPerChunk int

		switch {
		case totalChains <= 5:
			chainsPerChunk = totalChains // All chains in one chunk for better coherence
		case totalChains <= 10:
			chainsPerChunk = 5 // 2 chunks: 5+5 or 5+remaining
		default:
			chainsPerChunk = 5 // For 11+ chains, use 5 per chunk
		}

		h.logger.Info("Splitting chains into chunks",
			"total_chains", totalChains,
			"chains_per_chunk", chainsPerChunk)

		for i := 0; i < len(rca.Data.Chains); i += chainsPerChunk {
			end := i + chainsPerChunk
			if end > len(rca.Data.Chains) {
				end = len(rca.Data.Chains)
			}

			chunk := map[string]interface{}{
				"type":       "chains",
				"chains":     rca.Data.Chains[i:end],
				"chainRange": fmt.Sprintf("%d-%d of %d", i+1, end, len(rca.Data.Chains)),
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// buildChunkPrompt creates a focused prompt for a specific chunk of RCA data,
// including context from previously analyzed chunks to maintain coherence.
func (h *MIRARCAHandler) buildChunkPrompt(basePrompt string, chunk map[string]interface{}, chunkNum int, totalChunks int, previousContext string) string {
	chunkType := chunk["type"].(string)

	var promptBuilder strings.Builder

	if chunkNum == 1 {
		// First chunk gets the base instructions
		promptBuilder.WriteString("You are MIRA (Mirador Intelligent Research Assistant). ")
		promptBuilder.WriteString("This is a multi-part explanation. Focus on clarity and simplicity.\n\n")
	} else {
		promptBuilder.WriteString(fmt.Sprintf("Continuing MIRA analysis (part %d of %d).\n\n", chunkNum, totalChunks))

		// Include previous analysis for context
		if previousContext != "" {
			promptBuilder.WriteString("PREVIOUS ANALYSIS (for context):\n")
			promptBuilder.WriteString(previousContext)
			promptBuilder.WriteString("\n---\n\n")
			promptBuilder.WriteString("Now continue the analysis with the following information:\n\n")
		}
	}

	switch chunkType {
	case "impact_and_root_cause":
		promptBuilder.WriteString("Part 1: IMPACT & ROOT CAUSE SUMMARY\n")
		promptBuilder.WriteString("Explain WHAT happened, WHEN it happened, and the PRIMARY root cause.\n\n")
		promptBuilder.WriteString("IMPORTANT CONTEXT:\n")
		promptBuilder.WriteString("- This analysis follows the '5 Whys' methodology: each causal chain traces back through up to 5 levels of causation\n")
		promptBuilder.WriteString("- KPIs are classified into layers: IMPACT layer (user-facing symptoms) and CAUSE layer (underlying technical issues)\n")
		promptBuilder.WriteString("- The Impact KPI represents what users/business experienced\n")
		promptBuilder.WriteString("- The Root Cause KPI is the deepest 'Why' we found (Why #5 in the chain)\n\n")

		// Convert chunk data to JSON for structured input
		if impact, ok := chunk["impact"]; ok {
			impactJSON, _ := json.MarshalIndent(impact, "", "  ")
			promptBuilder.WriteString(fmt.Sprintf("Impact (IMPACT Layer - What users saw):\n%s\n\n", impactJSON))
		}
		if rootCause, ok := chunk["rootCause"]; ok {
			rcJSON, _ := json.MarshalIndent(rootCause, "", "  ")
			promptBuilder.WriteString(fmt.Sprintf("Root Cause (CAUSE Layer - The deepest 'Why'):\n%s\n\n", rcJSON))
		}

		promptBuilder.WriteString("Provide a clear explanation that:\n")
		promptBuilder.WriteString("1. Describes the IMPACT (what business/users experienced)\n")
		promptBuilder.WriteString("2. Identifies the ROOT CAUSE (the fundamental issue at Why #5)\n")
		promptBuilder.WriteString("3. Uses simple language - no technical jargon\n\n")

	case "chains":
		chainRange := chunk["chainRange"].(string)
		promptBuilder.WriteString(fmt.Sprintf("Part %d: CAUSAL CHAINS - THE '5 WHYS' PROPAGATION (%s)\n", chunkNum, chainRange))
		promptBuilder.WriteString("\nEach chain shows the '5 Whys' progression from Impact (Why #1) to Root Cause (up to Why #5).\n")
		promptBuilder.WriteString("The 'whyIndex' field shows the depth: 1 = user-facing impact, 5 = deepest root cause.\n\n")
		promptBuilder.WriteString("KPI Layer Guide:\n")
		promptBuilder.WriteString("- IMPACT layer KPIs (whyIndex 1-2): What users/business experienced\n")
		promptBuilder.WriteString("- CAUSE layer KPIs (whyIndex 3-5): Technical issues that triggered the impact\n\n")

		if chains, ok := chunk["chains"]; ok {
			chainsJSON, _ := json.MarshalIndent(chains, "", "  ")
			promptBuilder.WriteString(fmt.Sprintf("Chains (each step has 'whyIndex' showing its depth in the '5 Whys'):\n%s\n\n", chainsJSON))
		}

		promptBuilder.WriteString("For each chain, explain:\n")
		promptBuilder.WriteString("1. The '5 Whys' progression: Why #1 (impact) → Why #2 → ... → Why #5 (root cause)\n")
		promptBuilder.WriteString("2. How IMPACT layer KPIs (user symptoms) connect to CAUSE layer KPIs (technical issues)\n")
		promptBuilder.WriteString("3. The propagation path in simple, business-friendly language\n\n")
	}

	return promptBuilder.String()
}

// stitchExplanations combines chunk explanations into a coherent final response.
func (h *MIRARCAHandler) stitchExplanations(explanations []string, totalChunks int) string {
	var final strings.Builder

	final.WriteString("# Root Cause Analysis Summary\n\n")

	for i, explanation := range explanations {
		if i > 0 {
			final.WriteString("\n\n")
		}
		final.WriteString(explanation)
	}

	final.WriteString(fmt.Sprintf("\n\n---\n*Analysis generated from %d data segments*", totalChunks))

	return final.String()
}
