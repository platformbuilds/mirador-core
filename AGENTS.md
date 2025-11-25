# AGENTS.md

## 0. Purpose & Scope

This document defines how **agents, tests, and SDKs** must be used when working
on Mirador Core, with a focus on:

- Stage-01 **Correlation & RCA** engine behaviour.
- **Testing procedures** and required `make` targets.
- **GPT5 Mini** (coding agent) and **Raptor** (verification agent).
- **Weaviate** client usage and versioning.
- The **canonical folder** for Correlation/RCA engine work:
  - `dev/correlation-RCA-engine/current`

It is the guardrail for both humans and automation (agents, CI, DevX pipelines).

Key references that MUST be consulted before making changes to the Correlation
and RCA engine:

- `dev/correlation-RCA-engine/current/01-correlation-rca-approach-final.md`
- `dev/correlation-RCA-engine/current/01-correlation-rca-code-implementation-final.md`
- `dev/correlation-RCA-engine/current/correlation-rca-action-tracker.yml` (or the
  project-wide tracker at `dev/correlation-RCA-engine/current/correlation-rca-action-tracker.yml`)
- `dev/correlation-RCA-engine/current/dev-README-correlation-rca.md`

Whenever GPT5 Mini or a human developer works on Correlation/RCA code, these
documents in `dev/correlation-RCA-engine/current` MUST be treated as the
immediate context and source of truth.

---

## 1. Agents

### 1.1 GPT5 Mini – Coding Agent

**Role:** Primary coding assistant.

**Allowed tasks:**

- Generate or refactor Go/Python/Helm/config code for:
  - Correlation & RCA engines,
  - HTTP handlers,
  - EngineConfig wiring,
  - Tests (unit, integration, API).
- Produce documentation drafts, comments, and design notes.
- Propose patches that align with:
  - Stage-01 correlation/RCA design,
  - This `AGENTS.md`,
  - Project style guides.

**Required context for Correlation/RCA work:**

Before generating or modifying any code related to the Correlation or RCA
Engine, GPT5 Mini (and human authors) must conceptually load and respect:

- Approach & algorithmic guide:
  - `dev/correlation-RCA-engine/current/01-correlation-rca-approach-final.md`
- Code-level implementation guide:
  - `dev/correlation-RCA-engine/current/01-correlation-rca-code-implementation-final.md`
- Action tracker:
  - `dev/correlation-RCA-engine/current/correlation-rca-action-tracker.yml`
    or `dev/correlation-rca-action-tracker.yml`
- Usage guide:
  - `dev/correlation-RCA-engine/current/README-correlation-rca.md`

In practice, this means: when designing or changing Correlation/RCA behaviour,
always cross-check with these files in `dev/correlation-RCA-engine/current`
before proposing changes.

**Constraints:**

- Must treat the documents above as **non-negotiable contracts** for Stage-01.
- Must not introduce new API fields for `/unified/correlate` or `/unified/rca`
  beyond `{ "startTime", "endTime" }`.
- For Weaviate access, must only modify or add code within approved
  store/repo layers (see Section 4).

### 1.2 Raptor – Verification & Live Testing Agent

**Role:** Code verifier and test runner.

**Allowed tasks:**

- Run **static** checks (linting, formatting).
- Run **unit** and **integration** tests.
- Execute **API tests** against a locally running Mirador Core.
- Provide structured feedback:
  - Failing tests,
  - Stack traces,
  - Logs,
  - Suggested fix points.

**Required commands:**

Raptor must use these targets when verifying correlation/RCA changes:

```bash
make localdev-test-all-api     # Full checks: code quality + API tests
make localdev-test-api-only    # API endpoint tests only
```

Raptor may run narrower commands (e.g. `go test ./...`) for fast feedback,
but Stage-01 correlation/RCA changes are not considered verified until the two
`make` targets above pass.

**Constraints:**

- Must not auto-merge changes; always produce reports that humans review.
- When invoked for correlation/RCA work (AT-00x items), it must:
  - Confirm time-window-only API contract is respected,
  - Exercise buckets/rings logic,
  - Exercise statistical correlation and narrative output.

---

## 2. Testing Procedures for Mirador Core

Mirador Core uses a comprehensive testing strategy:

- Unit tests (with race detection where applicable)
- Integration tests and E2E tests (full environment)
- Code quality checks:
  - Linting
  - Formatting
  - Vulnerability scanning
- API endpoint tests
- Local development environment tests

### 2.1 Standard Make Targets

Primary targets:

```bash
make localdev-test-all-api     # Full checks: code quality + API tests
make localdev-test-api-only    # API tests only, skipping code quality
```

**Rules:**

- Any PR that touches **correlation, RCA, EngineConfig, or their HTTP handlers**
  must run both targets locally **or** rely on CI that runs them.
- Raptor-based automation must treat a failure in either target as a **hard stop**
  until investigated and fixed.

### 2.2 Correlation & RCA Specific Expectations

For Stage-01 correlation/RCA work, tests must cover:

- **Time-window-only API payloads**
  - Extra fields should be rejected with informative errors.
- **Buckets/Rings**
  - Proper alignment of data per bucket/ring.
  - Behaviour for different ring strategies.
- **Statistical correlation**
  - Pearson, Spearman, cross-correlation (with lags), partial correlation.
- **Narrative engine**
  - RCA explanations are generated as per design.
- **Error handling**
  - Invalid windows (`endTime <= startTime`),
  - Windows out of bounds (`MinWindow`, `MaxWindow`),
  - Backend failure paths (telemetry unavailable, etc.).

These expectations are enforced primarily via:

- The tasks described in `dev/correlation-rca-action-tracker.yml`
  (especially AT-005–AT-008),
- The tests updated under AT-008.

---

## 3. Stage-01 Correlation & RCA Development Rules

These rules apply to all correlation/RCA work.

### 3.1 API Contract (Hard Rule)

- **Public endpoints:**
  - `POST /api/v1/unified/correlate`
  - `POST /api/v1/unified/rca`
- **Request body JSON must be exactly:**

  ```json
  {
    "startTime": "ISO-8601 UTC timestamp",
    "endTime":   "ISO-8601 UTC timestamp"
  }
  ```

- No other fields in the JSON payload are allowed.
- Impact KPIs, incident metadata, and other context are discovered internally
  using Stage-00 registries and telemetry, not passed directly in the request.

### 3.2 Internal Types & Configuration

Implementation must follow the final implementation guide:

- Canonical window type:

  ```go
  type TimeRange struct {
      Start time.Time
      End   time.Time
  }
  ```

- Configuration via `EngineConfig`:
  - `MinWindow`, `MaxWindow`
  - `DefaultGraphHops`, `DefaultMaxWhys`
  - `RingStrategy`
  - Bucket/ring settings
  - Thresholds for correlation and anomaly scores

**Forbidden:**

- Adding request-scoped knobs like `graphHops`, `maxWhys`, `ringStrategy` to
  the public APIs.

### 3.3 Engine Layering

- `CorrelationEngine` must expose:

  ```go
  Correlate(ctx context.Context, tr TimeRange) (CorrelationResult, error)
  ```

- `RCAEngine` must expose:

  ```go
  RunRCA(ctx context.Context, tr TimeRange) (RCAResult, error)
  ```

- `RCAEngine` must **reuse** `CorrelationEngine` internally and build its
  why-chain and narrative purely from `CorrelationResult` + ring context.

### 3.4 Temporal Anchoring & Rings

- The engine must:
  - Detect impact time `T` within the window,
  - Build rings/buckets as per the final implementation doc,
  - Use these rings consistently for:
    - Data alignment,
    - Suspicion scoring,
    - Statistical correlation.

Any new code manipulating time windows for correlation/RCA must respect that
model and keep behaviour consistent with design docs in
`dev/correlation-RCA-engine/current`.

### 3.5 Statistical Correlation

For candidate Impact–Cause KPI pairs, the engine may use:

- Pearson correlation
- Spearman rank correlation
- Cross-correlation with lag
- Partial correlation

These methods:

- Feed into suspicion scoring,
- Inform time-ordering (lead/lag),
- Reduce confounders via partial correlation.

New work must not introduce opaque, undocumented statistical methods; if a new
method is introduced, update:

- `dev/correlation-RCA-engine/current/01-correlation-rca-approach-final.md`
- `dev/correlation-RCA-engine/current/01-correlation-rca-code-implementation-final.md`
- Tests under AT-008

accordingly.


### 3.6 Engine hygiene (no TODO stubs, no magic labels)

For the Correlation & RCA engines, the following rules are **mandatory**:

- **No TODO/FIXME stubs in engine logic**
  - The core engine packages (correlation, RCA) must not contain "TODO" or "FIXME"
    comments that describe unfinished behaviour.
  - If work is planned for a later tracker item, reference it explicitly instead of
    leaving a bare TODO, e.g.:
    - `// NOTE: Statistical correlation will be implemented under AT-007.`
  - Stage-01 is considered complete only when there are no anonymous TODOs in engine
    code; open work must live in:
    - `dev/correlation-RCA-engine/current/correlation-rca-action-tracker.yml`
    - or the design docs under `dev/correlation-RCA-engine/current`.

- **No hardcoded metric or label names in engines**
  - CorrelationEngine and RCAEngine must not hardcode:
    - Metric/KPI names (e.g. "transactions_failed_total", "http_errors_total"),
    - Log/metric label keys (e.g. "service.name", "kubernetes.pod_name"),
    - Service names, namespaces, or other environment-specific identifiers.
  - All such names must come from:
    - Stage-00 KPI/registry metadata, or
    - Engine configuration (e.g. EngineConfig.Labels), or
    - Other well-defined config/registry layers.
  - Engine code may only use **canonical semantic keys** (e.g. "service", "pod",
    "namespace") and must rely on config/registry to map these to raw field names.

- **Config- and registry-driven behaviour**
  - KPI discovery, label extraction, and suspicion scoring must be driven by:
    - EngineConfig and
    - KPI/telemetry registries,
    not hardcoded probe lists or ad-hoc string constants.
  - When new metrics or label schemes are introduced, they should be added to:
    - Config files (e.g. configs/config.yaml) or
    - Registry definitions,
    without requiring changes to engine code.

PR reviewers and Raptor must treat violations of these rules as **blocking** for
Stage-01 correlation/RCA work.

### 3.7 Enforcement Tooling

To ensure compliance with the engine hygiene rules in §3.6, the following automated
enforcement mechanisms are in place:

#### Automated Checks

**Scripts:**

1. **`scripts/check-hardcoded-violations.sh`**
   - Scans engine code for hardcoded metric names, service names, and label keys
   - Enforces proper use of empty slices with NOTE comments in `defaults.go`
   - Exempts test files, simulators, and test fixtures
   - Exit code 0 = compliant, 1 = violations detected

2. **`scripts/check-todo-violations.sh`**
   - Detects anonymous TODO/FIXME comments without tracker references
   - Requires all TODOs to reference tracker items (AT-XXX, HCB-XXX) or use NOTE()
   - Exempts test files where TODO comments for incomplete tests are acceptable
   - Exit code 0 = compliant, 1 = violations detected

**Makefile Targets:**

```bash
make check-hardcoded        # Run hardcoded violations check
make check-todos            # Run TODO violations check
make check-engine-hygiene   # Run both checks (recommended before commits)
```

**Git Pre-Commit Hook:**

Install pre-commit hooks that automatically run enforcement checks:

```bash
make install-git-hooks
```

Once installed, every `git commit` will:
1. Run `check-hardcoded-violations.sh`
2. Run `check-todo-violations.sh`
3. Run hardcode detection regression tests
4. Block the commit if any check fails

To bypass in emergencies (requires PR justification):

```bash
git commit --no-verify
```

#### CI Integration

The enforcement checks should be integrated into CI pipelines:

```yaml
# Example GitHub Actions workflow
- name: Check Engine Hygiene
  run: make check-engine-hygiene

- name: Run Hardcode Detection Tests
  run: go test ./internal/services -run "TestNoHardcodedStringsInEngines|TestNoAnonymousTODOsInEngines|TestConfigDefaultsNoHardcodedValues" -v
```

#### Exemptions

The following paths are **exempt** from hardcoded string enforcement:

1. **Test files** (`*_test.go`): May use hardcoded strings for test assertions
2. **Test fixtures** (`internal/services/testdata/`, `test_fixtures.go`): Centralized test constants
3. **Simulators** (`cmd/otel-fintrans-simulator/`): Intentional hardcoding for simulation fidelity (see HCB-008 exemption documentation)
4. **Documentation files** (`internal/rca/models.go`, `internal/rca/label_detector.go`): Example service names in comments

#### Violation Response

When violations are detected:

1. **Pre-commit hook blocks commit**: Fix violations before committing
2. **CI build fails**: PR cannot merge until violations are resolved
3. **Fix strategies**:
   - Hardcoded metrics/services: Move to registry or EngineConfig
   - Anonymous TODOs: Replace with `NOTE(TRACKER-ID): explanation` or implement immediately
   - Config defaults: Use empty slices with NOTE comments referencing tracker items

#### Regression Tests

Automated regression tests enforce these rules:

- `TestNoHardcodedStringsInEngines`: Scans engine source for forbidden patterns
- `TestNoAnonymousTODOsInEngines`: Scans for anonymous TODO/FIXME comments
- `TestConfigDefaultsNoHardcodedValues`: Validates defaults.go uses empty slices with NOTEs

These tests are located in `internal/services/hardcode_detection_test.go` and must
always pass before merging engine changes.

---

## 4. Weaviate Usage Guidelines

All Weaviate usage must be:

- **Centralised**
- **Testable**
- **Version-controlled**

### 4.1 Store/Repo Pattern (Do This)

1. **Do not** call the Weaviate Go SDK directly from:

   - HTTP handlers,
   - Business logic,
   - Correlation/RCA engines.

2. **Do**:

   - Centralise all Weaviate interactions in well-defined **store/repo layers**.
   - Wrap the SDK in small, testable helpers, e.g. `WeaviateKPIStore`.
   - Ensure all KPI-related Weaviate operations (Create, Update, Query, Delete)
     flow through a shared KPI repo/store, not ad-hoc calls.

3. Keep stores:

   - Well-typed (no `map[string]interface{}` dumps),
   - Easy to mock for tests,
   - Documented (input/output contracts).

### 4.2 Versioning

- New work must target the agreed major version of the Go client:
  - **Weaviate Go client v5**.
- If you need to upgrade the client version, you must:

  1. Update `go.mod`.
  2. Update all wrappers/stores that use the SDK.
  3. Update this section in `AGENTS.md` to:
     - Reflect the new version,
     - Document any breaking changes and their mitigations.

This ensures Weaviate access remains consistent, testable, and easy to evolve
as schemas, vector usage, and correlation/RCA requirements grow.

---

## 5. Action Tracker & PR Workflow

### 5.1 Action Tracker

The canonical tracker for Stage-01 correlation/RCA is:

```text
dev/correlation-rca-action-tracker.yml
```

A contextual copy or link may live in:

```text
dev/correlation-RCA-engine/current/correlation-rca-action-tracker.yml
```

See `dev/README-correlation-rca.md` for:

- How items are owned,
- How to change statuses,
- How to close items (artifacts + tests).

Any PR affecting correlation/RCA should:

1. Reference the relevant `AT-XXX` ID(s) in the PR title or description.
2. Update the action tracker with status changes.
3. Include or update the artifacts listed for those items.
4. Ensure required tests pass (Section 2).

### 5.2 PR Checklist (Humans + Agents)

Before merging a PR that touches correlation/RCA, confirm:

- [ ] API contract remains `{ startTime, endTime }` only.
- [ ] EngineConfig is used instead of request-scoped tuning fields.
- [ ] Correlation/RCA engine layering is preserved.
- [ ] Temporal rings/buckets are implemented according to the final docs.
- [ ] Tests:
  - [ ] `make localdev-test-all-api` passed (locally or in CI).
  - [ ] `make localdev-test-api-only` passed.
- [ ] Any Weaviate usage goes through store/repo layers and respects v5 client.
- [ ] Tracker (`dev/correlation-rca-action-tracker.yml`) is updated if relevant.
- [ ] Documentation changes are applied if behaviour changed.

Agents (GPT5 Mini, Raptor) are expected to assist with this checklist but not
replace human review.

---

## 6. Updating This Document

When any of the following change:

- Correlation/RCA API behaviour,
- EngineConfig structure,
- Testing standards or `make` targets,
- Weaviate client version,
- Agent roles (GPT5 Mini, Raptor),
- The layout or contents of `dev/correlation-RCA-engine/current`,

you must:

1. Update this `AGENTS.md`.
2. Update the relevant design/impl docs in `dev/correlation-RCA-engine/current`.
3. Ensure the tracker and dev README remain consistent.

All such updates must go through code review like any other change.

