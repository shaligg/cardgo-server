# 工坊系统设计文档

## 1. 文档目标

本文档定义卡牌休闲游戏中的工坊系统。

工坊系统承担局外长期成长、资源回收、目标承接和玩家表达。

本文档解决以下问题：

- 工坊由哪些设施组成。
- 每个设施提供什么功能。
- 设施如何解锁和升级。
- 设施升级消耗如何接入背包与资产系统。
- 离线收益如何计算。
- 装饰系统做到什么边界。
- 工坊界面展示哪些信息。
- 后端需要保存哪些数据。

本文档不展开活动工坊、好友参观、装饰排行榜等高级内容。

这些内容可以在后续活动文档和社交文档中补充。

## 2. 系统定位

工坊是局外主界面和长期成长系统。

玩家通过完成订单获得金币、材料、声望和卡牌资源，再消耗这些资源升级工坊设施。

工坊设施反过来提升订单收益、材料产出、离线收益和卡牌养成效率。

核心循环：

```text
进入工坊
  -> 查看订单与设施状态
  -> 进入关卡完成订单
  -> 获得金币、材料、声望、卡牌资源
  -> 升级设施或布置装饰
  -> 提升后续订单效率
  -> 解锁新章节、新订单和新设施
```

工坊不是战斗系统。

工坊不是单纯数值战力系统。

工坊要同时提供：

- 明确的成长目标。
- 可感知的效率提升。
- 轻度的空间装饰表达。
- 对卡牌、订单、背包和活动的承接。

## 3. 设计原则

### 3.1 工坊是长期目标，不是强卡点

工坊升级应该让玩家感觉后续体验更顺，但不应该让玩家因为某个设施没升级而完全不能玩。

错误做法：

- 第 10 关必须烤炉 5 级，否则无法进入。
- 某类订单必须指定设施等级才允许完成。
- 装饰加成直接决定能否通关。

推荐做法：

- 烤炉等级提高面包订单金币收益。
- 订单板等级提高高品质订单出现概率。
- 研究台等级解锁更多卡牌养成功能。
- 仓库等级提高材料上限，减少溢出。

### 3.2 升级价值必须可见

每次设施升级都要让玩家知道获得了什么。

升级表现至少包含：

- 设施外观变化。
- 数值收益变化。
- 解锁内容提示。
- 后续目标提示。

示例：

```text
烤炉 Lv.2 -> Lv.3
面包类订单金币收益 +4%
解锁订单：奶油可颂
下一级：面包类订单金币收益 +5%
```

### 3.3 装饰加成轻量化

装饰主要承担收集和表达。

装饰可以有轻微加成，但不能成为主数值系统。

建议控制：

- 单件装饰加成很小。
- 同类加成有上限。
- 装饰不影响核心关卡准入。
- 稀有装饰更多体现外观和收藏。

### 3.4 局内资源和工坊资源分离

局内资源只在单局中存在。

例如行动点、临时食材、连击数、临时金币等，不进入工坊背包。

工坊只接收单局结算后的最终奖励。

### 3.5 数据以服务器为准

工坊设施等级、装饰拥有、装饰摆放、离线收益结算都以服务器数据为准。

客户端可以展示倒计时和预估收益，但最终领取结果必须由服务器计算。

## 4. 工坊模块结构

工坊系统由以下子模块组成：

| 模块 | 说明 | MVP 是否需要 |
| --- | --- | --- |
| 工坊总览 | 主界面状态、设施入口、收益提示 | 需要 |
| 设施升级 | 核心长期成长 | 需要 |
| 离线收益 | 低压力回流奖励 | 需要 |
| 装饰仓库 | 装饰拥有与展示 | 可简化 |
| 装饰摆放 | 空间表达 | 可简化 |
| 主题皮肤 | 一键切换工坊风格 | 后续 |
| 猫咪派遣 | 派遣猫咪获得材料 | 后续 |
| 好友参观 | 查看好友工坊、点赞 | 后续 |
| 人气榜 | 装饰排行榜 | 后续 |

MVP 阶段建议只完成：

- 工坊总览。
- 6 个基础设施。
- 设施升级。
- 简版离线收益。
- 装饰拥有和一键展示。

## 5. 基础设施列表

MVP 使用 6 个基础设施。

| facility_id | 名称 | 定位 | 核心作用 |
| --- | --- | --- | --- |
| `oven` | 烤炉 | 订单收益 | 提升面包类订单收益 |
| `warehouse` | 仓库 | 资源容量 | 提升材料存储上限 |
| `order_board` | 订单板 | 订单质量 | 提升订单刷新数量和品质 |
| `showcase` | 展示柜 | 离线收益 | 提升离线金币收益 |
| `research_table` | 研究台 | 卡牌养成 | 解锁卡牌升级、合成和高级效果 |
| `rest_area` | 休息区 | 体力恢复 | 提升猫咪体力或局外恢复效率 |

设施设计目标：

- 每个设施都要对应一个清晰的系统收益。
- 不同设施的升级价值不能完全重叠。
- 前期 6 个设施足够支撑 Demo 和第一章。

## 6. 设施详细设计

### 6.1 烤炉 oven

定位：

提升面包类订单收益。

影响内容：

- 面包类订单金币结算。
- 面包类订单材料掉落。
- 部分面包主题关卡的额外奖励。

基础规则：

| 等级 | 效果 |
| --- | --- |
| Lv.1 | 面包类订单金币收益 +0% |
| Lv.2 | 面包类订单金币收益 +3% |
| Lv.3 | 面包类订单金币收益 +7% |
| Lv.4 | 面包类订单金币收益 +12% |
| Lv.5 | 面包类订单金币收益 +18% |

解锁建议：

- 第 1 局后默认解锁。
- 作为第一个升级教学设施。

升级消耗倾向：

- 金币。
- 木材。
- 面粉。

设计注意：

- 烤炉只影响面包类订单，不影响所有订单。
- 这样可以让主题设施有差异，而不是所有设施都做全局增益。

### 6.2 仓库 warehouse

定位：

提升材料存储上限。

影响内容：

- 普通材料上限。
- 活动材料上限可后续扩展。
- 溢出材料处理。

基础规则：

| 等级 | 基础材料上限 | 高级材料上限 |
| --- | --- | --- |
| Lv.1 | 999 | 99 |
| Lv.2 | 1500 | 150 |
| Lv.3 | 2200 | 220 |
| Lv.4 | 3000 | 300 |
| Lv.5 | 4000 | 400 |

解锁建议：

- 第 3 局后解锁。
- 当玩家第一次获得多种材料后出现。

升级消耗倾向：

- 金币。
- 木材。
- 铁钉。

溢出规则：

- MVP 阶段允许材料临时超过上限，但不能继续通过常规玩法获得同类材料。
- 邮件、付费、活动补偿类奖励可以突破上限。
- 超上限时客户端提示“仓库已满，升级仓库可继续获得材料”。

设计注意：

- 仓库不能过早变成强卡点。
- 前期上限应足够宽松，主要用于教育玩家有长期建设目标。

### 6.3 订单板 order_board

定位：

提升订单数量、刷新质量和特殊订单出现概率。

影响内容：

- 每日订单槽位。
- 高品质订单概率。
- 订单刷新次数。
- 特殊订单解锁。

基础规则：

| 等级 | 每日订单槽位 | 免费刷新次数 | 高品质订单权重 |
| --- | --- | --- | --- |
| Lv.1 | 3 | 1 | 100 |
| Lv.2 | 4 | 1 | 115 |
| Lv.3 | 4 | 2 | 130 |
| Lv.4 | 5 | 2 | 150 |
| Lv.5 | 5 | 3 | 175 |

解锁建议：

- 第 4 局后解锁。
- 与订单系统教学绑定。

升级消耗倾向：

- 金币。
- 纸张。
- 墨水。

设计注意：

- 订单板提升的是选择质量，不是直接提升通关能力。
- 每日订单槽位不要过多，避免休闲玩家压力过大。

### 6.4 展示柜 showcase

定位：

提升离线收益。

影响内容：

- 离线金币产出。
- 离线材料产出。
- 离线收益上限时长。

基础规则：

| 等级 | 离线金币效率 | 离线材料效率 | 最大累计时长 |
| --- | --- | --- | --- |
| Lv.1 | 100% | 100% | 4 小时 |
| Lv.2 | 110% | 105% | 5 小时 |
| Lv.3 | 120% | 110% | 6 小时 |
| Lv.4 | 135% | 115% | 8 小时 |
| Lv.5 | 150% | 120% | 10 小时 |

解锁建议：

- 玩家首次离线回归后解锁。
- 或第 5 局后解锁。

升级消耗倾向：

- 金币。
- 玻璃。
- 木材。

设计注意：

- 离线收益应该鼓励回流，不应该超过主动游玩收益。
- 建议每日离线总收益控制在活跃玩家日收益的 10% 到 25%。

### 6.5 研究台 research_table

定位：

解锁卡牌养成能力。

影响内容：

- 卡牌升级等级上限。
- 卡牌碎片合成。
- 高级卡牌效果解锁。
- 卡组方案数量。

基础规则：

| 等级 | 功能 |
| --- | --- |
| Lv.1 | 允许卡牌升到 Lv.2 |
| Lv.2 | 解锁卡牌碎片合成 |
| Lv.3 | 允许卡牌升到 Lv.3 |
| Lv.4 | 解锁第二套卡组方案 |
| Lv.5 | 解锁高级卡牌特性 |

解锁建议：

- 玩家获得第一张重复卡后解锁。
- 或第 6 局后解锁。

升级消耗倾向：

- 金币。
- 星尘。
- 图纸。

设计注意：

- 研究台和卡牌系统强关联，必须避免过早暴露复杂合成。
- MVP 阶段可以只做到 Lv.3。

### 6.6 休息区 rest_area

定位：

提升猫咪体力恢复或局外恢复效率。

MVP 如果暂不做猫咪体力，可以把休息区定义为“每日福利设施”。

影响内容：

- 每日免费体力。
- 猫咪恢复速度。
- 每日小礼包。
- 连续登录轻奖励。

基础规则：

| 等级 | 每日领取 | 额外效果 |
| --- | --- | --- |
| Lv.1 | 金币 50 | 无 |
| Lv.2 | 金币 80 | 小概率材料 |
| Lv.3 | 金币 120 | 每日体力 +1 |
| Lv.4 | 金币 160 | 小概率卡牌碎片 |
| Lv.5 | 金币 220 | 每日体力 +2 |

解锁建议：

- 第 2 天登录时解锁。
- 或第 8 局后解锁。

升级消耗倾向：

- 金币。
- 布料。
- 木材。

设计注意：

- 如果 MVP 没有体力系统，休息区先作为每日领取设施。
- 后续加入猫咪派遣时，休息区再接入恢复效率。

## 7. 设施解锁节奏

设施不能一开始全部开放。

推荐解锁节奏：

| 时机 | 解锁内容 | 目的 |
| --- | --- | --- |
| 第 1 局后 | 烤炉 | 建立“完成订单 -> 升级设施” |
| 第 3 局后 | 仓库 | 引入材料上限 |
| 第 4 局后 | 订单板 | 引入订单选择 |
| 第 5 局后 | 展示柜 | 引入离线收益 |
| 第 6 局后 | 研究台 | 引入卡牌养成 |
| 第 2 天或第 8 局后 | 休息区 | 引入每日领取 |

解锁条件字段：

```text
UnlockCondition
- min_player_level
- min_chapter_id
- min_level_id
- min_order_count
- min_login_day
- required_facility_id
- required_facility_level
```

MVP 建议使用简单条件：

- 通过指定关卡。
- 玩家等级达到指定等级。

暂不需要复杂的多条件组合。

## 8. 设施升级规则

### 8.1 升级入口

玩家在工坊主界面点击设施。

客户端展示：

- 当前等级。
- 当前效果。
- 下一级效果。
- 升级消耗。
- 是否满足条件。
- 升级后外观预览。

玩家确认后发起升级请求。

服务器校验资产、条件和配置。

升级成功后返回新设施等级和资产变化。

### 8.2 升级限制

设施升级限制包括：

- 玩家等级。
- 章节进度。
- 前置设施等级。
- 资源消耗。
- 最大等级。

MVP 阶段建议只保留：

- 资源消耗。
- 玩家等级。
- 最大等级。

### 8.3 升级消耗

设施升级消耗使用背包与资产系统中的 `CostItem`。

示例：

```text
UpgradeCost
- facility_id: oven
- from_level: 2
- to_level: 3
- costs:
  - type: asset
    id: gold
    count: 300
  - type: item
    id: flour
    count: 20
  - type: item
    id: wood
    count: 10
```

### 8.4 升级收益

设施收益统一配置为效果列表。

```text
FacilityEffect
- effect_type
- target_type
- target_id
- value_type
- value
```

示例：

```text
effect_type: order_reward_bonus
target_type: order_tag
target_id: bread
value_type: percent
value: 7
```

设计理由：

- 以后新增设施效果时不改玩家数据结构。
- 订单、卡牌、离线收益都可以读取统一效果。

### 8.5 升级流程

```text
客户端点击升级
  -> workshop.upgrade_facility 请求
  -> WorkshopService 校验设施配置
  -> WorkshopService 校验当前等级
  -> WorkshopService 校验解锁条件
  -> AssetService 扣除 CostItem
  -> WorkshopRepository 更新设施等级
  -> AssetLog 写入资产流水
  -> 返回设施新等级、资产变化、效果变化
```

注意：

- 扣资源和更新设施等级必须在同一事务中。
- 请求必须带 `req_id`，防止重复扣费。
- 返回内容要足够客户端刷新局部 UI。

## 9. 设施效果接入

### 9.1 订单收益接入

订单结算时，OrderService 查询当前玩家工坊效果。

```text
订单基础奖励
  -> 卡牌与连击修正
  -> 工坊设施修正
  -> 装饰轻量修正
  -> 活动修正
  -> 最终奖励
```

设施效果示例：

- 烤炉提升面包类订单金币。
- 订单板提升高品质订单概率。
- 展示柜影响离线订单收益。

### 9.2 卡牌系统接入

CardService 在卡牌升级、合成、卡组数量上读取研究台等级。

示例：

- 研究台 Lv.1 允许卡牌升到 Lv.2。
- 研究台 Lv.2 解锁碎片合成。
- 研究台 Lv.4 解锁第二套卡组方案。

卡牌系统不直接修改工坊数据。

只读取工坊能力。

### 9.3 背包系统接入

仓库等级影响材料上限。

背包系统发放材料时，需要读取仓库上限。

推荐逻辑：

```text
发放材料
  -> 查询材料当前数量
  -> 查询仓库上限
  -> 判断是否允许增加
  -> 写入 PlayerItem
  -> 返回实际获得数量和溢出数量
```

### 9.4 离线收益接入

展示柜等级影响离线收益效率和最大累计时间。

离线收益领取由 WorkshopService 负责。

AssetService 只负责发放最终奖励。

## 10. 离线收益设计

### 10.1 系统目的

离线收益用于：

- 提高回流体验。
- 让休闲玩家离线后也有轻微进度。
- 承接展示柜升级价值。
- 增加工坊“持续经营”的感觉。

离线收益不用于：

- 替代主动游玩。
- 成为主要资源来源。
- 强制玩家定时上线收菜。

### 10.2 开启条件

MVP 开启条件：

- 玩家完成第 5 局。
- 展示柜解锁。

领取条件：

- 离线时长至少 5 分钟。
- 最大累计时长由展示柜等级决定。

### 10.3 离线时长

```text
offline_seconds = now - last_offline_reward_at
effective_seconds = min(offline_seconds, max_offline_seconds)
```

如果玩家第一次领取：

```text
last_offline_reward_at = last_logout_at
```

如果没有可靠登出时间：

```text
last_offline_reward_at = last_active_at
```

### 10.4 基础产出

离线收益建议基于玩家当前进度计算。

基础公式：

```text
base_gold_per_hour = chapter_base_gold_per_hour
base_material_per_hour = chapter_base_material_per_hour

gold = base_gold_per_hour * hours * showcase_gold_rate * decoration_gold_rate
material = base_material_per_hour * hours * showcase_material_rate
```

其中：

- `chapter_base_gold_per_hour` 来自章节配置。
- `showcase_gold_rate` 来自展示柜等级。
- `decoration_gold_rate` 来自装饰总加成。

MVP 可简化为：

```text
gold = player_level * 20 * hours * showcase_rate
wood = player_level * 2 * hours
```

### 10.5 收益上限

离线收益需要上限。

建议：

- 最大累计时间：4 到 10 小时。
- 单日最大领取次数：不限制，但总量自然受累计时间限制。
- 广告双倍：后续商业化文档再定义。

### 10.6 领取流程

```text
玩家打开工坊
  -> 客户端请求 workshop.get_overview
  -> 服务器计算可领取离线收益
  -> 客户端展示离线收益弹窗
  -> 玩家点击领取
  -> 客户端请求 workshop.claim_offline_reward(req_id)
  -> WorkshopService 再次计算收益
  -> AssetService 发放奖励
  -> 更新 last_offline_reward_at
  -> 返回奖励明细
```

为什么领取时要再次计算：

- 避免客户端伪造奖励。
- 避免展示后长时间不领取导致时间变化。
- 保证服务器最终结果一致。

### 10.7 离线收益异常

异常情况：

| 情况 | 处理 |
| --- | --- |
| 离线时长不足 | 返回空奖励 |
| req_id 重复 | 返回上次领取结果 |
| 系统时间异常 | 使用服务器时间 |
| 玩家数据缺失 | 初始化默认工坊数据 |
| 背包已满 | 按资产系统规则处理溢出 |

## 11. 装饰系统设计

### 11.1 系统目的

装饰系统提供：

- 长期收集。
- 视觉表达。
- 活动奖励承接。
- 轻量数值加成。

MVP 阶段不做复杂摆放。

建议先做“装饰拥有 + 展示槽位 + 总加成”。

### 11.2 装饰类型

| 类型 | 说明 | 示例 |
| --- | --- | --- |
| wall | 墙面 | 砖墙、樱花墙纸 |
| floor | 地板 | 木地板、糖果地砖 |
| sign | 招牌 | 猫爪招牌 |
| furniture | 家具 | 小桌、椅子、柜子 |
| ornament | 小摆件 | 花瓶、面包篮 |
| theme | 主题套装 | 樱花工坊、海边工坊 |

MVP 只需要：

- wall。
- floor。
- sign。
- ornament。

### 11.3 装饰品质

| 品质 | 定位 | 加成建议 |
| --- | --- | --- |
| common | 常规收集 | 0% 到 0.2% |
| rare | 活动或中期奖励 | 0.3% 到 0.5% |
| epic | 高价值活动奖励 | 0.6% 到 1.0% |
| legendary | 稀有收藏 | 1.0% 到 2.0% |

注意：

- 加成不应成为付费压力。
- 高品质装饰更应该依赖外观价值。

### 11.4 装饰加成

装饰加成类型：

| bonus_type | 说明 |
| --- | --- |
| offline_gold_bonus | 离线金币收益 |
| order_gold_bonus | 订单金币收益 |
| material_bonus | 材料获得 |
| bread_order_bonus | 面包类订单收益 |
| card_exp_bonus | 卡牌经验获得 |

MVP 建议只开放：

- 离线金币收益。
- 订单金币收益。

### 11.5 加成上限

装饰总加成必须有上限。

建议：

| 加成类型 | MVP 上限 |
| --- | --- |
| 离线金币收益 | 10% |
| 订单金币收益 | 5% |
| 材料获得 | 5% |
| 卡牌经验获得 | 5% |

设计理由：

- 避免装饰堆叠破坏经济。
- 保留装饰的收集价值。
- 后续活动可以安全投放装饰。

### 11.6 装饰获得

来源：

- 主线章节奖励。
- 活动兑换。
- 成就奖励。
- 商店购买。
- 月卡或礼包。

MVP 来源：

- 第一章通关奖励。
- 新手任务奖励。
- 商店金币购买 2 到 3 个普通装饰。

### 11.7 装饰展示规则

MVP 展示规则：

- 每个类型只能展示 1 件。
- 展示后生效该装饰加成。
- 未展示的装饰只收藏，不生效。

后续可扩展：

- 多槽位摆放。
- 自由拖拽。
- 套装加成。
- 好友参观。

## 12. 工坊主界面信息结构

主界面需要展示：

- 玩家当前章节。
- 当前订单入口。
- 设施入口。
- 可升级设施红点。
- 离线收益提示。
- 装饰入口。
- 每日领取入口。
- 资源栏。

推荐布局：

```text
顶部资源栏：金币 / 钻石 / 体力 / 声望

中部工坊场景：
  烤炉
  仓库
  订单板
  展示柜
  研究台
  休息区

底部导航：
  订单
  卡牌
  背包
  工坊
  活动

右侧提示：
  可升级设施
  离线收益
  每日领取
```

### 12.1 设施红点

红点条件：

- 设施已解锁。
- 未达到最大等级。
- 当前资源足够升级。
- 玩家等级或关卡条件满足。

红点不应过度打扰。

如果多个设施可升级，主界面只显示一个总红点。

进入工坊后再显示具体设施红点。

### 12.2 离线收益提示

提示条件：

- 有可领取离线收益。
- 可领取金币大于最小展示阈值。

最小展示阈值建议：

```text
gold >= 10
or offline_seconds >= 30 minutes
```

### 12.3 设施详情弹窗

设施详情展示：

- 设施名称。
- 当前等级。
- 当前效果。
- 下一级效果。
- 升级消耗。
- 升级按钮。
- 最大等级提示。
- 解锁条件提示。

## 13. 配置结构

### 13.1 FacilityConfig

```text
FacilityConfig
- facility_id
- name
- desc
- max_level
- unlock_condition
- sort_order
- icon
- scene_node
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| facility_id | 设施 ID |
| name | 显示名称 |
| desc | 设施描述 |
| max_level | 最大等级 |
| unlock_condition | 解锁条件 |
| sort_order | 排序 |
| icon | 图标资源 |
| scene_node | 场景节点 |

### 13.2 FacilityLevelConfig

```text
FacilityLevelConfig
- facility_id
- level
- upgrade_costs
- upgrade_condition
- effects
- visual_stage
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| facility_id | 设施 ID |
| level | 设施等级 |
| upgrade_costs | 升到该等级所需消耗 |
| upgrade_condition | 升级条件 |
| effects | 当前等级效果 |
| visual_stage | 外观阶段 |

注意：

- `level = 1` 通常不需要消耗。
- `upgrade_costs` 表示从上一级升到当前等级的成本。

### 13.3 DecorationConfig

```text
DecorationConfig
- decoration_id
- name
- type
- quality
- theme
- bonuses
- icon
- asset_path
- obtain_desc
```

### 13.4 OfflineRewardConfig

```text
OfflineRewardConfig
- chapter_id
- base_gold_per_hour
- base_materials_per_hour
- min_seconds
- max_seconds_default
```

## 14. 玩家数据结构

### 14.1 PlayerWorkshop

```text
PlayerWorkshop
- uid
- level
- active_theme_id
- last_offline_reward_at
- created_at
- updated_at
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| uid | 玩家 ID |
| level | 工坊总等级，可选 |
| active_theme_id | 当前主题 |
| last_offline_reward_at | 上次离线收益结算时间 |
| created_at | 创建时间 |
| updated_at | 更新时间 |

MVP 可以不使用工坊总等级。

如果使用工坊总等级，建议由设施等级之和推导，不手动维护。

### 14.2 PlayerFacility

```text
PlayerFacility
- uid
- facility_id
- level
- unlocked
- unlocked_at
- updated_at
```

### 14.3 PlayerDecoration

```text
PlayerDecoration
- uid
- decoration_id
- count
- obtained_at
- updated_at
```

装饰通常是唯一拥有。

如果存在重复装饰，可以转换为装饰碎片。

### 14.4 PlayerDecorationDisplay

```text
PlayerDecorationDisplay
- uid
- slot_type
- decoration_id
- updated_at
```

MVP 使用槽位展示。

后续自由摆放可扩展为：

```text
PlayerDecorationPlacement
- uid
- decoration_id
- scene_id
- x
- y
- rotation
- scale
- layer
```

### 14.5 WorkshopUpgradeLog

```text
WorkshopUpgradeLog
- log_id
- uid
- facility_id
- from_level
- to_level
- costs
- req_id
- created_at
```

用途：

- 查询升级记录。
- 排查资源扣除问题。
- 支撑数据分析。

## 15. 客户端业务动作建议

本文档只描述工坊系统需要支持的业务动作和字段含义，不维护正式协议号、`op_code`、HTTP 路径或错误码枚举。

正式协议、`op_code`、错误码和消息结构以 [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md) 为准。

### 15.1 获取工坊总览

请求字段：

```text
- uid
```

响应：

```text
payload:
- workshop
- facilities
- decorations
- display_decorations
- offline_reward_preview
- upgrade_available
```

用途：

- 进入工坊主界面。
- 登录后同步工坊状态。

### 15.2 升级设施

请求字段：

```text
- facility_id
- req_id
```

响应：

```text
payload:
- facility_id
- old_level
- new_level
- effects
- asset_changes
- next_upgrade_preview
```

业务失败原因：

| 原因 | 说明 |
| --- | --- |
| FACILITY_NOT_FOUND | 设施不存在 |
| FACILITY_LOCKED | 设施未解锁 |
| FACILITY_MAX_LEVEL | 已达到最大等级 |
| COST_NOT_ENOUGH | 资源不足 |
| CONDITION_NOT_MET | 升级条件不满足 |
| DUPLICATE_REQ | 重复请求 |

### 15.3 领取离线收益

请求字段：

```text
- req_id
```

响应：

```text
payload:
- rewards
- offline_seconds
- effective_seconds
- next_claim_at
- asset_changes
```

### 15.4 装饰展示

请求字段：

```text
- slot_type
- decoration_id
```

响应：

```text
payload:
- slot_type
- decoration_id
- decoration_bonuses
- total_decoration_bonuses
```

### 15.5 获取可升级设施

请求字段：

```text
- uid
```

响应：

```text
payload:
- upgradeable_facility_ids
- locked_facility_hints
```

MVP 可不做独立协议动作。

可以在“获取工坊总览”中一起返回。

## 16. 服务边界

### 16.1 WorkshopService

负责：

- 工坊总览。
- 设施解锁。
- 设施升级。
- 设施效果汇总。
- 离线收益计算。
- 装饰展示。

不负责：

- 具体资产扣除和发放。
- 订单结算主逻辑。
- 卡牌升级主逻辑。

### 16.2 AssetService

负责：

- 扣除设施升级消耗。
- 发放离线收益。
- 写资产流水。
- 幂等处理。

### 16.3 OrderService

负责：

- 订单生成。
- 订单结算。
- 读取工坊设施效果。

OrderService 不应该直接修改设施等级。

### 16.4 CardService

负责：

- 卡牌升级。
- 卡牌合成。
- 卡组保存。
- 读取研究台能力。

CardService 不应该直接修改研究台等级。

## 17. 时序流程

### 17.1 登录进入工坊

```text
客户端登录游戏服
  -> Auth 验票
  -> Session 建立连接
  -> Client 请求工坊总览
  -> WorkshopService 读取 PlayerWorkshop
  -> WorkshopService 读取 PlayerFacility
  -> WorkshopService 读取 PlayerDecoration
  -> WorkshopService 计算离线收益预览
  -> WorkshopService 计算可升级红点
  -> 返回工坊总览
  -> Client 渲染工坊主界面
```

### 17.2 设施升级

```text
Client 点击烤炉升级
  -> 发送升级设施请求(facility_id=oven, req_id)
  -> Gateway 收到消息
  -> 协议层分发到工坊业务动作
  -> WorkshopHandler 解析参数
  -> WorkshopService 校验设施配置
  -> WorkshopService 校验玩家设施等级
  -> WorkshopService 生成 CostItem
  -> AssetService 在事务中扣除资产
  -> WorkshopRepository 更新 PlayerFacility
  -> WorkshopUpgradeLog 记录升级
  -> 返回新等级和资产变化
  -> Client 刷新设施表现和资源栏
```

### 17.3 领取离线收益

```text
Client 打开工坊
  -> 工坊总览返回 offline_reward_preview
  -> Client 展示离线收益弹窗
  -> 玩家点击领取
  -> 请求领取离线收益(req_id)
  -> WorkshopService 读取 last_offline_reward_at
  -> WorkshopService 计算 effective_seconds
  -> WorkshopService 读取展示柜等级和装饰加成
  -> WorkshopService 生成 RewardItem
  -> AssetService 幂等发奖
  -> WorkshopRepository 更新 last_offline_reward_at
  -> 返回奖励明细
```

### 17.4 装饰展示

```text
Client 选择装饰
  -> 请求设置装饰展示(slot_type, decoration_id)
  -> WorkshopService 校验玩家是否拥有装饰
  -> WorkshopService 校验装饰类型是否匹配槽位
  -> WorkshopRepository 更新展示槽位
  -> WorkshopService 重新计算装饰总加成
  -> 返回展示结果和加成结果
```

## 18. 数值建议

### 18.1 设施等级

MVP 每个设施 5 级。

第一章只要求玩家自然升级到：

- 烤炉 Lv.3。
- 仓库 Lv.2。
- 订单板 Lv.2。
- 展示柜 Lv.2。
- 研究台 Lv.2。
- 休息区 Lv.1。

不要要求第一章全部升满。

### 18.2 升级成本曲线

建议成本曲线：

```text
level 1 -> 2: 低成本，教学成本
level 2 -> 3: 需要 2 到 3 局收益
level 3 -> 4: 需要 1 天轻度活跃
level 4 -> 5: 需要 2 到 3 天积累
```

示例金币成本：

| 升级 | 金币成本 |
| --- | --- |
| Lv.1 -> Lv.2 | 100 |
| Lv.2 -> Lv.3 | 300 |
| Lv.3 -> Lv.4 | 800 |
| Lv.4 -> Lv.5 | 1600 |

材料成本随设施主题变化。

### 18.3 收益控制

设施收益建议：

- 订单金币收益单设施满级不超过 20%。
- 离线收益满级不超过初始值 150%。
- 装饰订单收益总加成不超过 5%。
- 装饰离线收益总加成不超过 10%。

### 18.4 第一日目标

第一日希望玩家完成：

- 解锁烤炉。
- 升级烤炉 1 次。
- 解锁仓库。
- 解锁订单板。
- 看见展示柜或离线收益入口。
- 获得 1 件装饰。

第一日不强制：

- 装饰摆放深度操作。
- 多设施同时升级。
- 卡牌复杂合成。

## 19. MVP 范围

MVP 必须做：

- 工坊总览接口。
- 设施配置。
- 玩家设施等级。
- 设施升级。
- 资产扣除接入。
- 展示柜离线收益。
- 研究台卡牌升级上限。
- 装饰拥有和展示槽位。

MVP 可以简化：

- 工坊总等级不做。
- 自由摆放不做。
- 猫咪派遣不做。
- 好友参观不做。
- 主题套装不做。
- 装饰套装加成不做。

MVP 不建议做：

- 多工坊地图。
- 装饰排行榜。
- 复杂生产队列。
- 多建筑同时施工。
- 建造等待时间。

## 20. 配置示例

### 20.1 烤炉配置示例

```text
facility_id: oven
name: 烤炉
max_level: 5
unlock_condition:
  level_id: 1
levels:
  - level: 1
    effects:
      - order_reward_bonus bread gold 0%
  - level: 2
    costs:
      - asset gold 100
      - item wood 5
    effects:
      - order_reward_bonus bread gold 3%
  - level: 3
    costs:
      - asset gold 300
      - item wood 10
      - item flour 20
    effects:
      - order_reward_bonus bread gold 7%
```

### 20.2 展示柜配置示例

```text
facility_id: showcase
name: 展示柜
max_level: 5
unlock_condition:
  level_id: 5
levels:
  - level: 1
    effects:
      - offline_gold_rate all percent 100
      - offline_max_seconds all fixed 14400
  - level: 2
    costs:
      - asset gold 200
      - item glass 5
    effects:
      - offline_gold_rate all percent 110
      - offline_max_seconds all fixed 18000
```

### 20.3 装饰配置示例

```text
decoration_id: deco_bread_basket_001
name: 面包篮
type: ornament
quality: common
bonuses:
  - bonus_type: offline_gold_bonus
    value_type: percent
    value: 0.2
obtain_desc: 第一章任务奖励
```

## 21. 研发实现建议

### 21.1 文档边界

本文档只提供工坊玩法侧实现建议。

后端目录、接口签名、缓存策略和协议定义以 [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md) 为准。

### 21.2 Effect 汇总

设施和装饰都可以产生效果。

推荐统一汇总为：

```text
WorkshopEffects
- order_bonus_by_tag
- offline_gold_bonus
- offline_material_bonus
- material_capacity_bonus
- card_level_limit
- deck_slot_bonus
```

调用方不关心效果来自设施还是装饰。

这样后续拆分和扩展更容易。

### 21.3 状态建议

工坊长期数据必须持久化。

可缓存对象：

- PlayerFacility。
- PlayerDecorationDisplay。
- WorkshopEffects 汇总结果。

缓存失效：

- 设施升级后删除或刷新 WorkshopEffects。
- 装饰展示变化后删除或刷新 WorkshopEffects。
- 领取离线收益后刷新 PlayerWorkshop。

## 22. 验收标准

### 22.1 程序验收

- 新玩家进入工坊时自动初始化默认设施。
- 已解锁设施可以正常展示。
- 未解锁设施展示解锁条件。
- 资源足够时可以升级设施。
- 资源不足时升级失败且不扣除资源。
- 重复升级请求不会重复扣除资源。
- 离线收益可以预览和领取。
- 重复领取请求不会重复发奖。
- 装饰展示后总加成正确刷新。
- 工坊总览动作能一次返回主界面所需信息。

### 22.2 数值验收

- 第一日玩家至少能完成 1 到 2 次设施升级。
- 第一章玩家能体验到 3 个以上设施。
- 离线收益不超过主动游玩日收益的 25%。
- 装饰总加成不破坏订单经济。
- 设施升级成本不会导致新手卡死。

### 22.3 体验验收

- 玩家知道为什么要升级设施。
- 玩家升级后能看到明确变化。
- 工坊主界面不会信息过载。
- 红点提示不过度打扰。
- 离线收益弹窗不会频繁打断。

### 22.4 QA 验收

- 修改客户端时间不会影响离线收益。
- 断线重连后工坊状态一致。
- 多次快速点击升级不会重复扣费。
- 背包材料满时升级和发奖规则正确。
- 删除配置或配置错误时服务返回明确错误。

## 23. 后续任务

系统策划：

- 补全 6 个设施 Lv.1 到 Lv.5 的完整效果。
- 确认设施解锁关卡。
- 定义装饰槽位数量。

数值策划：

- 补全设施升级消耗表。
- 验证第一日和第一章资源产出是否支撑升级。
- 计算离线收益占比。

关卡策划：

- 在前 10 局中安排设施解锁教学。
- 安排订单奖励与设施升级材料的对应关系。

客户端：

- 设计工坊主界面布局。
- 设计设施详情弹窗。
- 设计离线收益弹窗。

服务端：

- 实现工坊配置加载。
- 实现设施升级事务。
- 实现离线收益计算。
- 实现 WorkshopEffects 汇总。

## 24. 与其他文档关系

- `docs/design/card_casual_game_design.md`：定义工坊在整体游戏循环中的位置。
- `docs/design/inventory_asset_design.md`：定义设施升级消耗、离线收益发奖和资产流水。
- `docs/design/order_level_design.md`：定义订单如何读取工坊收益加成。
- `docs/design/card_system_design.md`：定义研究台如何影响卡牌升级和合成。
- `docs/design/economy_design.md`：后续定义金币、材料、离线收益和商业化之间的平衡。
