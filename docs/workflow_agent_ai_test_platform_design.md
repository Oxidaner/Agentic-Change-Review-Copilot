# 基于 Workflow Agent 的 AI 自动化测试平台设计文档

## 1. 项目定位

### 1.1 项目名称
基于 Workflow Agent 的 AI 自动化测试平台

英文描述可使用：
Workflow-Agent-Based AI Test Automation Platform

### 1.2 一句话定义
这是一个面向真实研发流程的 AI 自动化测试运行时。它以受控 workflow 为核心，把变更输入、测试点抽取、用例生成、工具执行、智能断言、失败归因和测试报告串成一条稳定、可扩展、可审计的测试自动化主线。

### 1.3 这不是一个什么项目
- 不是单点 prompt demo。
- 不是简单“调模型 API”的测试助手。
- 不是自由规划的通用 Agent。
- 不是大而空的平台叙事。

### 1.4 这是什么项目
- 是一个 workflow-first 的 AI 测试运行时。
- 是一个把 AI 能力嵌入真实工程测试流程的工程化系统。
- 是一个可持续扩展工具接入、断言策略、归因能力和报告能力的平台雏形。

---

## 2. 项目边界

### 2.1 只做一条主线
项目最终边界收敛为一条清晰主线：

`PR / 需求变更 -> 测试点抽取 -> 用例生成 -> 工具执行 -> 智能断言 -> 失败归因 -> 测试报告`

### 2.2 为什么这样定边界
- 和 AI 自动化测试岗位的职责最直接匹配。
- 依然可以讲清楚工程架构、平台能力和 workflow runtime。
- 不会落入“通用大平台”但做不出来的陷阱。
- 能做出真实可演示的 MVP。

### 2.3 非目标
- 不做全链路测试平台替代品。
- 不在第一版覆盖所有测试类型。
- 不做通用 AI QA 问答机器人。
- 不做自由自治式多 Agent 系统。

---

## 3. 核心设计原则

### 3.1 受控 workflow 优先
测试自动化场景需要稳定、可追踪、可复现。系统应以固定步骤、状态机和 DAG 为主，而不是自由 Agent。

### 3.2 AI 是能力层，不是系统边界
模型调用只是能力来源之一。系统真正的工程核心是：
- workflow 编排
- 工具接入
- 执行控制
- 结果结构化
- 审计与评测

### 3.3 工具执行必须是一等公民
测试平台不是只“生成文字”。它必须把测试工具接进主流程，例如：
- API 测试执行器
- UI 自动化执行器
- mock / sandbox 环境
- 日志与 trace 查询工具
- 覆盖率 / 测试结果采集工具

### 3.4 失败归因和闭环优先
测试平台的价值不只是“跑起来”，还包括：
- 为什么失败
- 失败属于产品缺陷、环境问题还是脚本问题
- 如何沉淀为后续优化输入

### 3.5 面向真实工程流程
系统嵌入：
- 开发流程
- 测试流程
- 提测 / 回归流程
- 变更后质量保障流程

---

## 4. 三层架构

### 4.1 Runtime / Orchestration 层

这一层负责“测试 workflow 怎么跑”。

职责：
- 定义 workflow step
- 管理状态流转
- 控制条件分支
- 调度工具执行
- 管理重试、超时和失败恢复
- 记录 trace / audit
- 支持人工接管

建议核心抽象：
- `Workflow`
- `Step`
- `State`
- `Tool`
- `ToolResult`
- `ModelAdapter`
- `TestTask`
- `TraceEvent`

建议状态流：
- `INIT`
- `PARSE_CHANGE`
- `EXTRACT_TEST_POINTS`
- `GENERATE_TEST_CASES`
- `PREPARE_ENV`
- `EXECUTE_TOOLS`
- `SMART_ASSERT`
- `ROOT_CAUSE_ANALYZE`
- `GENERATE_REPORT`
- `HUMAN_REVIEW_REQUIRED`
- `DONE`
- `FAILED`

### 4.2 AI Capability 层

这一层负责“智能能力从哪里来”。

职责：
- 变更理解
- 测试点抽取
- 用例生成
- 测试数据建议
- 智能断言
- 失败归因
- 报告摘要

建议模块：
- `change_parser`
- `test_point_extractor`
- `test_case_generator`
- `test_data_planner`
- `assertion_engine`
- `failure_analyzer`
- `report_generator`

### 4.3 Scenario Layer

这一层负责“具体落在哪类测试场景上”。

一期只打透一个主场景：
- PR / 需求变更驱动的自动化测试

一期优先支持：
- API 回归测试
- Web UI 核心路径测试

后续可扩展：
- 接口契约测试
- 数据校验测试
- 端到端业务流程测试
- 移动端自动化测试

---

## 5. MVP 范围

### 5.1 一期目标
一期目标不是做一个全能测试平台，而是做一个：

能围绕变更生成并执行自动化测试 workflow，输出结构化测试结果和失败归因的 AI 系统。

### 5.2 一期必须打通的链路
- 输入 PR 或需求变更
- 抽取测试点
- 生成结构化测试用例
- 选择并调用测试工具
- 执行测试
- 对结果进行智能断言
- 对失败结果做归因
- 输出测试报告

### 5.3 一期交付形态
优先交付：
- 后端 API
- CLI / JSON / Markdown report demo

不优先交付：
- 大型测试管理前端
- 多租户平台能力
- 十几类测试工具全接入

---

## 6. 端到端主流程

### 6.1 主流程
1. 接收 PR diff 或需求变更输入。
2. 解析改动内容和上下文。
3. 抽取测试点。
4. 生成结构化测试用例。
5. 选择执行工具与执行参数。
6. 执行测试工具。
7. 收集日志、结果、trace、截图等执行证据。
8. 对执行结果进行智能断言。
9. 对失败样本做归因分析。
10. 输出测试报告。

### 6.2 失败处理流程
当测试失败时，系统需要判断：
- 是产品逻辑缺陷
- 是环境异常
- 是数据污染
- 是脚本本身不稳定
- 是断言策略不合理

### 6.3 人工介入场景
- 结果冲突
- 归因置信度不足
- 高价值链路失败
- 工具执行结果不完整
- 测试成本过高需要中断

---

## 7. 关键模块

### 7.1 Change Input Layer
负责接收：
- PR diff
- 需求变更描述
- 接口变更说明
- 测试范围提示

并统一抽象成输入对象。

### 7.2 Test Point Extractor
负责从变更中提取：
- 受影响功能点
- 关键路径
- 风险点
- 建议测试维度

### 7.3 Test Case Generator
负责生成结构化测试用例，例如：
- 前置条件
- 测试步骤
- 输入数据
- 预期结果
- 推荐执行工具

### 7.4 Tool Execution Layer
负责测试工具调用。

一期建议工具类型：
- `api_test_runner`
- `ui_test_runner`
- `mock_data_loader`
- `log_query`
- `trace_query`
- `artifact_collector`

### 7.5 Smart Assertion Layer
负责把执行结果和预期结合起来判断：
- 是否通过
- 是否部分通过
- 是否存在弱失败
- 是否需要人工确认

### 7.6 Failure Analyzer
负责失败归因，输出：
- failure_type
- probable_root_cause
- evidence_refs
- confidence
- next_action

### 7.7 Report Generator
负责输出：
- 测试概览
- 通过 / 失败统计
- 失败归因摘要
- 关键证据
- 建议动作

---

## 8. 数据模型建议

建议核心对象：
- `ChangeInput`
- `TestPoint`
- `TestCase`
- `ExecutionPlan`
- `ExecutionResult`
- `AssertionResult`
- `FailureAnalysis`
- `TestReport`
- `TraceEvent`

对象作用：
- `ChangeInput`：统一变更输入
- `TestPoint`：描述需要覆盖的测试点
- `TestCase`：结构化测试用例
- `ExecutionPlan`：描述工具如何执行
- `ExecutionResult`：保存执行结果和产物
- `AssertionResult`：保存断言判断
- `FailureAnalysis`：保存失败归因
- `TestReport`：保存最终测试报告
- `TraceEvent`：保存 workflow 过程轨迹

---

## 9. 当前仓库与目标架构的关系

### 9.1 当前仓库已有的可复用底座
虽然代码当前还是 review 语义，但底层有一些部分可以复用到测试平台：
- HTTP API 骨架
- workflow / 状态流思路
- PostgreSQL 持久化
- 审计与评测反馈思路
- OpenAPI 契约
- Docker 本地开发方式
- 基础测试

### 9.2 当前仓库还没有切换的部分
还没有真正切成测试平台语义的部分包括：
- review -> test task 的领域模型迁移
- 风险信号 -> 测试点 / 断言 / 失败归因模型迁移
- review API -> test workflow API 的契约迁移
- 真实测试工具接入

### 9.3 这意味着什么
意味着当前仓库不是完全推翻重来，而是：
- 保留 workflow runtime 雏形
- 重写场景层叙事
- 渐进替换领域模型和 API 语义

---

## 10. 当前最优演进路径

### 10.1 第一优先级
- 把文档、OpenAPI、命名、领域模型从 review 语义切到 test platform 语义。
- 明确一期只做 API + CLI demo。

### 10.2 第二优先级
- 抽出 runtime 抽象：`Workflow`、`Step`、`Tool`、`ModelAdapter`。
- 把当前 service 中混在一起的逻辑拆成更清晰的模块边界。

### 10.3 第三优先级
- 先打透 API 回归测试这一个最容易落地的测试场景。
- 再增加 UI 核心路径测试。

### 10.4 第四优先级
- 增加 PostgreSQL 集成测试。
- 增加契约校验。
- 增加工具执行结果的报告落盘和 artifact 收集。

---

## 11. 面试叙事建议

### 11.1 对测试平台 / 工程效能岗位
重点讲：
- PR / 需求变更驱动测试自动化
- 测试点抽取和用例生成
- 工具执行和智能断言
- 失败归因和报告闭环

### 11.2 对后端 / 平台岗位
重点讲：
- workflow runtime
- 状态机与任务编排
- 工具抽象
- 持久化设计
- 审计和可观测性

### 11.3 对 AI 应用 / Agent 岗位
重点讲：
- 受控 workflow 优于自由 agent
- 结构化输出
- 工具调用和反馈闭环
- AI 如何嵌入真实测试流程

---

## 12. 结论

项目最终版本应定为：

`基于 Workflow Agent 的 AI 自动化测试平台`

清晰主线固定为：

`PR / 需求变更 -> 测试点抽取 -> 用例生成 -> 工具执行 -> 智能断言 -> 失败归因 -> 测试报告`

这条主线足够具体、足够工程化，也足够适合做出一个真实可落地的 AI 测试平台 MVP。
