# Agentic Change Review Copilot 设计文档

## 1. 文档概述

### 1.1 项目名称
Agentic Change Review Copilot（面向发布系统的智能变更审计协作系统）

### 1.2 一句话定义
面向代码发布、配置变更、数据库迁移、基础设施调整等高风险场景，构建一个具备上下文检索、证据收集、风险评估、审批建议、灰度策略生成、回滚预案生成与审计追踪能力的 Agent 系统，用于在发布前帮助工程团队做自动化变更审计，并在关键节点引入人工确认。

### 1.3 文档目标
本文档用于完整描述“变更风险审计 Agent”及其上层系统“Agentic Change Review Copilot”的业务目标、系统架构、关键模块、数据模型、工具链、评测方式、风险控制与落地路线，作为项目设计、简历包装、面试叙事和后续工程实现的统一蓝图。

### 1.4 目标读者
- 后端 / 平台 / SRE / DevOps / AI 工程师
- 面试官 / 技术评审 / 架构师
- 项目协作者

---

## 2. 背景与问题定义

### 2.1 背景
在现代微服务和云原生环境中，发布系统承载的变更种类越来越复杂，单次上线可能同时涉及：
- Git PR 合并
- SQL migration 执行
- 配置中心参数调整
- Kubernetes Deployment / HPA / Ingress 变更
- Helm values 更新
- Terraform / IaC 资源调整
- 网关路由和鉴权规则变更

这些变更分散在不同系统中，信息碎片化严重。实际发布前的风险审查依赖资深工程师经验，常见痛点包括：
- 审查耗时长，严重依赖专家个人经验
- 风险判断缺乏统一标准
- 变更影响面分析不充分
- 容易忽视历史事故、依赖链、流量高峰期等隐性上下文
- 回滚预案不充分，出现问题时缺少结构化止损流程
- 审批意见不留痕，难以评估审查质量

### 2.2 问题定义
需要一个系统，在发布流程中对“待上线变更”进行结构化分析，自动回答以下问题：
1. 这次变更改了什么，属于哪类变更？
2. 变更影响哪些服务、指标、依赖和业务链路？
3. 过往是否有类似变更导致事故？
4. 是否存在高风险信号，例如核心链路改动、不可逆 SQL、协议不兼容、配置越权、资源不足？
5. 应该直接发布、灰度发布、延期发布，还是升级人工审批？
6. 如果失败，回滚步骤和监控观察点是什么？
7. 这次审查的证据链和结论是否可追溯、可复盘、可评测？

### 2.3 核心设计原则
- **证据优先，而不是幻觉优先**：系统必须展示结论来源。
- **人机协同，而不是全自动放行**：高风险场景必须人工确认。
- **结构化输出，而不是纯自然语言摘要**：结论需便于嵌入审批流。
- **可评测，而不是单次 demo**：必须支持误报、漏报、采纳率等指标闭环。
- **可回滚，而不是只给建议**：风险审查必须产出止损方案。
- **面向真实系统，而不是通用聊天**：聚焦发布系统和变更治理。

---

## 3. 产品目标与非目标

### 3.1 产品目标
1. 在发布前对变更做自动化风险审计。
2. 统一收集发布相关上下文并生成可解释结论。
3. 显著降低人工审查时间。
4. 提升高风险变更召回率。
5. 为发布审批、灰度策略、回滚方案提供结构化建议。
6. 建立人机协同审查闭环与评测体系。

### 3.2 非目标
1. 不替代 CI/CD 平台本身。
2. 不直接执行生产发布动作。
3. 不在初期承担根因分析全自动闭环。
4. 不追求通用 Agent 万能问答。
5. 不在初期做多租户商业化产品的全部能力。

---

## 4. 典型使用场景

### 4.1 场景 A：PR 合并前风险审计
开发者提交 PR，系统自动：
- 分析 diff 类型
- 拉取历史相似改动
- 查依赖服务
- 检索历史事故与 runbook
- 给出风险等级和审批建议

### 4.2 场景 B：SQL migration 发布前审计
系统自动识别：
- DDL / DML 类型
- 是否存在锁表风险
- 是否不可逆
- 是否跨大表执行
- 是否需要业务低峰期执行
- 是否必须先扩容后切流

### 4.3 场景 C：K8s / Helm 配置发布前审计
系统自动识别：
- 副本数调整是否低于安全阈值
- HPA / resource limit 是否会引起抖动
- readiness / liveness 是否缺失
- ingress / gateway 规则变更是否影响入口流量

### 4.4 场景 D：Terraform 变更前审计
识别资源删除、替换、权限放宽、网络暴露、状态漂移等高风险信号，并建议需要审批的角色。

### 4.5 场景 E：网关规则与鉴权策略变更审计
识别 API 路由、限流、Header 透传、鉴权配置、流量切分策略的变化，预判是否可能引起越权、熔断、放量或业务路由偏差。

---

## 5. 用户角色

### 5.1 变更提交者
希望快速知道这次改动风险在哪里、如何补充回滚和灰度策略。

### 5.2 审批人 / Reviewer / TL
希望快速获得可信的风险摘要、证据链和建议动作，减少逐行 diff 阅读成本。

### 5.3 SRE / 发布经理
希望在上线前统一审计高风险发布，保证有监控指标、观察窗口和止损策略。

### 5.4 平台管理员
负责规则库、工具接入、风险阈值、权限治理、审计日志与评测数据管理。

---

## 6. 总体方案概述

### 6.1 系统定位
Agentic Change Review Copilot 是一个挂接在发布系统前置环节的智能审查层。它不是简单的 LLM 问答机器人，而是一个带工具调用、证据整合、结构化决策输出和人机协同机制的 Agent 系统。

### 6.2 总体工作流
1. 接收待审变更对象
2. 对变更进行标准化解析
3. 调用上下文采集工具
4. 进行规则校验和风险特征提取
5. 由 Agent 编排分析步骤
6. 生成风险评分与证据链
7. 输出审批建议、灰度方案、观察指标和回滚预案
8. 高风险场景升级人工审批
9. 记录结果进入审计与评测系统

### 6.3 核心价值
- 降低审查认知负担
- 沉淀专家经验为规则和知识库
- 让 AI 决策过程可解释、可追溯
- 构建发布审查数据闭环

---

## 7. 系统边界与集成点

### 7.1 上游输入系统
- GitHub / GitLab / Gitea
- CI 系统（Jenkins / GitHub Actions / GitLab CI）
- 发布平台
- 配置中心
- 数据库变更平台
- Terraform / IaC pipeline
- API 网关配置平台

### 7.2 下游输出系统
- 发布审批页面
- 企业 IM（钉钉 / 飞书 / Slack）
- 工单系统
- 审计日志系统
- 风险看板

### 7.3 外部依赖
- LLM Gateway / Model Router
- 向量检索 / 文档检索系统
- Prometheus / Grafana
- Git 服务
- CMDB / Service Catalog
- 事故库 / Runbook 库 / Wiki
- 特征规则引擎

### 7.4 系统边界声明
本系统输出的是“审查建议与风险证据”，不是最终生产发布执行器。任何高风险变更必须由人类审批决定是否放行。

---

## 8. 详细功能设计

## 8.1 变更接入层

### 8.1.1 输入对象类型
- Pull Request / Merge Request
- SQL migration 文件
- Kubernetes YAML / Helm diff
- Terraform plan
- 网关配置 diff
- 配置中心参数 diff

### 8.1.2 标准化解析结果
统一抽象为 `ChangeBundle`：
- change_id
- change_type
- repo / service / environment
- author / reviewer / submit_time
- diff summary
- file list
- impacted components
- release target

### 8.1.3 为什么要做标准化
因为后续 Agent 不应直接依赖每种来源的原始格式，而应围绕统一变更语义做分析和工具调用。

---

## 8.2 变更理解模块（Change Understanding Engine）

### 8.2.1 职责
从原始 diff 中抽取结构化语义：
- 改了什么
- 改动属于逻辑 / 配置 / 数据 / 资源 / 权限 / 网络 / 路由哪一类
- 是否触发高风险关键词
- 是否涉及跨边界兼容性问题

### 8.2.2 输入
- 原始 diff
- 文件路径
- commit message / PR description
- 服务元数据

### 8.2.3 输出
- change_summary
- semantic_tags
- risk_hints
- impacted_services
- potential_failure_modes

### 8.2.4 技术方案
采用“规则 + AST / 结构解析 + LLM 摘要”的混合方式：
- 对 SQL / YAML / Terraform 先做结构解析
- 对代码 diff 可做路径、函数签名、配置项提取
- 最后由 LLM 做高层语义归纳

### 8.2.5 示例标签
- schema_change
- auth_logic_changed
- traffic_routing_changed
- irreversible_operation
- core_path_modified
- cross_service_contract_change
- resource_limit_lowered
- public_exposure_risk

---

## 8.3 上下文采集模块（Context Collector）

### 8.3.1 职责
围绕当前变更自动补齐风险审查所需上下文。

### 8.3.2 可调用工具
1. **Git 历史工具**
   - 相似文件历史改动
   - blame / owner
   - 最近相关 PR
2. **服务依赖图工具**
   - 上游调用方
   - 下游依赖
   - 核心路径标识
3. **监控工具**
   - 当前错误率
   - QPS / P95 / CPU / Memory
   - 最近 24h / 7d 波动
4. **事故检索工具**
   - 历史 P0 / P1 / P2 事故
   - 相似关键字检索
5. **Runbook / Wiki 检索工具**
   - 标准发布步骤
   - 回滚指引
   - 风险提示
6. **发布日历工具**
   - 高峰时段
   - 黑名单时间窗
7. **审批策略工具**
   - 哪类变更需要谁审批

### 8.3.3 输出
统一为 `EvidencePack`：
- related_incidents
- service_owners
- dependency_edges
- recent_metrics
- runbook_refs
- similar_changes
- policy_requirements

### 8.3.4 设计重点
证据必须带来源、时间戳、可信度和引用片段，便于后续展示证据链。

---

## 8.4 风险特征抽取模块（Risk Signal Extractor）

### 8.4.1 职责
从变更内容和上下文中提取风险信号。

### 8.4.2 高风险信号样例
- 删除或替换生产资源
- 修改鉴权逻辑
- 核心表 DDL
- 网关入口规则变化
- 缩容或降低资源限制
- 缺失 readiness / rollback
- 协议字段删除
- 未覆盖测试
- 近期同类事故频发
- 当前环境已处于高错误率

### 8.4.3 输出
- signal_name
- severity
- evidence_refs
- confidence

### 8.4.4 说明
该模块尽量不依赖大模型主观判断，而是优先用 deterministic 逻辑产出风险特征，减少不稳定性。

---

## 8.5 Agent 编排模块（Risk Review Agent）

### 8.5.1 职责
协调各模块完成“计划—取证—判断—建议—升级人工”的完整审查过程。

### 8.5.2 Agent 子任务拆分
1. 判断此次变更属于哪类审查模板
2. 选择需要调用的工具
3. 汇总证据并生成中间判断
4. 判断是否需要补充取证
5. 输出结构化结论
6. 决定是否升级人工

### 8.5.3 推荐的执行模式
采用**有限步骤的可控 Agent**，而不是无限自由规划。
建议模式：
- 基于状态机 / DAG 的 workflow
- 每一步有明确输入输出
- 限制最多工具调用次数
- 中间状态可追踪

### 8.5.4 为什么不做全自由 Agent
变更审计属于高可信场景，需要：
- 稳定性
- 可复现性
- 审计合规
- 输出可控
因此更适合 workflow-based agent，而不是开放式自治代理。

---

## 8.6 风险评分模块（Risk Scoring Engine）

### 8.6.1 职责
根据风险信号、上下文证据和规则策略，为变更给出风险等级。

### 8.6.2 风险等级
- Low：可直接审批
- Medium：建议补充说明或低比例灰度
- High：需资深审批 / 限制发布时段 / 强制灰度
- Critical：默认阻断，必须人工豁免

### 8.6.3 评分方式
建议采用混合模式：
- 硬规则分值
- 风险特征加权
- LLM 辅助解释而非直接打分主导

### 8.6.4 样例规则
- 核心鉴权逻辑变更：+30
- 不可逆 SQL：+40
- 核心链路服务：+20
- 缺少回滚方案：+15
- 当前环境错误率异常：+10
- 低峰期发布：-10
- 完整灰度方案：-8

### 8.6.5 输出
- score
- risk_level
- main_reasons
- score_breakdown

---

## 8.7 审批建议模块（Approval Advisor）

### 8.7.1 职责
将风险结果映射为可执行的审批动作。

### 8.7.2 输出内容
- 是否允许进入发布流程
- 需要哪些审批人
- 建议发布窗口
- 建议灰度比例
- 需要观察哪些指标
- 是否要求值班 SRE 在场

### 8.7.3 样例输出
- “高风险，建议在非高峰期以 5% -> 20% -> 50% -> 100% 灰度方式发布。”
- “涉及鉴权变更，必须由网关 owner 和安全 reviewer 双审批。”
- “涉及核心表 DDL，建议先 shadow 验证并确认回滚脚本可执行。”

---

## 8.8 回滚预案生成模块（Rollback Planner）

### 8.8.1 职责
为变更生成可执行、结构化的回滚建议。

### 8.8.2 输出模板
- 回滚触发条件
- 回滚步骤
- 负责人
- 所需脚本 / 命令
- 观察指标
- 最大容忍时间窗口

### 8.8.3 设计重点
回滚方案必须引用：
- 历史 runbook
- 已存在脚本
- 配置版本号
- 镜像 tag
而不是只生成泛泛而谈的自然语言。

---

## 8.9 人工升级与协同模块（Human-in-the-Loop）

### 8.9.1 触发条件
- 风险等级为 High / Critical
- 证据不足
- 模型置信度低
- 多工具结果冲突
- 涉及权限 / 安全 / 资金 / 数据删除

### 8.9.2 升级方式
- 指定 reviewer
- 发送 IM 通知
- 附带证据摘要
- 要求填写审批意见

### 8.9.3 人工反馈记录
- approve / reject / override
- 原因说明
- 是否采纳 Agent 建议

### 8.9.4 价值
这是系统评测闭环的重要数据来源，也体现 enterprise agent 的可信控制边界。

---

## 8.10 结果展示模块（Review Console）

### 8.10.1 页面展示信息
- 变更摘要
- 风险等级
- 主要风险点
- 证据列表
- 相似历史事故
- 建议审批链
- 灰度策略
- 回滚预案
- Agent 推理轨迹摘要

### 8.10.2 UI 设计原则
- 先结论，后证据
- 证据可点击溯源
- 风险点按严重程度排序
- 明确显示“模型建议，不代表自动批准”

---

## 8.11 审计与评测模块（Audit & Evaluation Hub）

### 8.11.1 记录内容
- 输入变更对象
- 调用过的工具
- 中间结论
- 最终输出
- 人工审批结果
- 上线结果
- 是否发生事故

### 8.11.2 核心指标
- 高风险召回率
- 误报率
- 漏报率
- 建议采纳率
- 人工审查耗时下降比例
- 平均工具调用数
- 平均 token 成本
- 审查 P95 延迟

### 8.11.3 离线评测集
构建 `Change Review Benchmark`：
- 正常变更样本
- 高风险变更样本
- 历史事故样本
- 易混淆中风险样本

### 8.11.4 在线评估
- A/B 测试：人工 only vs 人工 + Agent
- 审批时长对比
- 事故前置拦截数
- override 原因分类

---

## 9. 系统架构设计

## 9.1 逻辑架构
系统可分为六层：
1. 接入层
2. 变更语义层
3. 上下文工具层
4. Agent 编排层
5. 决策输出层
6. 审计评测层

## 9.2 组件划分
- Change Ingestion Service
- Change Normalizer
- Change Understanding Engine
- Context Collector
- Risk Signal Extractor
- Workflow Agent Orchestrator
- Risk Scoring Engine
- Approval Advisor
- Rollback Planner
- Review API / Console
- Audit Log Service
- Evaluation Service
- Knowledge Retrieval Service
- LLM Gateway

## 9.3 建议部署方式
- 核心服务以 Go 实现：接入层、API、规则引擎、任务编排、审计服务
- Agent workflow / evaluation / 部分检索流水线可用 Python 实现
- 使用消息队列解耦异步审查任务
- 使用对象存储 / 文档库保留原始审查记录

---

## 10. 数据模型设计

## 10.1 ChangeBundle
- change_id
- source_type
- repo
- service
- environment
- author
- reviewers
- created_at
- diff_payload
- semantic_summary
- tags

## 10.2 EvidenceItem
- evidence_id
- type
- source
- title
- content_snippet
- reference_url
- timestamp
- confidence

## 10.3 RiskSignal
- signal_id
- name
- severity
- triggered_by
- evidence_ids
- explanation

## 10.4 ReviewDecision
- decision_id
- change_id
- score
- risk_level
- recommendation
- approvers_required
- can_release
- rollout_strategy
- rollback_plan_id

## 10.5 HumanFeedback
- reviewer
- decision
- override_flag
- reason
- created_at

## 10.6 EvaluationRecord
- change_id
- agent_risk_level
- final_human_decision
- release_outcome
- incident_flag
- metrics_snapshot

---

## 11. 关键流程设计

## 11.1 主流程：一次完整审查
1. 发布系统提交 change_id 和原始变更
2. Ingestion Service 标准化为 ChangeBundle
3. Change Understanding Engine 解析改动语义
4. Agent 根据变更类型选取工具清单
5. Context Collector 拉取证据
6. Risk Signal Extractor 产出风险信号
7. Risk Scoring Engine 生成分数与等级
8. Approval Advisor 输出审批建议
9. Rollback Planner 生成回滚预案
10. Console 展示结果并进入人工审批
11. 审计与评测模块记录全链路数据

## 11.2 高风险升级流程
1. 风险达到 High
2. 自动阻止进入下一发布环节
3. 通知指定 reviewer
4. 展示关键证据和建议
5. reviewer 选择 approve / reject / override
6. 记录 override 理由

## 11.3 证据补采流程
1. Agent 初步判断证据不足
2. 增量调用更多工具
3. 若仍不足则标记 low confidence
4. 强制人工处理

---

## 12. Agent 设计细节

## 12.1 Agent 不是单一大模型调用
该系统中的 Agent 是“受控工作流 + 工具选择 + 结构化输出”的组合。

## 12.2 推荐状态机
- INIT
- UNDERSTAND_CHANGE
- COLLECT_CONTEXT
- EXTRACT_SIGNALS
- SCORE_RISK
- GENERATE_RECOMMENDATION
- PLAN_ROLLBACK
- REQUIRE_HUMAN_REVIEW
- DONE

## 12.3 工具调用策略
- 每类变更有预定义工具模板
- 允许少量按需追加调用
- 限制最大调用次数，避免成本失控
- 每个工具结果都写入证据缓存

## 12.4 Prompt 策略
Prompt 不负责决定一切，而是负责：
- 解释当前状态
- 指明可用工具
- 约束输出 schema
- 强调必须引用证据
- 要求在证据不足时输出“不确定”

## 12.5 输出 Schema
- risk_summary
- risk_level
- signals
- evidence_refs
- rollout_recommendation
- rollback_plan
- human_review_required
- confidence

---

## 13. 知识库与检索设计

## 13.1 知识源
- 历史事故复盘
- 发布 runbook
- 值班手册
- 服务说明文档
- 数据库变更规范
- 安全基线
- 审批策略文档

## 13.2 检索策略
- 关键词检索 + 向量检索混合
- 以服务名、模块名、错误码、字段名作为检索锚点
- 检索结果必须带版本与时间信息

## 13.3 检索风险
避免把过期 runbook 或已废弃架构文档作为高置信证据，需要加入时效性过滤和权重控制。

---

## 14. LLM 与模型策略

## 14.1 模型职责划分
- 大模型用于语义总结、证据归纳、解释生成、策略草拟
- 规则引擎负责硬约束、高风险检测、审批门槛
- 检索系统负责提供可验证上下文

## 14.2 模型接入建议
通过现有 LLM Gateway 接入：
- 多模型路由
- 成本统计
- 缓存
- 限流与 fallback
- 审计日志

## 14.3 模型选择原则
- 审查摘要场景用高性价比模型
- 高风险解释场景用高质量模型
- 输出必须强约束为 JSON / structured output

## 14.4 模型安全要求
- 禁止模型直接执行敏感操作
- 敏感信息脱敏后再送入模型
- 关键审批结论必须可解释并可人工覆盖

---

## 15. 安全与合规设计

### 15.1 权限边界
- Agent 仅读取，不直接改写发布配置
- 工具按最小权限开放
- 敏感资源查询需要审计

### 15.2 数据脱敏
- SQL、配置、日志中的敏感字段需屏蔽
- 生产凭据和密钥不可进入 prompt

### 15.3 审计要求
- 每次工具调用均记录
- 每次结论都可回溯至证据
- 每次人工 override 都需留痕

### 15.4 失败安全原则
当证据不足、模型异常、外部依赖超时时，默认倾向保守，升级人工审批，而不是自动放行。

---

## 16. 可观测性设计

### 16.1 系统级指标
- 审查请求数
- 成功率
- 平均延迟
- P95 延迟
- 工具调用成功率
- 模型调用成本

### 16.2 业务指标
- 风险等级分布
- 高风险召回率
- 误报率 / 漏报率
- override 率
- 灰度建议采纳率
- 回滚预案点击率

### 16.3 Trace 设计
一次审查需有完整 trace id，串联：
- 输入变更
- 工具调用
- 中间状态
- 最终决策
- 人工审批

---

## 17. MVP 范围设计

### 17.1 第一阶段只支持
- PR diff
- SQL migration
- K8s YAML / Helm diff

### 17.2 第一阶段工具
- Git 历史
- 服务依赖图
- 事故检索
- Runbook 检索
- Prometheus 指标快照

### 17.3 第一阶段输出
- 风险等级
- 主要风险点
- 审批建议
- 灰度策略
- 回滚预案

### 17.4 第一阶段不做
- Terraform 复杂资源图分析
- 自动发布执行
- 多组织多租户
- 自主长链推理

---

## 18. 分阶段落地路线

## 18.1 Phase 1：单机场景验证
目标：做出可演示闭环
- 接收 PR / SQL / YAML diff
- 实现规则 + LLM 基础审查
- 接入 2 到 3 个工具
- 页面展示审查结果

## 18.2 Phase 2：证据链与评测闭环
目标：从 demo 走向工程项目
- 建立审计日志
- 建立历史样本评测集
- 引入人工 override 记录
- 做风险召回率与误报分析

## 18.3 Phase 3：接入发布流
目标：体现真实工程价值
- 接 CI / 发布平台 webhook
- 加入审批流和 IM 通知
- 支持灰度建议模板

## 18.4 Phase 4：平台化扩展
目标：形成“Copilot 系统”而不只是单一 agent
- 多变更类型扩展
- 多模型策略
- 多规则包
- 多团队接入

---

## 19. 面试叙事建议

### 19.1 你要强调的不是“我做了个 Agent”
而是：
“我做了一个能真正嵌入发布系统、对高风险变更做证据化审查的人机协同 Agentic 系统。”

### 19.2 最能打动面试官的点
- 不是通用问答，而是高价值企业流程
- 有明确的人机协同边界
- 有审计、评测、回滚和治理思路
- 能和你现有网关 / 平台背景强耦合

### 19.3 简历项目名建议
- 面向发布系统的 Agentic Change Review Copilot
- 智能变更风险审计 Agent
- 发布前变更证据审查与回滚规划系统

### 19.4 简历表述示例
构建面向代码、SQL 与 K8s 配置变更的 Agentic 审查系统，基于工具调用、证据检索、风险规则与结构化输出，实现发布前风险评估、审批建议、灰度策略与回滚预案自动生成，并通过人工 override 与事故结果建立评测闭环。

---

## 20. 可能的亮点指标

可在项目完成后补齐以下量化指标：
- 将单次人工审查时间从 15 分钟降至 4 分钟
- 在历史样本上达到 85% 高风险召回率
- 误报率控制在 12% 以下
- 平均每次审查工具调用 4.2 次
- 审查 P95 耗时 8 秒
- 单次审查 token 成本低于指定阈值

---

## 21. 未来扩展方向

### 21.1 从“审查”扩展到“上线守护”
在发布后继续跟踪指标，结合变更上下文辅助做异常归因与止血建议。

### 21.2 多 Agent 协作
- SQL specialist agent
- Security review agent
- Gateway config review agent
- SRE policy agent

### 21.3 组织经验沉淀
将人工 override、事故复盘、审批意见沉淀为规则包和知识库，持续提高系统命中率。

---

## 22. 风险与挑战

### 22.1 数据源碎片化
需要解决多个系统接入一致性问题。

### 22.2 过期文档误导
知识库检索必须带时效性控制。

### 22.3 LLM 幻觉风险
必须坚持证据优先与结构化输出。

### 22.4 评测不容易
需要构建带标签的历史变更集。

### 22.5 人工信任建立慢
早期建议以“辅助审批”而非“自动阻断”切入。

---

## 23. 结论

Agentic Change Review Copilot 的核心不是“让模型替你看 diff”，而是：

**把发布前审查这件依赖专家经验、缺乏证据链、缺少统一标准的高风险流程，重构为一个可检索、可解释、可评测、可升级人工、可沉淀组织经验的智能协作系统。**

对于求职场景，它相比通用 Agent demo 更具区分度，因为它：
- 紧贴真实企业流程
- 具备强工程味道
- 能同时体现平台能力与 Agent 能力
- 能自然连接你的网关、微服务、K8s 与治理背景

这不是一个“会调 API 的 Agent demo”，而是一个可进化为真实内部平台的系统构想。

---

## 24. 可开发版本设计（Developer-Ready Spec）

本章节将系统从“方案级设计”继续下钻到“可直接拆任务开发”的粒度，重点补齐：
- 服务拆分
- API 契约
- 数据表设计
- 时序设计
- 任务状态机
- 模块边界
- MVP 实施细节

### 24.1 建议技术栈

#### 24.1.1 服务端语言
- **Go**：
  - Review API
  - Change Ingestion Service
  - Risk Rules Engine
  - Audit Log Service
  - Review Task Scheduler
  - Notification Service
- **Python**：
  - Agent Workflow Runner
  - Retrieval Pipeline
  - Evaluation Jobs
  - Benchmark 构建脚本

#### 24.1.2 存储与中间件
- PostgreSQL：业务主存储
- Redis：缓存、任务幂等键、短期上下文缓存
- Kafka / RabbitMQ：异步事件分发
- MinIO / S3：原始 diff、审查快照、评测工件存储
- OpenSearch / Elasticsearch：全文检索与审计检索
- 向量库（pgvector / Milvus）：runbook / 事故知识检索

#### 24.1.3 基础设施
- Kubernetes：部署平台
- Prometheus + Grafana：监控
- Tempo / Jaeger：Tracing
- Loki / ELK：日志

#### 24.1.4 模型接入
通过你现有的 LLM Gateway：
- 多模型路由
- structured output
- request/semantic cache
- token 审计
- fallback

---

## 25. 服务拆分与职责边界

### 25.1 Review API Service
对外统一入口，负责：
- 接收审查请求
- 查询审查结果
- 人工审批提交
- 审查任务取消 / 重试
- webhook 接入

### 25.2 Change Ingestion Service
负责：
- 接收 PR / SQL / YAML / Terraform 等原始输入
- 拉取外部系统元数据
- 标准化为 ChangeBundle
- 生成 change snapshot

### 25.3 Review Orchestrator
负责：
- 创建审查任务
- 驱动状态机流转
- 调度理解、检索、打分、建议、通知等子流程
- 管理重试、超时和失败恢复

### 25.4 Agent Workflow Runner
负责：
- 受控执行 agent workflow
- 工具调用编排
- 结构化输出生成
- 证据引用校验

### 25.5 Context Retrieval Service
负责：
- 统一封装 Git / 依赖图 / 事故库 / Runbook / Metrics 查询
- 对外暴露标准工具接口
- 做结果归一化与缓存

### 25.6 Risk Rule Engine
负责：
- 执行静态规则
- 风险信号抽取
- 计算基础评分
- 策略阈值判断

### 25.7 Recommendation Engine
负责：
- 审批建议
- 灰度策略
- 指标观察建议
- 回滚预案模板拼装

### 25.8 Audit & Evaluation Service
负责：
- 审查过程全链路留痕
- 线上结果采集
- 离线评测集管理
- 指标报表

### 25.9 Notification Service
负责：
- 飞书 / Slack / 钉钉消息通知
- 高风险升级通知
- 审批待办提醒

---

## 26. 运行时状态机设计

### 26.1 Review Task 状态定义
- `CREATED`：任务已创建
- `NORMALIZED`：变更已标准化
- `UNDERSTOOD`：变更语义解析完成
- `COLLECTING_CONTEXT`：正在采集上下文
- `CONTEXT_READY`：证据准备完成
- `SIGNALS_EXTRACTED`：风险信号已抽取
- `SCORED`：风险评分完成
- `RECOMMENDED`：审批建议与回滚方案生成完成
- `WAITING_HUMAN_REVIEW`：等待人工审批
- `APPROVED`：人工批准
- `REJECTED`：人工拒绝
- `OVERRIDDEN`：人工覆盖 Agent 建议
- `FAILED`：任务失败
- `CANCELLED`：任务取消

### 26.2 状态迁移规则
- `CREATED -> NORMALIZED`
- `NORMALIZED -> UNDERSTOOD`
- `UNDERSTOOD -> COLLECTING_CONTEXT`
- `COLLECTING_CONTEXT -> CONTEXT_READY`
- `CONTEXT_READY -> SIGNALS_EXTRACTED`
- `SIGNALS_EXTRACTED -> SCORED`
- `SCORED -> RECOMMENDED`
- `RECOMMENDED -> WAITING_HUMAN_REVIEW`（高风险/低置信度）
- `RECOMMENDED -> APPROVED`（低风险自动通过，仅限 demo / sandbox）
- 任意状态可进入 `FAILED` / `CANCELLED`

### 26.3 失败恢复策略
- 外部工具超时：允许有限重试
- LLM 调用失败：切换 fallback 模型
- 检索无结果：进入 low-confidence 分支
- 数据源部分不可用：降级继续，但标记证据不完整

---

## 27. 核心 API 设计

以下 API 面向 MVP，采用 REST 为主，后续可补充 gRPC 内部调用。

### 27.1 创建审查任务
`POST /api/v1/reviews`

#### Request
```json
{
  "source_type": "pull_request",
  "source_id": "pr_12345",
  "repo": "gateway-service",
  "service": "api-gateway",
  "environment": "prod",
  "triggered_by": "github_webhook",
  "payload": {
    "title": "refactor auth middleware and update route rules",
    "author": "alice",
    "base_commit": "abc",
    "head_commit": "def",
    "diff_url": "https://...",
    "metadata": {
      "pr_url": "https://...",
      "labels": ["release-blocking"]
    }
  }
}
```

#### Response
```json
{
  "review_id": "rvw_001",
  "task_id": "task_001",
  "status": "CREATED"
}
```

### 27.2 查询审查详情
`GET /api/v1/reviews/{review_id}`

#### Response
```json
{
  "review_id": "rvw_001",
  "status": "WAITING_HUMAN_REVIEW",
  "risk_level": "HIGH",
  "score": 78,
  "confidence": 0.82,
  "summary": "涉及鉴权链路与入口路由规则变更，存在高风险。",
  "signals": [
    {
      "name": "auth_logic_changed",
      "severity": "high",
      "explanation": "修改了认证中间件核心逻辑"
    }
  ],
  "approval_recommendation": {
    "required_reviewers": ["gateway_owner", "security_reviewer"],
    "release_window": "off_peak",
    "rollout_strategy": "5%-20%-50%-100%"
  },
  "rollback_plan": {
    "trigger_condition": "401 ratio > 3x baseline for 5 min",
    "steps": [
      "rollback deployment to previous revision",
      "restore gateway routing config version v102",
      "verify login success rate"
    ]
  },
  "evidence": [
    {
      "type": "incident",
      "title": "2025-11 auth routing regression",
      "source": "incident_db"
    }
  ]
}
```

### 27.3 提交人工审批结果
`POST /api/v1/reviews/{review_id}/human-decision`

#### Request
```json
{
  "reviewer": "bob",
  "decision": "override",
  "reason": "业务窗口有限，已安排值班和灰度监控",
  "notify": true
}
```

### 27.4 获取审查任务时间线
`GET /api/v1/reviews/{review_id}/timeline`

返回任务状态流转、工具调用摘要、耗时统计。

### 27.5 重试失败任务
`POST /api/v1/reviews/{review_id}/retry`

### 27.6 查询评测统计
`GET /api/v1/evaluations/metrics?from=2026-01-01&to=2026-01-31`

---

## 28. 内部模块接口契约

### 28.1 Change Ingestion -> Orchestrator
```json
{
  "change_bundle": {
    "change_id": "chg_001",
    "source_type": "pull_request",
    "repo": "gateway-service",
    "service": "api-gateway",
    "environment": "prod",
    "author": "alice",
    "raw_diff_ref": "s3://bucket/chg_001.diff"
  }
}
```

### 28.2 Orchestrator -> Context Retrieval
```json
{
  "review_id": "rvw_001",
  "change_type": "gateway_config_change",
  "service": "api-gateway",
  "tags": ["auth_logic_changed", "traffic_routing_changed"],
  "queries": [
    "recent incidents on auth routing for api-gateway",
    "runbook for gateway route rollback"
  ]
}
```

### 28.3 Rule Engine 输出
```json
{
  "signals": [
    {
      "name": "traffic_routing_changed",
      "severity": "high",
      "score_delta": 18,
      "evidence_refs": ["ev_101"]
    }
  ],
  "base_score": 42
}
```

### 28.4 Agent Workflow 输出
```json
{
  "summary": "本次变更涉及认证中间件与网关路由，影响入口流量与鉴权成功率。",
  "risk_hypotheses": [
    "可能导致部分路由进入错误认证链路",
    "高峰期发布可能放大影响面"
  ],
  "missing_information": [],
  "confidence": 0.82,
  "evidence_refs": ["ev_101", "ev_102", "ev_204"]
}
```

---

## 29. 数据库表设计（MVP）

### 29.1 reviews
```sql
CREATE TABLE reviews (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) UNIQUE NOT NULL,
  change_id VARCHAR(64) NOT NULL,
  source_type VARCHAR(32) NOT NULL,
  repo VARCHAR(255),
  service VARCHAR(255),
  environment VARCHAR(64),
  author VARCHAR(128),
  status VARCHAR(64) NOT NULL,
  risk_level VARCHAR(32),
  score INT,
  confidence NUMERIC(5,4),
  summary TEXT,
  can_release BOOLEAN,
  human_review_required BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 29.2 review_tasks
```sql
CREATE TABLE review_tasks (
  id BIGSERIAL PRIMARY KEY,
  task_id VARCHAR(64) UNIQUE NOT NULL,
  review_id VARCHAR(64) NOT NULL,
  current_state VARCHAR(64) NOT NULL,
  retry_count INT NOT NULL DEFAULT 0,
  last_error TEXT,
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 29.3 change_snapshots
```sql
CREATE TABLE change_snapshots (
  id BIGSERIAL PRIMARY KEY,
  change_id VARCHAR(64) NOT NULL,
  raw_payload_ref TEXT NOT NULL,
  normalized_payload JSONB NOT NULL,
  semantic_tags JSONB,
  file_list JSONB,
  diff_stats JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 29.4 evidence_items
```sql
CREATE TABLE evidence_items (
  id BIGSERIAL PRIMARY KEY,
  evidence_id VARCHAR(64) UNIQUE NOT NULL,
  review_id VARCHAR(64) NOT NULL,
  evidence_type VARCHAR(64) NOT NULL,
  source VARCHAR(128) NOT NULL,
  title TEXT,
  content_snippet TEXT,
  reference_url TEXT,
  confidence NUMERIC(5,4),
  metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 29.5 risk_signals
```sql
CREATE TABLE risk_signals (
  id BIGSERIAL PRIMARY KEY,
  signal_id VARCHAR(64) UNIQUE NOT NULL,
  review_id VARCHAR(64) NOT NULL,
  signal_name VARCHAR(128) NOT NULL,
  severity VARCHAR(32) NOT NULL,
  score_delta INT NOT NULL,
  explanation TEXT,
  evidence_refs JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 29.6 recommendations
```sql
CREATE TABLE recommendations (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) NOT NULL,
  recommendation JSONB NOT NULL,
  rollback_plan JSONB,
  rollout_strategy JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 29.7 human_decisions
```sql
CREATE TABLE human_decisions (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) NOT NULL,
  reviewer VARCHAR(128) NOT NULL,
  decision VARCHAR(32) NOT NULL,
  reason TEXT,
  override_flag BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 29.8 audit_events
```sql
CREATE TABLE audit_events (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  payload JSONB NOT NULL,
  trace_id VARCHAR(128),
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 29.9 evaluation_records
```sql
CREATE TABLE evaluation_records (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) NOT NULL,
  final_human_decision VARCHAR(32),
  release_outcome VARCHAR(32),
  incident_flag BOOLEAN,
  outcome_metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

---

## 30. 核心时序设计

### 30.1 PR 触发审查时序
1. GitHub/GitLab webhook 推送 PR 事件
2. Review API 校验签名并创建 `review_id`
3. Change Ingestion 拉取 diff、commit、PR metadata
4. 标准化后写入 `change_snapshots`
5. Orchestrator 创建 `review_tasks`
6. Agent Workflow Runner 执行“理解 -> 检索 -> 打分 -> 建议”链路
7. 写入 `evidence_items`、`risk_signals`、`recommendations`
8. 若高风险，Notification Service 通知 reviewer
9. 审批人进入 Console 查看结果并提交 decision
10. Audit Service 记录全链路事件

### 30.2 文本版时序图
```text
Git Webhook
   -> Review API
   -> Change Ingestion
   -> Review Orchestrator
   -> Change Understanding Engine
   -> Context Retrieval Service
   -> Risk Rule Engine
   -> Agent Workflow Runner
   -> Recommendation Engine
   -> Review API / Console
   -> Human Reviewer
   -> Audit & Evaluation Service
```

### 30.3 SQL migration 审查时序差异
与 PR 审查大体一致，但额外增加：
- SQL parser
- 表元数据查询
- 慢 SQL 风险规则
- 不可逆操作检测

---

## 31. 工具层设计

### 31.1 工具接口统一规范
每个工具统一输出：
```json
{
  "tool_name": "incident_search",
  "status": "success",
  "latency_ms": 120,
  "results": [...],
  "error": null,
  "retrieved_at": "2026-03-17T12:00:00Z"
}
```

### 31.2 MVP 工具清单
- `git_diff_fetch`
- `git_history_search`
- `incident_search`
- `runbook_search`
- `service_dependency_lookup`
- `metrics_snapshot`
- `release_window_lookup`
- `owner_lookup`

### 31.3 工具调用守则
- 单次审查最多 6 个外部工具
- 同类工具结果支持缓存
- 工具失败必须显式记录
- 无结果不等于无风险

### 31.4 工具适配器模式
建议为每类外部系统写 adapter：
- GitAdapter
- IncidentAdapter
- MetricsAdapter
- WikiAdapter
- CMDBAdapter

这样后续替换 GitHub / GitLab 或飞书文档 / Confluence 更容易。

---

## 32. Agent Workflow 细化

### 32.1 Workflow 模式
推荐使用 DAG / state machine，而非自由 ReAct 长链。

### 32.2 节点定义
- Node A：变更分类
- Node B：工具计划生成
- Node C：上下文采集
- Node D：风险信号整合
- Node E：结论生成
- Node F：建议与回滚生成
- Node G：置信度判断与人工升级

### 32.3 每个节点输入输出必须结构化
例如 Node E 输入：
- semantic_tags
- signals
- evidence_refs
- score_breakdown

输出：
- summary
- top_risks
- unsupported_claims
- confidence

### 32.4 证据约束机制
模型输出的每条关键结论必须绑定至少一个 `evidence_ref`；没有证据的结论只能放进 `hypothesis` 字段，不能进入最终审批建议。

### 32.5 幻觉抑制策略
- 强制 structured output
- 只允许从 evidence 列表中引用材料
- “证据不足”作为合法输出
- 对最终 recommendation 做后置校验

---

## 33. 规则引擎设计

### 33.1 规则分类
- 通用规则：适用于所有变更
- 变更类型规则：SQL / K8s / 网关 / Terraform
- 服务特定规则：某核心服务专属规则
- 环境规则：prod / staging 差异化阈值

### 33.2 规则示例
```yaml
rule_id: gateway_auth_change_prod
applies_to:
  change_type: gateway_config_change
  environment: prod
conditions:
  - tag == "auth_logic_changed"
  - tag == "traffic_routing_changed"
actions:
  score_delta: 25
  risk_level_floor: HIGH
  require_reviewers:
    - gateway_owner
    - security_reviewer
```

### 33.3 规则执行顺序
1. 基础解析规则
2. 风险信号规则
3. 打分规则
4. 审批策略规则
5. 回滚模板规则

### 33.4 规则管理建议
MVP 可先使用 YAML + 热加载；后期可加规则管理后台。

---

## 34. Recommendation Engine 设计

### 34.1 输出对象
- `approval_recommendation`
- `rollout_strategy`
- `observability_plan`
- `rollback_plan`

### 34.2 灰度策略模板
支持模板化生成：
- 小流量渐进放量
- 单可用区先发
- 单租户先发
- canary + 自动观测回滚

### 34.3 指标观察模板
按变更类型预设：
- 鉴权改动：401、登录成功率、认证耗时
- 路由改动：route hit 分布、5xx、目标服务错误率
- SQL 改动：慢查询数、锁等待、DB CPU
- 资源改动：Pod restart、CPU throttling、OOM

### 34.4 回滚计划模板字段
- version_to_restore
- command_refs
- owner_contacts
- verification_metrics
- rollback_deadline_seconds

---

## 35. 控制台页面信息架构

### 35.1 页面布局
1. 顶部：风险等级、摘要、审批状态
2. 左侧：主要风险点、建议动作、回滚摘要
3. 右侧：证据卡片、历史相似事故、工具结果
4. 底部：时间线、人工审批区、审计记录

### 35.2 主要页面
- 审查详情页
- 我的待审批页
- 高风险变更列表
- 规则命中分析页
- 评测指标看板

### 35.3 控制台最小功能
- 查看风险摘要
- 查看证据详情
- approve / reject / override
- 导出审查报告 JSON / Markdown

---

## 36. Webhook 与外部集成设计

### 36.1 GitHub/GitLab webhook 事件
- PR opened
- PR synchronize / updated
- PR labeled
- merge request opened / updated

### 36.2 发布平台 webhook
- release_created
- release_approved
- release_started
- release_completed
- release_rolled_back

### 36.3 通知模板
高风险通知内容应包含：
- review_id
- 风险等级
- 3 个最关键风险点
- 建议审批人
- 审查链接

### 36.4 幂等性要求
- 相同 `source_type + source_id + head_commit` 只创建一个活动审查任务
- webhook 重放需要幂等去重

---

## 37. 缓存与性能设计

### 37.1 可缓存对象
- 相同 diff 的语义解析结果
- 短时间内的 metrics snapshot
- 相同服务的依赖图
- 常用 runbook 检索结果

### 37.2 缓存策略
- diff parse cache：1 天
- metrics cache：1 分钟
- dependency cache：10 分钟
- incident search cache：30 分钟

### 37.3 目标性能指标
- 单次审查 P50 < 4s
- 单次审查 P95 < 10s
- 工具调用失败率 < 2%
- 审查任务成功率 > 99%

---

## 38. 可观测性与告警设计

### 38.1 核心监控项
- review_request_qps
- review_task_latency_ms
- tool_call_error_rate
- llm_call_latency_ms
- recommendation_generation_failures
- human_override_rate

### 38.2 关键告警
- LLM Gateway 故障或 fallback 比例过高
- 工具层连续超时
- 高风险任务积压
- 审查结果生成失败率升高

### 38.3 Trace 字段建议
- trace_id
- review_id
- task_id
- source_type
- service
- environment
- model_name
- tool_name

---

## 39. 安全与权限落地方案

### 39.1 鉴权
- API 使用 JWT / internal service auth
- webhook 使用签名校验
- Console 接入组织 SSO

### 39.2 细粒度授权
- 审批人只能看自己有权限的服务变更
- 部分证据源（安全、生产指标）需额外权限
- 敏感 SQL 内容支持字段级脱敏

### 39.3 Prompt 前脱敏
在进入 LLM 前对以下数据做 mask：
- token / secret / AK/SK
- 用户隐私标识
- 数据库连接串

### 39.4 审计最小要求
- 谁看过审查结果
- 谁做了 override
- 哪些敏感工具被调用过

---

## 40. MVP 开发任务拆分

### 40.1 第一阶段（1-2 周）
- 建 review API
- 建基础表结构
- 接入 Git webhook
- 支持 PR diff 拉取
- 完成 ChangeBundle 标准化

### 40.2 第二阶段（1-2 周）
- 实现变更理解模块
- 接入 3 个工具：Git 历史、runbook 检索、incident 检索
- 实现基础规则引擎
- 输出风险等级和摘要

### 40.3 第三阶段（1-2 周）
- 接入 Recommendation Engine
- 生成灰度与回滚建议
- 做审查详情页
- 接入飞书/Slack 通知

### 40.4 第四阶段（1-2 周）
- 加人工审批与 override
- 加审计日志
- 加基础评测脚本
- 跑历史样本验证

---

## 41. 面试时可强调的工程点

### 41.1 为什么采用 Go + Python 混合架构
Go 负责服务稳定性、吞吐与工程主链；Python 负责 agent workflow、检索与评测生态，兼顾系统性和 AI 研发效率。

### 41.2 为什么用状态机而不是自由 Agent
因为发布审查是高可信、高风险场景，更需要可复现和可审计，而不是开放自治。

### 41.3 为什么规则引擎与 LLM 混合
硬规则保证底线，LLM 提供语义理解与解释能力，两者结合比纯规则或纯模型更稳。

### 41.4 为什么证据链是核心
如果没有证据引用，审查系统就只是一个“会说话的评论器”；只有证据链和人工 override 闭环，系统才具备企业可信性。

---

## 42. 下一步建议

若继续深化到真正可编码阶段，下一轮可以继续补：
1. gRPC / OpenAPI 契约完整定义
2. 主要表的索引设计与迁移脚本
3. Review Orchestrator 的伪代码
4. Agent Workflow Runner 的节点配置文件示例
5. 控制台页面原型
6. 真实简历版项目描述与 STAR 叙事

---

## 43. OpenAPI 详细接口定义（MVP）

本节采用更接近 OpenAPI 的方式定义核心接口，便于后续直接生成 server stub 或 client SDK。

### 43.1 Review 对象 Schema
```json
{
  "type": "object",
  "required": ["review_id", "status", "source_type", "created_at"],
  "properties": {
    "review_id": {"type": "string"},
    "change_id": {"type": "string"},
    "source_type": {"type": "string", "enum": ["pull_request", "sql_migration", "k8s_diff", "terraform_plan", "gateway_config"]},
    "repo": {"type": "string"},
    "service": {"type": "string"},
    "environment": {"type": "string"},
    "status": {"type": "string"},
    "risk_level": {"type": "string", "enum": ["LOW", "MEDIUM", "HIGH", "CRITICAL"]},
    "score": {"type": "integer"},
    "confidence": {"type": "number"},
    "summary": {"type": "string"},
    "human_review_required": {"type": "boolean"},
    "created_at": {"type": "string", "format": "date-time"},
    "updated_at": {"type": "string", "format": "date-time"}
  }
}
```

### 43.2 创建审查任务
`POST /api/v1/reviews`

#### Headers
- `X-Request-Id`
- `X-Webhook-Source`（可选）
- `Authorization` 或 webhook signature

#### Request Body
```json
{
  "source_type": "pull_request",
  "source_id": "pr_12345",
  "repo": "gateway-service",
  "service": "api-gateway",
  "environment": "prod",
  "trigger_mode": "async",
  "dedupe_key": "github:gateway-service:pr_12345:def",
  "payload": {
    "title": "refactor auth middleware and update route rules",
    "author": "alice",
    "base_commit": "abc",
    "head_commit": "def",
    "diff_url": "https://...",
    "metadata": {
      "pr_url": "https://...",
      "labels": ["release-blocking"],
      "target_branch": "main"
    }
  }
}
```

#### Response 202
```json
{
  "review_id": "rvw_001",
  "task_id": "task_001",
  "status": "CREATED",
  "poll_url": "/api/v1/reviews/rvw_001"
}
```

#### Error Cases
- `400`：非法 source_type / payload 缺失
- `401`：签名校验失败
- `409`：重复任务（返回既有 review_id）
- `429`：频率限制

### 43.3 查询审查详情
`GET /api/v1/reviews/{review_id}`

#### Response 200
```json
{
  "review": {
    "review_id": "rvw_001",
    "change_id": "chg_001",
    "source_type": "pull_request",
    "repo": "gateway-service",
    "service": "api-gateway",
    "environment": "prod",
    "status": "WAITING_HUMAN_REVIEW",
    "risk_level": "HIGH",
    "score": 78,
    "confidence": 0.82,
    "summary": "涉及鉴权链路与入口路由规则变更，存在高风险。",
    "human_review_required": true,
    "created_at": "2026-03-17T12:00:00Z",
    "updated_at": "2026-03-17T12:00:08Z"
  },
  "signals": [
    {
      "signal_name": "auth_logic_changed",
      "severity": "HIGH",
      "score_delta": 20,
      "evidence_refs": ["ev_101"]
    }
  ],
  "recommendation": {
    "required_reviewers": ["gateway_owner", "security_reviewer"],
    "release_window": "off_peak",
    "rollout_strategy": {
      "steps": ["5%", "20%", "50%", "100%"],
      "interval_minutes": 10
    },
    "observability_plan": ["401_ratio", "login_success_rate", "gateway_5xx"]
  },
  "rollback_plan": {
    "trigger_condition": "401 ratio > 3x baseline for 5 min",
    "steps": [
      "rollback deployment to previous revision",
      "restore gateway routing config version v102",
      "verify login success rate"
    ]
  },
  "evidence": [
    {
      "evidence_id": "ev_101",
      "type": "incident",
      "source": "incident_db",
      "title": "2025-11 auth routing regression",
      "reference_url": "https://..."
    }
  ]
}
```

### 43.4 查询时间线
`GET /api/v1/reviews/{review_id}/timeline`

#### Response 200
```json
{
  "review_id": "rvw_001",
  "events": [
    {"state": "CREATED", "at": "2026-03-17T12:00:00Z"},
    {"state": "NORMALIZED", "at": "2026-03-17T12:00:01Z"},
    {"state": "UNDERSTOOD", "at": "2026-03-17T12:00:02Z"},
    {"state": "CONTEXT_READY", "at": "2026-03-17T12:00:04Z"},
    {"state": "RECOMMENDED", "at": "2026-03-17T12:00:08Z"}
  ]
}
```

### 43.5 提交人工审批结果
`POST /api/v1/reviews/{review_id}/human-decision`

#### Request Body
```json
{
  "reviewer": "bob",
  "decision": "override",
  "reason": "业务窗口有限，已安排值班和灰度监控",
  "notify": true
}
```

#### Response 200
```json
{
  "review_id": "rvw_001",
  "status": "OVERRIDDEN",
  "recorded": true
}
```

### 43.6 重试任务
`POST /api/v1/reviews/{review_id}/retry`

#### Request Body
```json
{
  "reason": "tool timeout recovered"
}
```

### 43.7 导出审查报告
`GET /api/v1/reviews/{review_id}/export?format=markdown`

支持：
- `markdown`
- `json`

### 43.8 评测统计接口
`GET /api/v1/evaluations/metrics?from=2026-03-01&to=2026-03-31&service=api-gateway`

#### Response 200
```json
{
  "window": {"from": "2026-03-01", "to": "2026-03-31"},
  "service": "api-gateway",
  "metrics": {
    "review_count": 182,
    "high_risk_recall": 0.87,
    "false_positive_rate": 0.11,
    "override_rate": 0.16,
    "p95_latency_ms": 8200
  }
}
```

---

## 44. gRPC 内部契约建议

内部服务间建议使用 gRPC，尤其是 Orchestrator 与各子模块之间。

### 44.1 ReviewOrchestratorService
```proto
service ReviewOrchestratorService {
  rpc StartReview(StartReviewRequest) returns (StartReviewResponse);
  rpc RetryReview(RetryReviewRequest) returns (RetryReviewResponse);
  rpc GetReviewState(GetReviewStateRequest) returns (GetReviewStateResponse);
}
```

### 44.2 ContextRetrievalService
```proto
service ContextRetrievalService {
  rpc CollectContext(CollectContextRequest) returns (CollectContextResponse);
}
```

### 44.3 RuleEngineService
```proto
service RuleEngineService {
  rpc EvaluateSignals(EvaluateSignalsRequest) returns (EvaluateSignalsResponse);
  rpc ScoreReview(ScoreReviewRequest) returns (ScoreReviewResponse);
}
```

### 44.4 RecommendationService
```proto
service RecommendationService {
  rpc GenerateRecommendation(GenerateRecommendationRequest) returns (GenerateRecommendationResponse);
}
```

---

## 45. 数据库索引设计

为了保证查询性能与时间线、详情页、评测分析的可用性，建议增加以下索引。

### 45.1 reviews 索引
```sql
CREATE UNIQUE INDEX idx_reviews_review_id ON reviews(review_id);
CREATE INDEX idx_reviews_status_created_at ON reviews(status, created_at DESC);
CREATE INDEX idx_reviews_service_env_created_at ON reviews(service, environment, created_at DESC);
CREATE INDEX idx_reviews_risk_level_created_at ON reviews(risk_level, created_at DESC);
```

### 45.2 review_tasks 索引
```sql
CREATE UNIQUE INDEX idx_review_tasks_task_id ON review_tasks(task_id);
CREATE INDEX idx_review_tasks_review_id ON review_tasks(review_id);
CREATE INDEX idx_review_tasks_state_updated_at ON review_tasks(current_state, updated_at DESC);
```

### 45.3 change_snapshots 索引
```sql
CREATE INDEX idx_change_snapshots_change_id ON change_snapshots(change_id);
CREATE INDEX idx_change_snapshots_tags_gin ON change_snapshots USING GIN (semantic_tags);
CREATE INDEX idx_change_snapshots_file_list_gin ON change_snapshots USING GIN (file_list);
```

### 45.4 evidence_items 索引
```sql
CREATE UNIQUE INDEX idx_evidence_items_evidence_id ON evidence_items(evidence_id);
CREATE INDEX idx_evidence_items_review_id ON evidence_items(review_id);
CREATE INDEX idx_evidence_items_type_source ON evidence_items(evidence_type, source);
CREATE INDEX idx_evidence_items_metadata_gin ON evidence_items USING GIN (metadata);
```

### 45.5 risk_signals 索引
```sql
CREATE UNIQUE INDEX idx_risk_signals_signal_id ON risk_signals(signal_id);
CREATE INDEX idx_risk_signals_review_id ON risk_signals(review_id);
CREATE INDEX idx_risk_signals_name_severity ON risk_signals(signal_name, severity);
```

### 45.6 recommendations / human_decisions / audit_events 索引
```sql
CREATE INDEX idx_recommendations_review_id ON recommendations(review_id);
CREATE INDEX idx_human_decisions_review_id_created_at ON human_decisions(review_id, created_at DESC);
CREATE INDEX idx_audit_events_review_id_created_at ON audit_events(review_id, created_at DESC);
CREATE INDEX idx_audit_events_trace_id ON audit_events(trace_id);
```

### 45.7 evaluation_records 索引
```sql
CREATE INDEX idx_evaluation_records_review_id ON evaluation_records(review_id);
CREATE INDEX idx_evaluation_records_incident_flag_created_at ON evaluation_records(incident_flag, created_at DESC);
```

---

## 46. 迁移脚本组织建议

建议使用 `golang-migrate` 或 `atlas` 管理 schema migration。

### 46.1 目录结构
```text
migrations/
  000001_create_reviews.up.sql
  000001_create_reviews.down.sql
  000002_create_review_tasks.up.sql
  000002_create_review_tasks.down.sql
  ...
```

### 46.2 首批 migration 顺序
1. create_reviews
2. create_review_tasks
3. create_change_snapshots
4. create_evidence_items
5. create_risk_signals
6. create_recommendations
7. create_human_decisions
8. create_audit_events
9. create_evaluation_records
10. add_indexes

### 46.3 演进建议
- MVP 阶段尽量保守，不要过度范式化
- 高频查询的 JSON 字段后续再拆列
- 对 recommendation / rollback_plan 保留 JSONB 灵活性

---

## 47. Review Orchestrator 伪代码

### 47.1 主流程伪代码（Go）
```go
func (s *ReviewOrchestrator) StartReview(ctx context.Context, req StartReviewRequest) error {
    reviewID := req.ReviewID

    if err := s.taskRepo.UpdateState(ctx, reviewID, "NORMALIZED"); err != nil {
        return err
    }

    bundle, err := s.ingestion.Normalize(ctx, req.ChangeSource)
    if err != nil {
        return s.failReview(ctx, reviewID, "normalize_failed", err)
    }

    understanding, err := s.understanding.Analyze(ctx, bundle)
    if err != nil {
        return s.failReview(ctx, reviewID, "understand_failed", err)
    }
    _ = s.taskRepo.UpdateState(ctx, reviewID, "UNDERSTOOD")

    ctxPack, err := s.retrieval.CollectContext(ctx, CollectContextInput{
        ReviewID: reviewID,
        Bundle:   bundle,
        Tags:     understanding.Tags,
    })
    if err != nil {
        return s.failReview(ctx, reviewID, "context_failed", err)
    }
    _ = s.taskRepo.UpdateState(ctx, reviewID, "CONTEXT_READY")

    signals, baseScore, err := s.ruleEngine.Evaluate(ctx, bundle, understanding, ctxPack)
    if err != nil {
        return s.failReview(ctx, reviewID, "signal_failed", err)
    }
    _ = s.taskRepo.UpdateState(ctx, reviewID, "SIGNALS_EXTRACTED")

    agentOutput, err := s.agentRunner.Run(ctx, AgentInput{
        Bundle:        bundle,
        Understanding: understanding,
        Context:       ctxPack,
        Signals:       signals,
        BaseScore:     baseScore,
    })
    if err != nil {
        return s.failReview(ctx, reviewID, "agent_failed", err)
    }

    scoreResult, err := s.ruleEngine.Score(ctx, ScoreInput{
        BaseScore:   baseScore,
        Signals:     signals,
        Confidence:  agentOutput.Confidence,
        Environment: bundle.Environment,
    })
    if err != nil {
        return s.failReview(ctx, reviewID, "score_failed", err)
    }
    _ = s.taskRepo.UpdateState(ctx, reviewID, "SCORED")

    recommendation, err := s.recommendation.Generate(ctx, RecommendationInput{
        Bundle:      bundle,
        Signals:     signals,
        Score:       scoreResult,
        AgentOutput: agentOutput,
        Evidence:    ctxPack.Evidence,
    })
    if err != nil {
        return s.failReview(ctx, reviewID, "recommendation_failed", err)
    }
    _ = s.taskRepo.UpdateState(ctx, reviewID, "RECOMMENDED")

    requireHuman := scoreResult.RiskLevel == "HIGH" || scoreResult.RiskLevel == "CRITICAL" || agentOutput.Confidence < 0.75

    if requireHuman {
        _ = s.taskRepo.UpdateState(ctx, reviewID, "WAITING_HUMAN_REVIEW")
        _ = s.notification.NotifyReviewers(ctx, reviewID, recommendation.RequiredReviewers)
    } else {
        _ = s.taskRepo.UpdateState(ctx, reviewID, "APPROVED")
    }

    return s.audit.RecordCompletion(ctx, reviewID)
}
```

### 47.2 失败处理伪代码
```go
func (s *ReviewOrchestrator) failReview(ctx context.Context, reviewID, reason string, err error) error {
    _ = s.taskRepo.MarkFailed(ctx, reviewID, reason, err.Error())
    _ = s.audit.RecordFailure(ctx, reviewID, reason, err)
    return err
}
```

### 47.3 幂等控制建议
- `dedupe_key` 写入 Redis / DB
- 同一 `source_id + head_commit` 不重复创建任务
- 重试时沿用原 `review_id`

---

## 48. Agent Workflow Runner 配置示例

建议把 workflow 节点配置化，便于后续扩展不同变更类型。

### 48.1 YAML 配置示例
```yaml
workflow_name: change_review_pr
version: v1
nodes:
  - id: classify_change
    type: llm_structured
    input:
      from: change_bundle
    output_schema: ChangeClassification

  - id: collect_context
    type: tool_batch
    depends_on: [classify_change]
    tools:
      - git_history_search
      - incident_search
      - runbook_search
      - service_dependency_lookup

  - id: evaluate_signals
    type: rule_engine
    depends_on: [collect_context]

  - id: summarize_risk
    type: llm_structured
    depends_on: [evaluate_signals]
    prompt_template: risk_summary_v2
    constraints:
      require_evidence_refs: true
      allow_unknown: true

  - id: generate_recommendation
    type: template_merge
    depends_on: [summarize_risk]

  - id: gate_human_review
    type: decision
    depends_on: [generate_recommendation]
    conditions:
      - field: risk_level
        op: in
        value: [HIGH, CRITICAL]
      - field: confidence
        op: lt
        value: 0.75
```

### 48.2 Python Runner 伪代码
```python
class WorkflowRunner:
    def run(self, workflow, context):
        node_results = {}
        for node in workflow.nodes:
            if not self._deps_satisfied(node, node_results):
                continue
            result = self._execute_node(node, context, node_results)
            node_results[node.id] = result
        return self._assemble_output(node_results)
```

---

## 49. Prompt / Structured Output 设计示例

### 49.1 风险总结 Prompt 约束
系统提示核心约束：
- 你是发布前变更审查助手，不是代码生成助手
- 只允许基于提供的 evidence 作出确定性结论
- 没有证据支持的内容必须放入 `hypotheses`
- 输出必须符合 JSON schema
- 必须给每条关键风险绑定 evidence_refs

### 49.2 输出 JSON Schema
```json
{
  "type": "object",
  "required": ["summary", "top_risks", "confidence", "human_review_required"],
  "properties": {
    "summary": {"type": "string"},
    "top_risks": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["risk", "evidence_refs"],
        "properties": {
          "risk": {"type": "string"},
          "evidence_refs": {"type": "array", "items": {"type": "string"}}
        }
      }
    },
    "hypotheses": {
      "type": "array",
      "items": {"type": "string"}
    },
    "confidence": {"type": "number"},
    "human_review_required": {"type": "boolean"}
  }
}
```

---

## 50. Recommendation Engine 伪代码

```go
func (r *RecommendationEngine) Generate(ctx context.Context, in RecommendationInput) (RecommendationOutput, error) {
    output := RecommendationOutput{}

    output.RequiredReviewers = r.policyResolver.ResolveReviewers(in.Bundle, in.Signals, in.Score)
    output.ReleaseWindow = r.policyResolver.ResolveReleaseWindow(in.Bundle, in.Score)
    output.RolloutStrategy = r.rolloutPlanner.Plan(in.Bundle, in.Score, in.Signals)
    output.ObservabilityPlan = r.metricsPlanner.Plan(in.Bundle.ChangeType, in.Signals)
    output.RollbackPlan = r.rollbackPlanner.Generate(in.Bundle, in.Evidence, in.Signals)

    return output, nil
}
```

---

## 51. 控制台页面原型草案

### 51.1 审查详情页模块
```text
--------------------------------------------------
Review #rvw_001     HIGH RISK      Waiting Approval
--------------------------------------------------
Summary:
涉及鉴权链路与入口路由规则变更，存在高风险。

Top Risks:
1. auth_logic_changed
2. traffic_routing_changed
3. missing_verified_rollback

Recommendation:
- Required reviewers: gateway_owner, security_reviewer
- Release window: off_peak
- Rollout: 5% -> 20% -> 50% -> 100%

Rollback Plan:
- rollback deployment to previous revision
- restore route config v102
- verify login success rate

Evidence:
[Incident] 2025-11 auth routing regression
[Runbook] Gateway rollback SOP
[Metrics] current gateway error baseline

Timeline:
CREATED -> NORMALIZED -> UNDERSTOOD -> ...

[Approve] [Reject] [Override]
--------------------------------------------------
```

### 51.2 高风险列表页字段
- review_id
- service
- source_type
- risk_level
- summary
- created_at
- assigned_reviewer
- status

---

## 52. 测试设计

### 52.1 单元测试
- 规则引擎命中
- recommendation 模板生成
- rollback planner 输出完整性
- webhook 签名校验
- 幂等逻辑

### 52.2 集成测试
- PR webhook -> review created -> recommendation generated
- 高风险 -> 通知 -> 人工 override -> 审计落库
- 工具超时 -> fallback -> low-confidence -> 人工升级

### 52.3 回归测试
建立一批固定样本：
- 正常变更
- 高风险鉴权变更
- SQL 锁表风险变更
- K8s 缩容风险变更

### 52.4 LLM 评测
- 证据引用完整率
- unsupported claim 比例
- 风险摘要一致性
- recommendation 可执行性

---

## 53. MVP 仓库结构建议

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

---

## 54. 实现优先级建议

### 54.1 第一优先级（必须先做）
- Review API
- review/review_tasks/change_snapshots 表
- Git PR 接入
- 变更理解
- 规则引擎最小集
- recommendation 输出
- 审查详情页只读版

### 54.2 第二优先级（体现 Agent 价值）
- 工具调用层
- evidence 引用
- incident / runbook 检索
- human review
- 通知能力

### 54.3 第三优先级（体现平台和闭环）
- 评测看板
- override 分析
- 导出报告
- 缓存和 trace 细化

---

## 55. 进一步深化建议

如果继续往下做，下一版最值得补的是：
1. 真正的 OpenAPI YAML 文件
2. proto 文件完整定义
3. 关键 Go 接口与 domain model
4. `review-api` / `orchestrator` / `worker` 的目录级代码骨架
5. 规则配置文件与 Prompt 模板样例
6. 前端页面线框图

这一步做完后，这个项目就已经非常接近“可以开始按模块编码”的程度了。

