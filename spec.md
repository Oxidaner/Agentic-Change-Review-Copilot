# Agentic Change Review Copilot Spec

## 1. 目标与边界

### 1.1 目标
- 在发布前对变更进行结构化风险审查。
- 将变更内容、历史证据、运行态上下文和审批策略统一到一个审查闭环中。
- 产出可执行的结构化结果，而不是仅输出自然语言摘要。
- 支持人工升级、审计留痕和后续评测闭环。

### 1.2 非目标
- 不直接执行生产发布动作。
- 不替代现有 CI/CD、发布平台、CMDB 或监控系统。
- 不在 MVP 阶段做自由自治型 Agent。
- 不在 MVP 阶段覆盖所有变更类型和多租户平台能力。

### 1.3 设计原则
- 证据优先：所有关键结论必须可追溯到证据。
- 人机协同：高风险和低置信度场景必须人工确认。
- 结构化输出：输出需可嵌入审批流和审计系统。
- 可评测：支持误报、漏报、采纳率、召回率分析。
- 可降级：外部依赖异常时允许带不完整证据继续运行，但必须显式暴露置信度下降。

## 2. 核心场景

- PR / MR 发布前风险审计
- SQL migration 发布前风险审计
- K8s YAML / Helm 配置发布前风险审计
- 后续扩展：Terraform、网关路由与鉴权配置、配置中心参数变更

## 3. 逻辑架构

系统拆成六层：

1. 接入层：接收 webhook 或主动提交的审查请求。
2. 变更语义层：将原始输入标准化为统一的 `ChangeBundle`，并抽取语义标签。
3. 上下文与证据层：查询 Git 历史、依赖图、事故库、Runbook、监控快照等。
4. 风险决策层：提取风险信号、执行规则、计算评分、生成建议。
5. 协同输出层：输出审批建议、灰度策略、回滚预案，并触发人工审批。
6. 审计评测层：记录全流程事件、人工反馈和线上结果。

### 3.1 高层组件图

```text
Webhook / API / Release Platform
  -> Review API Service
  -> Change Ingestion Service
  -> Review Orchestrator
  -> Change Understanding Engine
  -> Context Retrieval Service
  -> Risk Rule Engine
  -> Agent Workflow Runner
  -> Recommendation Engine
  -> Review Console / Notification Service
  -> Audit & Evaluation Service
```

### 3.2 架构决策
- 采用 workflow-based agent，而不是自由 ReAct。
- 规则引擎负责稳定的高风险命中，LLM 负责摘要、归因和建议解释。
- 证据采集与决策输出解耦，便于缓存和回放。
- 审查主链路异步执行，对外提供查询接口和时间线。

## 4. 服务划分

### 4.1 Review API Service
职责：
- 创建审查任务
- 查询审查详情、时间线、结果导出
- 接收人工审批结果
- 接收 webhook 并做签名校验、幂等校验

输入：
- Git webhook
- 发布平台 webhook
- 控制台手动触发

输出：
- `review_id`
- 查询接口
- 人工决策写入

### 4.2 Change Ingestion Service
职责：
- 拉取原始 diff 和元数据
- 识别变更类型
- 标准化成 `ChangeBundle`
- 写入原始快照与标准化快照

### 4.3 Review Orchestrator
职责：
- 驱动状态机
- 串联理解、检索、信号抽取、评分、建议、通知
- 管理超时、重试、幂等和失败恢复

### 4.4 Change Understanding Engine
职责：
- 从代码 diff、SQL、YAML、Terraform plan 中抽取结构化语义
- 识别语义标签和潜在失败模式

实现策略：
- 结构解析优先，LLM 总结兜底
- SQL / YAML / Terraform 先 AST 或结构化解析
- 代码 diff 先做路径、函数签名、配置项和关键字提取

### 4.5 Context Retrieval Service
职责：
- 提供统一工具接口
- 聚合外部证据源
- 为下游输出标准 `EvidencePack`

MVP 工具：
- `git_history_search`
- `incident_search`
- `runbook_search`
- `service_dependency_lookup`
- `metrics_snapshot`
- `owner_lookup`
- `release_window_lookup`

### 4.6 Risk Rule Engine
职责：
- 依据规则抽取 `RiskSignal`
- 计算基础分和风险等级下限
- 应用环境、服务和变更类型策略

### 4.7 Agent Workflow Runner
职责：
- 按固定 DAG 执行有限步骤工作流
- 汇总证据并生成结构化结论
- 检查所有关键结论都有 `evidence_refs`

建议节点：
1. 变更分类
2. 工具计划
3. 上下文采集
4. 风险信号整合
5. 结论生成
6. 建议与回滚生成
7. 置信度判断与人工升级

### 4.8 Recommendation Engine
职责：
- 生成审批建议
- 生成灰度策略
- 生成观测指标清单
- 生成回滚计划

### 4.9 Notification Service
职责：
- 通知高风险待审批任务
- 催办人工审批
- 推送结果摘要到 IM

### 4.10 Audit & Evaluation Service
职责：
- 记录全链路审查事件
- 记录人工决策和 override
- 汇总线上发布结果
- 产出评测指标和报表

## 5. 核心数据模型

### 5.1 ChangeBundle

```json
{
  "change_id": "chg_001",
  "source_type": "pull_request",
  "change_type": "gateway_config_change",
  "repo": "gateway-service",
  "service": "api-gateway",
  "environment": "prod",
  "author": "alice",
  "reviewers": ["bob"],
  "submit_time": "2026-03-18T10:00:00Z",
  "diff_summary": "auth middleware and route rule updates",
  "file_list": ["gateway/auth.go", "configs/routes.yaml"],
  "impacted_components": ["auth", "routing"],
  "release_target": "prod-cn"
}
```

### 5.2 EvidenceItem

```json
{
  "evidence_id": "ev_101",
  "type": "incident",
  "source": "incident_db",
  "title": "2025-11 auth routing regression",
  "content_snippet": "similar route auth change caused 401 spike",
  "reference_url": "https://...",
  "confidence": 0.93,
  "retrieved_at": "2026-03-18T10:00:03Z",
  "metadata": {}
}
```

### 5.3 RiskSignal

```json
{
  "signal_id": "sig_001",
  "signal_name": "auth_logic_changed",
  "severity": "HIGH",
  "score_delta": 20,
  "explanation": "core auth middleware modified",
  "evidence_refs": ["ev_101", "ev_104"]
}
```

### 5.4 ReviewDecision

```json
{
  "review_id": "rvw_001",
  "status": "WAITING_HUMAN_REVIEW",
  "risk_level": "HIGH",
  "score": 78,
  "confidence": 0.82,
  "summary": "鉴权链路和入口路由同时变更，风险较高。",
  "top_risks": [
    {
      "risk": "traffic_routing_changed",
      "evidence_refs": ["ev_101"]
    }
  ],
  "approval_recommendation": {},
  "rollout_strategy": {},
  "observability_plan": [],
  "rollback_plan": {}
}
```

### 5.5 HumanFeedback

```json
{
  "review_id": "rvw_001",
  "reviewer": "bob",
  "decision": "override",
  "reason": "已安排值班和灰度窗口",
  "override_flag": true,
  "created_at": "2026-03-18T10:12:00Z"
}
```

## 6. 状态机

### 6.1 状态定义
- `CREATED`
- `NORMALIZED`
- `UNDERSTOOD`
- `COLLECTING_CONTEXT`
- `CONTEXT_READY`
- `SIGNALS_EXTRACTED`
- `SCORED`
- `RECOMMENDED`
- `WAITING_HUMAN_REVIEW`
- `APPROVED`
- `REJECTED`
- `OVERRIDDEN`
- `FAILED`
- `CANCELLED`

### 6.2 状态迁移

```text
CREATED
  -> NORMALIZED
  -> UNDERSTOOD
  -> COLLECTING_CONTEXT
  -> CONTEXT_READY
  -> SIGNALS_EXTRACTED
  -> SCORED
  -> RECOMMENDED
  -> WAITING_HUMAN_REVIEW | APPROVED

Any State -> FAILED | CANCELLED
WAITING_HUMAN_REVIEW -> APPROVED | REJECTED | OVERRIDDEN
```

### 6.3 升级条件
- 风险等级为 `HIGH` 或 `CRITICAL`
- `confidence < 0.75`
- 关键证据缺失
- 工具结果冲突
- 涉及鉴权、权限、数据删除、核心链路变更

## 7. 主流程设计

### 7.1 审查主链路
1. 外部系统触发审查请求。
2. Review API 创建 `review_id`，做签名和幂等校验。
3. Change Ingestion 拉取原始变更并标准化为 `ChangeBundle`。
4. Change Understanding Engine 生成 `semantic_tags`、`risk_hints`、`potential_failure_modes`。
5. Context Retrieval Service 查询历史事故、Runbook、依赖关系和监控快照。
6. Risk Rule Engine 提取风险信号并计算基础分。
7. Agent Workflow Runner 基于证据生成结构化总结和缺失信息列表。
8. Recommendation Engine 输出审批建议、灰度策略、观测计划和回滚预案。
9. Orchestrator 根据风险等级和置信度决定是否进入人工审批。
10. Audit & Evaluation Service 记录全流程事件。

### 7.2 失败恢复
- 工具超时：有限重试，超限后记录低置信度。
- LLM 失败：切换 fallback 模型。
- 部分证据源不可用：不阻塞整体流程，但要求结果显式带 `missing_information`。
- 重试必须复用已有 `review_id`，避免重复审查。

### 7.3 幂等策略
- 使用 `source_id + head_commit + environment` 作为 `dedupe_key`
- 重复请求直接返回已有 `review_id`
- webhook 和手动重试共用状态机记录

## 8. 规则与模型协作

### 8.1 职责划分
- 规则引擎负责可确定的风险识别和基础评分。
- LLM 负责高层摘要、风险归纳、建议解释和缺失信息识别。
- Recommendation Engine 负责模板化输出，不让 LLM 直接生成最终执行指令。

### 8.2 典型高风险信号
- `auth_logic_changed`
- `traffic_routing_changed`
- `irreversible_operation`
- `core_path_modified`
- `cross_service_contract_change`
- `resource_limit_lowered`
- `public_exposure_risk`
- `missing_verified_rollback`

### 8.3 评分策略
- 硬规则分值
- 风险信号加权
- 环境修正项
- 服务重要性修正项
- 证据完整性修正项

示例：
- 核心鉴权逻辑变更：`+30`
- 不可逆 SQL：`+40`
- 核心链路服务：`+20`
- 缺少回滚方案：`+15`
- 当前环境高错误率：`+10`
- 低峰期发布：`-10`
- 完整灰度方案：`-8`

### 8.4 输出约束
- 每条关键风险必须绑定至少一个 `evidence_ref`
- 没有证据支撑的内容只能进入 `hypotheses`
- `confidence` 必须由证据完整度、工具成功率和模型一致性共同决定

## 9. 外部集成

### 9.1 上游
- GitHub / GitLab / Gitea
- Jenkins / GitHub Actions / GitLab CI
- 发布平台
- 配置中心
- 数据库变更平台

### 9.2 下游
- 审批控制台
- Slack / 飞书 / 钉钉
- 审计日志平台
- 风险与评测看板

### 9.3 外部依赖
- PostgreSQL
- Redis
- Kafka 或 RabbitMQ
- S3 或 MinIO
- OpenSearch
- Prometheus / Grafana
- CMDB / 服务目录
- Runbook / Wiki / 事故库
- LLM Gateway

## 10. 存储设计

MVP 建议保留以下核心表：
- `reviews`
- `review_tasks`
- `change_snapshots`
- `evidence_items`
- `risk_signals`
- `recommendations`
- `human_decisions`
- `audit_events`
- `evaluation_records`

存储原则：
- 审查主实体存 PostgreSQL
- 原始 diff 和快照存对象存储
- 检索索引与审计检索进 OpenSearch
- 规则配置先用 YAML 管理，后续再引入规则后台

## 11. API 规格

### 11.1 对外 REST API
- `POST /api/v1/reviews`
- `GET /api/v1/reviews/{review_id}`
- `GET /api/v1/reviews/{review_id}/timeline`
- `POST /api/v1/reviews/{review_id}/human-decision`
- `POST /api/v1/reviews/{review_id}/retry`
- `GET /api/v1/evaluations/metrics`

### 11.2 创建任务请求示例

```json
{
  "source_type": "pull_request",
  "source_id": "pr_12345",
  "repo": "gateway-service",
  "service": "api-gateway",
  "environment": "prod",
  "trigger_mode": "async",
  "payload": {
    "title": "refactor auth middleware and update route rules",
    "author": "alice",
    "base_commit": "abc",
    "head_commit": "def",
    "diff_url": "https://..."
  }
}
```

### 11.3 返回对象最小字段
- `review_id`
- `status`
- `risk_level`
- `score`
- `confidence`
- `summary`
- `human_review_required`
- `signals`
- `recommendation`
- `rollback_plan`
- `evidence`

## 12. 可观测性与安全

### 12.1 监控指标
- 审查请求量
- 状态机各阶段耗时
- 工具调用成功率和超时率
- LLM 调用耗时、失败率、token 成本
- 高风险召回率
- override 率
- 审查 P95 时延

### 12.2 Trace 关键字段
- `review_id`
- `task_id`
- `change_id`
- `source_type`
- `service`
- `environment`
- `tool_name`
- `model_name`

### 12.3 安全要求
- webhook 签名校验
- 基于角色的结果访问控制
- Prompt 前脱敏
- 审计日志不可变更或最小可追责
- 生产变更默认不允许自动放行

## 13. MVP 范围

### 13.1 支持范围
- PR diff
- SQL migration
- K8s YAML / Helm diff

### 13.2 输出范围
- 风险等级
- 主要风险点
- 审批建议
- 灰度策略
- 回滚预案

### 13.3 不做
- Terraform 复杂资源图分析
- 自动执行发布
- 多租户平台化能力
- 自主长链推理

## 14. 技术选型

### 14.1 服务端
- Go：API、编排、规则、审计、通知
- Python：工作流执行、检索流水线、评测任务

### 14.2 基础设施
- PostgreSQL
- Redis
- Kafka / RabbitMQ
- MinIO / S3
- OpenSearch
- Kubernetes
- Prometheus + Grafana

### 14.3 模型接入
- 统一通过 LLM Gateway
- 支持 structured output
- 支持 fallback model
- 支持 token 审计与缓存

## 15. 实施计划

### Phase 1
- 建立 Review API、Orchestrator、基础表结构
- 支持 PR / SQL / K8s 三类输入
- 打通规则 + LLM 的最小审查闭环
- 提供只读审查详情页

### Phase 2
- 引入证据链、工具调用结果和人工 override
- 建立审计日志和评测集
- 输出召回率、误报率、采纳率

### Phase 3
- 接入真实 CI / 发布 webhook
- 加入 IM 通知和审批待办
- 支持灰度策略模板和回滚模板

### Phase 4
- 扩展更多变更类型
- 支持多规则包、多模型路由、多团队接入

## 16. 目录建议

```text
agentic-change-review/
  cmd/
    review-api/
    orchestrator/
    worker/
  internal/
    api/
    orchestrator/
    ingestion/
    rules/
    recommendation/
    audit/
    storage/
    adapters/
  proto/
  migrations/
  web/
    console/
  python/
    workflow_runner/
    retrieval/
    evaluation/
  configs/
    workflows/
    prompts/
    rules/
  docs/
```

## 17. 当前建议

- 先把 `Review API + Orchestrator + Risk Rule Engine + Recommendation Engine` 作为第一条主线做通。
- 第一版不要追求多 Agent，先把固定 DAG、证据引用和人工升级闭环做扎实。
- `spec.md` 作为开发基线，后续可继续拆成 `openapi.yaml`、`proto/`、数据库 migration 和各服务骨架代码。
