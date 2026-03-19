# Evidence-Grounded Change Review Copilot 设计文档

## 1. 项目定位

### 1.1 项目名称
Evidence-Grounded Change Review Copilot

中文名称：
基于证据链的工程变更审查 Copilot

### 1.2 一句话定义
这是一个面向真实工程流程的 AI 审查运行时。它通过受控 workflow、工具调用、证据链整合、规则检查、模型分析和人工接管机制，对工程变更进行风险审查，并输出结构化建议、回滚方案和评测反馈。

### 1.3 本项目不是什么
- 不是单点 prompt demo。
- 不是自由规划的通用 Agent。
- 不是替代 CI/CD 的发布执行器。
- 不是只会“调模型 API”的聊天机器人。

### 1.4 本项目是什么
- 是一个可复用的 workflow-first runtime。
- 是一个把 AI 能力嵌进真实工程流程的审查平台雏形。
- 是一个支持工具接入、结构化输出、审计、评测和反馈回流的工程化系统。

---

## 2. 为什么做这个项目

### 2.1 背景
真实的软件发布和配置变更并不是单一代码 diff，它们通常发生在开发、测试、发布和运行治理链路中，涉及：
- PR diff
- Kubernetes YAML / Helm 变更
- SQL migration
- 网关路由与鉴权策略
- 基础设施配置

这些变更的风险判断长期依赖资深工程师经验，问题在于：
- 信息分散在 Git、CI、监控、事故库、runbook、CMDB 等多个系统。
- 风险判断口径不统一，难以沉淀为可复用能力。
- 人工审查成本高，且容易遗漏隐性上下文。
- AI 如果没有受控 workflow 和证据约束，很容易变成不可解释的“聪明回答”。

### 2.2 项目目标
本项目要解决的不是“让模型看起来会分析”，而是：
- 让工程变更审查变成一个可编排、可复用、可观测、可评测的系统能力。
- 让 AI 成为工程流程中的受控能力层，而不是独立漂浮的问答层。
- 让风险结论始终尽量绑定证据、规则和人工反馈。

---

## 3. 设计原则

### 3.1 受控 workflow 优先
采用固定步骤、状态机和 DAG 驱动，而不是自由 Agent。高可信工程场景更需要可解释性、稳定性和可回放性。

### 3.2 证据优先
任何风险结论都应尽量绑定证据来源，例如：
- Git 历史
- 历史事故
- runbook
- 配置解析结果
- 规则命中结果
- 人工反馈

### 3.3 AI 是能力层，不是系统边界
模型调用、检索、结构化输出只是系统中的一部分，外层真正重要的是 workflow runtime、工具抽象、状态流转和审计闭环。

### 3.4 工程流程优先于聊天体验
系统聚焦于开发流程、发布流程和运行治理闭环，不追求通用问答能力。

### 3.5 闭环优先于单次效果
系统必须支持：
- 审计
- 评测
- 反馈回流
- 指标量化
- 持续迭代

---

## 4. 三层架构

本项目采用三层架构，而不是单个写死脚本，也不是一开始就做成大而全平台。

### 4.1 Runtime / Orchestration 层

这一层负责“流程怎么跑”。

职责：
- 定义 workflow step
- 状态机流转
- 条件分支
- 工具调用调度
- 超时控制
- 重试与失败恢复
- 中断与人工接管
- trace / audit

这一层本质上是一个轻量 workflow runtime，而不是自由 Agent。

建议的核心抽象：
- `Workflow`
- `Step`
- `State`
- `Tool`
- `ModelAdapter`
- `ReviewTask`
- `TraceEvent`

建议状态流：
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

### 4.2 AI Capability 层

这一层负责“智能能力从哪里来”。

职责：
- 模型调用
- prompt 模板
- structured output
- 检索与 rerank
- 风险信号抽取
- 风险评分
- 建议生成
- 回滚方案生成

建议模块：
- `change_parser`
- `context_retriever`
- `incident_retriever`
- `runbook_retriever`
- `risk_signal_extractor`
- `risk_rule_engine`
- `risk_scorer`
- `recommendation_generator`
- `rollback_planner`

### 4.3 Scenario Layer

这一层负责“具体业务场景是什么”。

一期不要做过多场景，只打透一个主场景：
- 工程变更审查

一期优先支持两类输入：
- PR diff
- K8s / YAML 配置变更

这样做的原因：
- 工程味足够强
- 容易接规则、检索和模型分析
- 容易讲清楚风险审查价值
- 便于后续扩展成平台

后续可扩展：
- SQL migration review
- 网关配置 review
- Terraform review

---

## 5. 一期 MVP 范围

### 5.1 一期目标
一期目标不是全能 Agent 平台，而是做一个：

能对工程变更做风险审查，并输出结构化建议的 AI workflow 系统。

### 5.2 一期必须打通的链路
- 接收 change 输入
- 解析 change
- 分类 change
- 检索相关上下文
- 提取风险信号
- 做规则检查
- 调用 LLM 综合分析
- 输出风险等级、证据、建议和回滚方案
- 高风险时进入人工复核
- 记录审计与评测反馈

### 5.3 一期交付形态
优先做：
- 后端 API
- CLI / JSON / Markdown demo

不优先做：
- 完整前端工作台
- 通用多租户平台
- 大而全场景覆盖

---

## 6. 系统边界

### 6.1 上游系统
- GitHub / GitLab
- CI 系统
- 发布平台
- 配置平台
- 监控与指标系统
- runbook / wiki / incident 系统

### 6.2 下游系统
- 审批流
- 通知系统
- 审计系统
- 评测与看板

### 6.3 边界声明
本系统输出的是：
- 风险审查结果
- 证据链
- 审批建议
- 灰度和回滚建议
- 评测反馈记录

本系统不负责：
- 直接执行生产发布
- 自动放行所有高风险变更
- 做无边界的自由推理

---

## 7. 关键模块

### 7.1 Change Ingestion
负责接收和标准化不同来源的变更输入，抽象成统一的 `ChangeBundle`。

### 7.2 Workflow Runtime
负责受控 DAG、状态迁移、失败重试、超时和任务生命周期管理。

### 7.3 Tool Layer
负责和外部系统交互，建议统一抽象工具接口。

一期工具类型：
- `git_history_search`
- `incident_search`
- `runbook_search`
- `service_dependency_lookup`
- `metrics_snapshot`
- `owner_lookup`

### 7.4 Risk Rule Engine
负责稳定、可解释、可配置的规则判断，输出 `RiskSignal`。

### 7.5 Model Adapter
负责模型调用和结构化结果封装，隐藏底层模型差异。

### 7.6 Recommendation Engine
负责生成：
- 风险等级
- 审批建议
- 灰度策略
- 观测指标
- 回滚计划

### 7.7 Audit & Evaluation
负责：
- trace event
- 审查记录
- 人工决策
- 发布反馈
- 指标汇总

---

## 8. 数据模型建议

建议核心对象包括：
- `ChangeBundle`
- `EvidenceItem`
- `RiskSignal`
- `ReviewDecision`
- `HumanDecision`
- `EvaluationRecord`
- `TraceEvent`

这些对象的作用分别是：
- `ChangeBundle`：统一变更语义
- `EvidenceItem`：保存证据链
- `RiskSignal`：保存规则或模型识别出的风险信号
- `ReviewDecision`：保存系统结论
- `HumanDecision`：保存人工 override 或审批结果
- `EvaluationRecord`：保存发布结果和事故反馈
- `TraceEvent`：保存 workflow 运行轨迹

---

## 9. 当前仓库实现与目标架构的对应关系

截至当前版本，仓库已经实现了一个最小可运行骨架，和目标架构的对应关系如下。

### 9.1 已实现部分
- Review API 骨架
- 固定 workflow 流程
- 内存存储和 PostgreSQL 存储
- human decision 持久化
- evaluation feedback 持久化
- metrics 查询
- OpenAPI 契约
- 本地开发启动方案
- API 集成测试

### 9.2 当前实现仍然是 MVP
当前实现还没有真正接入：
- Git
- incident 库
- runbook 系统
- 指标系统
- CI / 发布系统 webhook

当前风险分析仍然是启发式规则 + 模板化建议，不是真实生产级审查引擎。

### 9.3 这说明什么
说明当前仓库已经不是单点 demo，而是：
- 一个可继续扩展的 review runtime 雏形
- 一个有审计和反馈闭环意识的工程化骨架
- 一个可以继续往平台化能力生长的底座

---

## 10. 当前最优演进路径

### 10.1 第一优先级
- 增加 PostgreSQL 集成测试
- 验证 migration、持久化读写和 metrics 链路

### 10.2 第二优先级
- 把 runtime、tool、model adapter、rule engine 的抽象边界进一步显式化
- 减少当前 service 中的“全堆在一起”的实现

### 10.3 第三优先级
- 先打透 `PR diff + K8s/YAML` 两类场景
- 明确每类场景的解析器、规则和证据来源

### 10.4 第四优先级
- 增加契约校验
- 增加 CLI demo
- 增加更清晰的 handoff / README 文档

---

## 11. 面试叙事建议

这个项目最值得强调的不是“我接了一个 LLM”，而是：

### 11.1 对后端 / 平台岗
重点讲：
- workflow runtime
- 状态机
- 存储设计
- 审计与可观测性
- 工具抽象
- 可扩展架构

### 11.2 对 AI 应用 / Agent 岗
重点讲：
- 受控 workflow 优于自由 agent
- evidence-grounded 输出
- 规则 + 检索 + LLM 的混合架构
- 结构化输出和反馈闭环

### 11.3 对 SRE / DevOps / 工程效能方向
重点讲：
- AI 如何嵌入真实发布流程
- 如何降低高风险变更漏判
- 如何让审查结果可回滚、可审计、可评测

---

## 12. 结论

Evidence-Grounded Change Review Copilot 的正确方向，不是做一个“会分析 diff 的 AI demo”，而是做一个：

面向真实工程流程的、受控 workflow 驱动的、可接工具和系统、可沉淀证据链、可支持审计与评测闭环的审查运行时。

一期只需要把这一条主线做扎实：
- PR diff
- K8s / YAML review
- 证据链
- 风险信号
- 结构化建议
- 人工复核
- 反馈闭环

这条主线一旦打透，这个项目就已经足够支撑工程化叙事、平台化叙事和 AI 应用叙事。
