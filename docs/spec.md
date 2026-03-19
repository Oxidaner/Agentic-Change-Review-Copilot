# Evidence-Grounded Change Review Copilot Spec

## 1. Goal And Boundary

### 1.1 Goal
- Perform structured risk review before engineering changes enter release flow.
- Combine change content, evidence, runtime context, rules, model analysis, and human approval into one controlled workflow.
- Produce structured outputs instead of plain natural language summaries.
- Support audit trail, human override, evaluation feedback, and continuous improvement.

### 1.2 Non-Goal
- Do not directly execute production release actions.
- Do not replace CI/CD, release platforms, CMDB, or observability systems.
- Do not build a free-form autonomous agent in the MVP.
- Do not cover every change type in Phase 1.

### 1.3 Design Principles
- Evidence first.
- Controlled workflow first.
- AI as capability layer, not system boundary.
- Human-in-the-loop for high-risk or low-confidence cases.
- Evaluation and feedback loop as first-class features.

## 2. Primary Scenario

The project should now be described as one primary scenario:
- engineering change review

Phase 1 focus:
- PR diff review
- K8s / YAML configuration review

Later expansion:
- SQL migration review
- gateway config review
- Terraform review

## 3. Three-Layer Architecture

### 3.1 Runtime / Orchestration Layer

This layer controls how the workflow runs.

Responsibilities:
- receive review tasks
- manage state transitions
- coordinate steps
- schedule tool calls
- handle timeout and retry
- support human takeover
- persist trace and audit events

Core abstractions to grow toward:
- `Workflow`
- `Step`
- `State`
- `Tool`
- `ModelAdapter`
- `ReviewTask`
- `TraceEvent`

Suggested workflow states:
- `INIT`
- `PARSE_CHANGE`
- `CLASSIFY_CHANGE`
- `RETRIEVE_CONTEXT`
- `EXTRACT_RISK_SIGNALS`
- `RULE_CHECK`
- `LLM_ANALYZE`
- `SCORE_RISK`
- `GENERATE_RECOMMENDATION`
- `HUMAN_REVIEW_REQUIRED`
- `DONE`
- `FAILED`

### 3.2 AI Capability Layer

This layer provides intelligent capabilities.

Responsibilities:
- parse change semantics
- retrieve relevant context
- extract risk signals
- run structured model analysis
- score risk
- generate recommendation
- generate rollback plan

Suggested modules:
- `change_parser`
- `context_retriever`
- `incident_retriever`
- `runbook_retriever`
- `risk_signal_extractor`
- `risk_rule_engine`
- `risk_scorer`
- `recommendation_generator`
- `rollback_planner`

### 3.3 Scenario Layer

This layer binds the runtime and AI capabilities to concrete engineering scenarios.

Phase 1 should keep the scenario layer narrow:
- PR diff review
- K8s / YAML review

## 4. Current System Shape

The current repository already contains a minimal backend skeleton aligned with this spec.

Implemented APIs:
- create review
- get review
- get timeline
- submit human decision
- retry review
- export review
- submit evaluation feedback
- query evaluation metrics

Current runtime shape:
- fixed DAG / controlled workflow
- heuristic rules
- structured responses
- PostgreSQL persistence
- audit and evaluation feedback persistence

Current limitation:
- real external systems are not integrated yet
- tool calls are not fully abstracted yet
- AI layer is still mostly heuristic / template driven

## 5. Services And Modules

### 5.1 Review API
Responsibilities:
- create review tasks
- expose review detail and timeline
- accept human decisions
- accept evaluation feedback
- export reports
- query metrics

### 5.2 Change Ingestion
Responsibilities:
- normalize input into `ChangeBundle`
- identify source type and change type
- persist raw and normalized snapshots later

### 5.3 Workflow Runtime
Responsibilities:
- drive state machine
- run controlled step sequence
- coordinate review lifecycle

### 5.4 Tool Layer
Responsibilities:
- provide stable interface to external systems
- return structured evidence objects

Planned tool types:
- `git_history_search`
- `incident_search`
- `runbook_search`
- `service_dependency_lookup`
- `metrics_snapshot`
- `owner_lookup`

### 5.5 Risk Rule Engine
Responsibilities:
- produce deterministic `RiskSignal`
- compute base score and lower bound of risk level

### 5.6 Recommendation Engine
Responsibilities:
- generate approval suggestion
- generate rollout suggestion
- generate observability checklist
- generate rollback plan

### 5.7 Audit And Evaluation
Responsibilities:
- store review events
- store human decisions
- store evaluation feedback
- support reporting metrics

## 6. Core Data Models

### 6.1 ChangeBundle
Normalized engineering change payload used across workflow steps.

### 6.2 EvidenceItem
Evidence object with source, title, snippet, reference, confidence, and metadata.

### 6.3 RiskSignal
Structured risk signal extracted from rules, parsing, or model analysis.

### 6.4 ReviewDecision
Structured system conclusion containing status, score, risk level, recommendation, and rollback plan.

### 6.5 HumanDecision
Reviewer action such as approve, reject, or override.

### 6.6 EvaluationRecord
Post-release outcome record used for feedback loop and metrics.

### 6.7 TraceEvent
Workflow execution event for timeline and audit.

## 7. MVP End-To-End Flow

1. Accept review input.
2. Normalize change payload.
3. Classify change.
4. Retrieve context or simulated evidence.
5. Extract risk signals.
6. Run rule checks.
7. Produce structured recommendation.
8. Escalate to human review when high risk or low confidence.
9. Record human decision.
10. Record evaluation feedback after release.
11. Query metrics from persisted records.

## 8. Current Priorities

Highest priority:
1. Add PostgreSQL integration tests.
2. Make runtime abstractions explicit in code.
3. Narrow implementation work to PR diff and K8s/YAML.

Next priority:
1. Add OpenAPI contract validation.
2. Add CLI demo.
3. Separate rule engine / retriever / recommendation generator modules more clearly.

## 9. Success Criteria

This project is successful as an MVP if it can demonstrate:
- controlled workflow rather than free-form agent behavior
- structured change review output
- evidence-grounded risk reasoning
- human review and override support
- audit and evaluation feedback loop
- a credible path to platformization and reuse
