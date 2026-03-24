# Workflow Agent AI Test Platform Spec

## 1. Goal And Boundary

### 1.1 Goal
- Turn engineering changes into a controlled AI-driven automated testing workflow.
- Extract test points from PRs or requirement changes.
- Generate executable test cases.
- Run tools in a structured workflow.
- Perform intelligent assertions on execution results.
- Analyze failures and generate structured test reports.

### 1.2 Non-Goal
- Do not build a generic autonomous agent.
- Do not replace every testing platform capability in the MVP.
- Do not try to cover every test type in the first phase.
- Do not optimize for chat interaction as the primary interface.

### 1.3 Core Principle
- Workflow first.
- Tool execution first.
- Structured output first.
- Failure attribution first.
- Feedback loop first.

## 2. Main Line

The project boundary is now fixed to one clear path:

`PR / requirement change -> test point extraction -> test case generation -> tool execution -> smart assertion -> failure analysis -> test report`

This is the only main line the MVP should optimize for.

## 3. Three-Layer Architecture

### 3.1 Runtime / Orchestration Layer
Responsibilities:
- receive tasks
- manage workflow states
- call tools
- handle timeout and retry
- support human intervention
- persist trace and execution records

Target abstractions:
- `Workflow`
- `Step`
- `State`
- `Tool`
- `ToolResult`
- `ModelAdapter`
- `TestTask`
- `TraceEvent`

### 3.2 AI Capability Layer
Responsibilities:
- parse engineering changes
- extract test points
- generate test cases
- suggest test data
- perform smart assertions
- analyze failures
- generate reports

Suggested modules:
- `change_parser`
- `test_point_extractor`
- `test_case_generator`
- `test_data_planner`
- `assertion_engine`
- `failure_analyzer`
- `report_generator`

### 3.3 Scenario Layer
Responsibilities:
- bind the runtime and AI capability layers to concrete testing scenarios

Phase 1 priority:
- API regression testing
- Web UI core-path testing

## 4. Core Workflow

1. Accept PR diff or requirement change.
2. Parse the change.
3. Extract test points.
4. Generate structured test cases.
5. Choose tool execution plan.
6. Execute tools.
7. Collect logs, traces, screenshots, and artifacts.
8. Perform intelligent assertions.
9. Run failure attribution.
10. Generate structured test report.

## 5. Key Modules

### 5.1 Change Input Layer
Normalizes PR diff or requirement change input into a shared input object.

### 5.2 Test Point Extractor
Produces structured test points from engineering changes.

### 5.3 Test Case Generator
Produces executable or near-executable test case descriptions.

### 5.4 Tool Execution Layer
Executes tools such as:
- `api_test_runner`
- `ui_test_runner`
- `mock_data_loader`
- `log_query`
- `trace_query`
- `artifact_collector`

### 5.5 Smart Assertion Layer
Determines whether execution output satisfies expectations.

### 5.6 Failure Analyzer
Determines likely failure type, root cause, evidence, and next action.

### 5.7 Report Generator
Produces JSON or Markdown test report outputs.

## 6. Core Data Models

- `ChangeInput`
- `TestPoint`
- `TestCase`
- `ExecutionPlan`
- `ExecutionResult`
- `AssertionResult`
- `FailureAnalysis`
- `TestReport`
- `TraceEvent`

## 7. Current Repository Status

The repository currently contains a reusable backend skeleton from the previous review-oriented MVP.

What can be reused:
- HTTP API skeleton
- service orchestration pattern
- PostgreSQL persistence pattern
- evaluation / metrics persistence idea
- OpenAPI-first contract workflow
- Docker startup and basic tests

What still needs migration:
- domain naming
- workflow states
- API contract
- persistence object semantics
- actual testing tool integrations

## 8. Immediate Next Steps

1. Rewrite OpenAPI around the test automation workflow.
2. Rename domain objects from review semantics to test-task semantics.
3. Keep the first executable scenario narrow: API regression workflow.
4. Add PostgreSQL integration tests after the domain migration.

## 9. MVP Success Criteria

The MVP is successful if it can show:
- controlled workflow execution
- test point extraction from change input
- generated test cases
- real tool execution
- intelligent assertion results
- failure attribution
- structured test report output