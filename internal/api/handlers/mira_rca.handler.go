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

		// Extract time ring definitions for temporal context
		if rca.Data.TimeRings != nil && len(rca.Data.TimeRings.Definitions) > 0 {
			var ringDefs []string
			for ring, def := range rca.Data.TimeRings.Definitions {
				if ring != "R_OUT_OF_SCOPE" {
					ringDefs = append(ringDefs, fmt.Sprintf("%s=%s (%s)", ring, def.Duration, def.Description))
				}
			}
			data["TimeRings"] = strings.Join(ringDefs, ", ")

			// Extract peak time if available
			if len(rca.Data.TimeRings.PerChain) > 0 {
				data["PeakTime"] = rca.Data.TimeRings.PerChain[0].PeakTime
			}
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
	const maxTokensPerChunk = 2800 // Reduced from 3000 to provide more buffer (4096 - 1296 = 2800)

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

		// Estimate token count (rough approximation: 1 token ≈ 4 characters)
		estimatedTokens := len(chunkPrompt) / 4

		h.logger.Debug("Processing chunk",
			"chunk_number", i+1,
			"total_chunks", len(chunks),
			"prompt_length", len(chunkPrompt),
			"estimated_tokens", estimatedTokens,
			"has_context", len(conversationContext.String()) > 0)

		if estimatedTokens > maxTokensPerChunk {
			h.logger.Warn("Chunk prompt may exceed token limit",
				"chunk_number", i+1,
				"estimated_tokens", estimatedTokens,
				"max_tokens", maxTokensPerChunk,
				"overage", estimatedTokens-maxTokensPerChunk)
		}

		// Generate explanation for this chunk
		miraResponse, err := h.miraService.GenerateExplanation(ctx, chunkPrompt)
		if err != nil {
			return "", 0, false, fmt.Errorf("failed to generate explanation for chunk %d: %w", i+1, err)
		}

		explanations = append(explanations, miraResponse.Explanation)
		totalTokens += miraResponse.TokensUsed
		allCached = allCached && miraResponse.Cached

		// Add this response to conversation context for next chunk (truncate to save tokens)
		// Increased from 300 to 600 chars to preserve more narrative detail across chunks
		truncatedResponse := miraResponse.Explanation
		if len(truncatedResponse) > 600 {
			truncatedResponse = truncatedResponse[:600] + "..."
		}
		conversationContext.WriteString(fmt.Sprintf("\n[Part %d]: %s\n", i+1, truncatedResponse))

		h.logger.Debug("Chunk processed",
			"chunk_number", i+1,
			"tokens_used", miraResponse.TokensUsed,
			"cached", miraResponse.Cached,
			"response_length", len(miraResponse.Explanation))
	}

	// Step 3: Stitch explanations together
	stitchedExplanation := h.stitchExplanations(explanations, len(chunks))

	// Step 4: Optional final synthesis for multi-chunk responses
	// If we had multiple chunks (complex RCA), send all chunk responses back to AI
	// for a final comprehensive synthesis that ensures narrative coherence
	if len(chunks) > 1 {
		h.logger.Info("Performing final synthesis",
			"total_chunks", len(chunks),
			"explanation_length", len(stitchedExplanation))

		finalExplanation, synthesisTokens, synthesisCached, err := h.synthesizeFinalReport(ctx, explanations, rca)
		if err != nil {
			h.logger.Warn("Final synthesis failed, using stitched explanation",
				"error", err)
			// Fallback to stitched explanation if synthesis fails
			return stitchedExplanation, totalTokens, allCached, nil
		}

		totalTokens += synthesisTokens
		allCached = allCached && synthesisCached

		h.logger.Info("Final synthesis completed",
			"synthesis_tokens", synthesisTokens,
			"synthesis_cached", synthesisCached,
			"final_length", len(finalExplanation))

		return finalExplanation, totalTokens, allCached, nil
	}

	return stitchedExplanation, totalTokens, allCached, nil
}

// synthesizeFinalReport takes all chunk explanations and performs a final AI synthesis
// to create a comprehensive, coherent report. This ensures that multi-chunk analyses
// maintain narrative flow and don't lose important details.
func (h *MIRARCAHandler) synthesizeFinalReport(ctx context.Context, chunkExplanations []string, rca *models.RCAResponse) (string, int, bool, error) {
	var promptBuilder strings.Builder

	promptBuilder.WriteString("You are MIRA. You have analyzed a complex incident in multiple parts. ")
	promptBuilder.WriteString("Your task now is to synthesize these parts into ONE comprehensive, detailed final report.\\n\\n")

	promptBuilder.WriteString("=== ANALYSIS PARTS ===\\n\\n")
	for i, explanation := range chunkExplanations {
		promptBuilder.WriteString(fmt.Sprintf("## Part %d:\\n%s\\n\\n", i+1, explanation))
	}

	promptBuilder.WriteString("\\n=== YOUR TASK ===\\n\\n")
	promptBuilder.WriteString("Create a COMPREHENSIVE final report that:\\n")
	promptBuilder.WriteString("1. Combines all information from the parts above into a coherent narrative\\n")
	promptBuilder.WriteString("2. Preserves ALL service names, KPI names, metrics, scores, and data points\\n")
	promptBuilder.WriteString("3. Maintains the detailed, verbose style - do not summarize or condense\\n")
	promptBuilder.WriteString("4. Creates a logical flow from impact → causal chains → root cause\\n")
	promptBuilder.WriteString("5. Includes all evidence and correlation data\\n")
	promptBuilder.WriteString("6. Ends with comprehensive prevention recommendations\\n\\n")

	promptBuilder.WriteString("Structure your report with clear sections:\\n")
	promptBuilder.WriteString("- Executive Summary (comprehensive, not brief)\\n")
	promptBuilder.WriteString("- Detailed Incident Description\\n")
	promptBuilder.WriteString("- User Impact Analysis\\n")
	promptBuilder.WriteString("- Complete Causal Chain Analysis\\n")
	promptBuilder.WriteString("- Root Cause Explanation\\n")
	promptBuilder.WriteString("- Supporting Evidence & Data\\n")
	promptBuilder.WriteString("- Prevention & Remediation Steps\\n\\n")

	promptBuilder.WriteString("IMPORTANT: This is the FINAL report - be thorough and comprehensive. Include ALL details from the parts above.\\n")

	synthesisPrompt := promptBuilder.String()

	// Estimate tokens
	estimatedTokens := len(synthesisPrompt) / 4
	h.logger.Debug("Synthesis prompt prepared",
		"prompt_length", len(synthesisPrompt),
		"estimated_tokens", estimatedTokens)

	// Check if synthesis prompt exceeds token limit
	const maxSynthesisTokens = 3500 // Leave room for response
	if estimatedTokens > maxSynthesisTokens {
		h.logger.Warn("Synthesis prompt too large, truncating chunk explanations",
			"estimated_tokens", estimatedTokens,
			"max_tokens", maxSynthesisTokens)

		// Truncate each chunk explanation proportionally
		maxCharsPerChunk := (maxSynthesisTokens * 4) / len(chunkExplanations)
		var truncatedBuilder strings.Builder
		truncatedBuilder.WriteString("You are MIRA. You have analyzed a complex incident in multiple parts. ")
		truncatedBuilder.WriteString("Synthesize these parts into ONE comprehensive final report.\\n\\n")

		for i, explanation := range chunkExplanations {
			truncated := explanation
			if len(truncated) > maxCharsPerChunk {
				truncated = truncated[:maxCharsPerChunk] + "... [truncated for token limit]"
			}
			truncatedBuilder.WriteString(fmt.Sprintf("Part %d: %s\\n\\n", i+1, truncated))
		}

		truncatedBuilder.WriteString("Create a comprehensive report preserving ALL details mentioned above.\\n")
		synthesisPrompt = truncatedBuilder.String()
	}

	// Generate final synthesis
	miraResponse, err := h.miraService.GenerateExplanation(ctx, synthesisPrompt)
	if err != nil {
		return "", 0, false, fmt.Errorf("synthesis generation failed: %w", err)
	}

	h.logger.Debug("Synthesis generated",
		"tokens_used", miraResponse.TokensUsed,
		"cached", miraResponse.Cached,
		"response_length", len(miraResponse.Explanation))

	return miraResponse.Explanation, miraResponse.TokensUsed, miraResponse.Cached, nil
}

// splitRCAIntoChunks divides the RCA response into smaller chunks that fit within token limits.
// Only includes essential fields to minimize token usage.
func (h *MIRARCAHandler) splitRCAIntoChunks(rca *models.RCAResponse, maxTokensPerChunk int) []map[string]interface{} {
	var chunks []map[string]interface{}

	// Chunk 1: Impact and root cause (essential summary data only)
	chunk1 := map[string]interface{}{
		"type": "impact_and_root_cause",
	}

	// Add time ring context to first chunk
	if rca.Data.TimeRings != nil {
		var ringDefs []string
		for ring, def := range rca.Data.TimeRings.Definitions {
			if ring != "R_OUT_OF_SCOPE" {
				ringDefs = append(ringDefs, fmt.Sprintf("%s=%s", ring, def.Duration))
			}
		}
		if len(ringDefs) > 0 {
			chunk1["timeRings"] = strings.Join(ringDefs, ", ")
		}

		if len(rca.Data.TimeRings.PerChain) > 0 {
			chunk1["peakTime"] = rca.Data.TimeRings.PerChain[0].PeakTime
		}
	}

	// Extract only essential impact fields
	if rca.Data.Impact != nil {
		chunk1["impact"] = map[string]interface{}{
			"service":   rca.Data.Impact.ImpactService,
			"metric":    rca.Data.Impact.MetricName,
			"severity":  rca.Data.Impact.Severity,
			"timeStart": rca.Data.Impact.TimeStartStr,
			"timeEnd":   rca.Data.Impact.TimeEndStr,
		}
	}

	// Extract only essential root cause fields
	if rca.Data.RootCause != nil {
		chunk1["rootCause"] = map[string]interface{}{
			"service":   rca.Data.RootCause.Service,
			"component": rca.Data.RootCause.Component,
			"score":     rca.Data.RootCause.Score,
			"summary":   rca.Data.RootCause.Summary,
		}
	}

	chunks = append(chunks, chunk1)

	// Split chains into separate chunks (include only key fields per step)
	if len(rca.Data.Chains) > 0 {
		totalChains := len(rca.Data.Chains)
		var chainsPerChunk int

		switch {
		case totalChains <= 5:
			chainsPerChunk = totalChains
		case totalChains <= 10:
			chainsPerChunk = 5
		default:
			chainsPerChunk = 5
		}

		h.logger.Info("Splitting chains into chunks",
			"total_chains", totalChains,
			"chains_per_chunk", chainsPerChunk)

		for i := 0; i < len(rca.Data.Chains); i += chainsPerChunk {
			end := i + chainsPerChunk
			if end > len(rca.Data.Chains) {
				end = len(rca.Data.Chains)
			}

			// Extract only essential chain fields
			compactChains := make([]map[string]interface{}, 0, end-i)
			for _, chain := range rca.Data.Chains[i:end] {
				compactSteps := make([]map[string]interface{}, 0, len(chain.Steps))
				for _, step := range chain.Steps {
					// Build descriptive KPI/metric information
					// Priority: KPIFormula > KPIName > Summary > Component
					kpiInfo := step.KPIFormula
					if kpiInfo == "" {
						kpiInfo = step.KPIName
					}
					if kpiInfo == "" && step.Summary != "" {
						kpiInfo = step.Summary
					}
					if kpiInfo == "" && step.Component != "" {
						kpiInfo = step.Component
					}

					compactSteps = append(compactSteps, map[string]interface{}{
						"service":  step.Service,
						"kpi":      kpiInfo,
						"whyIndex": step.WhyIndex,
						"score":    step.Score,
						"ring":     step.Ring,
					})
				}
				compactChains = append(compactChains, map[string]interface{}{
					"score": chain.Score,
					"steps": compactSteps,
				})
			}

			chunk := map[string]interface{}{
				"type":       "chains",
				"chains":     compactChains,
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
		promptBuilder.WriteString("MIRA RCA Analysis - Part 1 of " + fmt.Sprintf("%d", totalChunks) + "\n")
		promptBuilder.WriteString("IMPORTANT: Provide a DETAILED, COMPREHENSIVE analysis. Be VERBOSE and include ALL specific details:\n")
		promptBuilder.WriteString("- Mention ALL service names, KPI names, and metric names explicitly\n")
		promptBuilder.WriteString("- Include ALL data points, scores, and correlation values\n")
		promptBuilder.WriteString("- Explain EACH step thoroughly with business context\n")
		promptBuilder.WriteString("- Use plain language but preserve all technical details\n\n")
	} else {
		promptBuilder.WriteString(fmt.Sprintf("\nPart %d/%d - Continue detailed analysis\n", chunkNum, totalChunks))
		if previousContext != "" {
			// Truncate previous context to max 500 chars (increased from 200 for better continuity)
			truncatedContext := previousContext
			if len(truncatedContext) > 500 {
				truncatedContext = truncatedContext[:500] + "..."
			}
			promptBuilder.WriteString("Previous analysis summary: " + truncatedContext + "\n\n")
		}
	}

	// Add time ring context if available (for all chunk types)
	if timeRings, ok := chunk["timeRings"].(string); ok && timeRings != "" {
		promptBuilder.WriteString("Time rings: " + timeRings + "\n")
	}
	if peakTime, ok := chunk["peakTime"].(string); ok && peakTime != "" {
		promptBuilder.WriteString("Peak: " + peakTime + "\n")
	}
	if len(chunk) > 2 { // Has time context
		promptBuilder.WriteString("\n")
	}

	switch chunkType {
	case "impact_and_root_cause":
		promptBuilder.WriteString("=== IMPACT & ROOT CAUSE ANALYSIS ===\n\n")
		promptBuilder.WriteString("Context: This is a 5-Whys analysis where Why#1 represents user-visible impact and Why#5 represents the technical root cause.\n\n")

		// Serialize only essential fields
		if impact, ok := chunk["impact"]; ok {
			impactJSON, _ := json.Marshal(impact)
			promptBuilder.WriteString(fmt.Sprintf("IMPACT DATA:\n%s\n\n", impactJSON))
		}
		if rootCause, ok := chunk["rootCause"]; ok {
			rcJSON, _ := json.Marshal(rootCause)
			promptBuilder.WriteString(fmt.Sprintf("ROOT CAUSE DATA:\n%s\n\n", rcJSON))
		}

		promptBuilder.WriteString("REQUIRED OUTPUT - Provide detailed explanation covering:\n")
		promptBuilder.WriteString("1. WHAT HAPPENED: Describe the user-visible symptoms in detail, mentioning the specific service and metric names\n")
		promptBuilder.WriteString("2. BUSINESS IMPACT: Explain the severity and business consequences with specific data points\n")
		promptBuilder.WriteString("3. ROOT CAUSE: Explain the underlying technical issue in plain language, naming the specific service, component, and what went wrong\n")
		promptBuilder.WriteString("4. CORRELATION: Explain how the root cause connects to the impact (include the score if mentioned)\n")
		promptBuilder.WriteString("\nBe comprehensive - include ALL service names, KPI names, and data values mentioned above.\n")

	case "chains":
		chainRange := chunk["chainRange"].(string)
		promptBuilder.WriteString(fmt.Sprintf("=== CAUSAL CHAINS ANALYSIS (%s) ===\n\n", chainRange))
		promptBuilder.WriteString("Context: Each chain shows the causal progression from user impact (whyIndex=1) to technical root cause (whyIndex=5).\n")
		promptBuilder.WriteString("Ring context indicates temporal relationship to the incident peak time.\n\n")

		if chains, ok := chunk["chains"]; ok {
			// Use compact JSON without indentation to save tokens
			chainsJSON, _ := json.Marshal(chains)
			promptBuilder.WriteString(fmt.Sprintf("CHAIN DATA:\n%s\n\n", chainsJSON))
		}

		promptBuilder.WriteString("REQUIRED OUTPUT - For EACH chain, provide detailed explanation:\n")
		promptBuilder.WriteString("1. CHAIN OVERVIEW: Start with the overall score and what this chain represents\n")
		promptBuilder.WriteString("2. STEP-BY-STEP ANALYSIS: For EACH step in the chain, explain:\n")
		promptBuilder.WriteString("   - The specific SERVICE name and KPI name\n")
		promptBuilder.WriteString("   - What this step means in the causal chain (whyIndex context)\n")
		promptBuilder.WriteString("   - The score/correlation strength\n")
		promptBuilder.WriteString("   - The temporal context (ring) and what it means\n")
		promptBuilder.WriteString("3. CAUSAL NARRATIVE: Explain how each step leads to the next in plain language\n")
		promptBuilder.WriteString("4. BUSINESS MEANING: Translate the technical progression into business terms\n")
		promptBuilder.WriteString("\nBe thorough - mention EVERY service, KPI, and data point. Explain the complete causal story.\n")
	}

	return promptBuilder.String()
}

// stitchExplanations combines chunk explanations into a coherent final response.
func (h *MIRARCAHandler) stitchExplanations(explanations []string, totalChunks int) string {
	var final strings.Builder

	final.WriteString("# Comprehensive Root Cause Analysis Report\n\n")
	final.WriteString("## Executive Summary\n")
	final.WriteString("This detailed analysis traces the incident from user-visible impact through the complete causal chain to the technical root cause. ")
	final.WriteString("All service names, KPI metrics, correlation scores, and temporal relationships are included for full traceability.\n\n")

	// Add section headers based on chunk count
	for i, explanation := range explanations {
		if i == 0 {
			final.WriteString("## Impact and Root Cause Overview\n\n")
		} else {
			final.WriteString(fmt.Sprintf("\n\n## Causal Chain Analysis - Part %d\n\n", i))
		}
		final.WriteString(explanation)
	}

	final.WriteString("\n\n---\n")
	final.WriteString("## Technical Notes\n\n")
	final.WriteString(fmt.Sprintf("- This comprehensive analysis was generated from %d data segments\n", totalChunks))
	final.WriteString("- All service names, KPI metrics, and correlation data have been preserved for full technical transparency\n")
	final.WriteString("- Temporal context (time rings) indicates when each anomaly occurred relative to the incident peak\n")

	return final.String()
}
