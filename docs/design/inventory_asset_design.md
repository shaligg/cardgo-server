# 背包与资产系统设计文档

## 1. 文档目标

本文档定义玩家长期资产、背包道具、卡牌库存、资产变更、发奖扣除、幂等、防重复结算和后端数据边界。

相关文档：

- `docs/design/card_casual_game_design.md`
- `docs/design/card_system_design.md`
- `docs/design/order_level_design.md`
- `docs/design/economy_design.md`

## 2. 系统定位

背包与资产系统负责保存玩家长期拥有的资源。

本游戏需要明确区分：

- 局内资源：只在单局中存在，例如面包、木材、布料、魔法粉。
- 局外资产：长期持有，例如金币、钻石、体力、材料、卡牌、道具。

设计目标：

- 所有发奖、扣除和消耗都有统一入口。
- 所有关键资产变更都有日志。
- 支持请求幂等，避免重复发奖和重复扣费。
- 支撑卡牌、工坊、订单、活动、商业化共同使用。

## 3. 核心原则

### 3.1 局内资源不进背包

局内资源只用于本局订单结算。

例如：

- 面包。
- 木材。
- 布料。
- 魔法粉。

这些资源本局结束后清空，只把最终奖励写入局外资产或背包。

### 3.2 货币资产独立于普通道具

金币、钻石、体力、声望等高频货币应存入资产表，不和普通道具混在同一张背包表里。

原因：

- 读写频率高。
- 数值需要强一致。
- 需要清晰流水。
- 前端展示位置固定。

### 3.3 卡牌库存独立于普通道具

卡牌不是普通道具。卡牌有等级、经验、数量、碎片、上阵状态等信息，应独立建模。

### 3.4 所有变更必须有原因

每次资产变化必须记录来源。

来源示例：

- 主线关卡。
- 每日委托。
- 无尽订单。
- 活动兑换。
- 商店购买。
- 卡牌升级。
- 工坊升级。
- GM 补发。

## 4. 资产类型

### 4.1 核心资产

| asset_type | 名称 | 用途 | 是否高频 |
|---|---|---|---|
| `gold` | 金币 | 卡牌升级、设施升级、普通商店 | 是 |
| `gem` | 钻石 | 高级卡包、体力、礼包 | 是 |
| `stamina` | 体力 | 主线关卡消耗 | 是 |
| `reputation` | 声望 | 章节解锁、工坊等级 | 中 |
| `friendship_coin` | 友情币 | 社交互助商店 | 中 |

### 4.2 活动资产

活动币可以进入资产表，也可以进入活动进度表。

建议：

- 常规活动币使用 `PlayerEventProgress`。
- 可跨活动使用的通用活动币才进入资产表。

MVP 暂不设计跨活动通用币。

### 4.3 资产上限

建议：

- 金币：无硬上限。
- 钻石：无硬上限。
- 体力：有自然恢复上限，但可超上限保存购买或奖励体力。
- 声望：无硬上限。
- 友情币：可设置软上限，超过后仍可获得。


## 4.4 资源与道具存储原则
资源和道具统一使用策划配置 ID 标识，推荐字段名为 `item_id`。

规则：

1. 高频基础货币可以直接存玩家基础表，例如金币、钻石、体力、声望。
2. 通用背包表只存可堆叠道具，例如材料、碎片、消耗券、宝箱钥匙。
3. 只在单一系统内产消的专属资源，可以存所属系统表，例如竞技币、公会贡献、活动币。
4. 不可堆叠道具不进入通用背包表，由所属系统单独建表。
5. 无论存在哪里，奖励、消耗和日志都必须记录统一 `item_id`，便于策划配置和日志查询。

示例：

| item_id | 名称 | 存储方式 | 说明 |
|---|---|---|---|
| `1` | 金币 | 玩家基础表字段 | 高频基础货币 |
| `2` | 钻石 | 玩家基础表字段 | 高频基础货币 |
| `10001` | 基础材料 | 通用背包表 | 可堆叠通用道具 |
| `20001` | 面包师卡 | 卡牌系统表 | 不可堆叠或卡牌系统拥有记录 |
| `30001` | 竞技币 | 竞技场系统表 | 系统专属资源 |

## 4.5 ItemConfig
`ItemConfig` 是策划静态配置，不是玩家业务数据表。

策划源文件使用 Excel，程序运行配置使用 Excel 导出的 JSON。

服务端启动时加载 JSON 到内存，并执行字段、重复 ID 和引用校验。

字段建议：

```text
ItemConfig
- item_id
- key
- name
- item_type
- storage_type
- storage_key
- system
- stackable
- visible_in_bag
- icon
- desc
```

运行配置路径示例：

```text
configs/gamedata/items.json
```

`storage_type` 示例：

| storage_type | 说明 |
|---|---|
| `player_field` | 存玩家基础表字段，例如 gold |
| `inventory_stack` | 存通用可堆叠背包表 |
| `card_instance` | 存卡牌系统表 |
| `decoration_instance` | 存工坊装饰系统表 |
| `arena_field` | 存竞技场系统字段 |
| `event_field` | 存活动系统字段 |

`CostService/RewardService` 根据 `item_id` 查 `ItemConfig`，再决定实际写入位置。

## 5. 普通道具

通用普通道具表只存可堆叠道具。不可堆叠道具由所属系统单独建表，例如卡牌、装饰、宠物、装备等。

### 5.1 道具类型

| item_type | 名称 | 示例 |
|---|---|---|
| `material` | 材料道具 | 基础材料、高级材料 |
| `ticket` | 消耗券 | 订单刷新券、双倍金币券 |
| `chest` | 宝箱 | 材料箱、卡包 |
| `decoration_fragment` | 装饰碎片 | 装饰合成材料 |
| `skin_ticket` | 皮肤券 | 皮肤兑换或抽取券 |
| `fragment` | 碎片 | 卡牌碎片、装饰碎片 |

### 5.2 材料道具

局外材料用于卡牌升级、设施升级和合成。

MVP 材料：

- `basic_material`：基础材料。
- `advanced_material`：高级材料。

不建议 MVP 阶段把材料拆得太细。

### 5.3 消耗券

消耗券用于局外或局内入口。

示例：

- 订单刷新券。
- 双倍金币券。
- 关卡扫荡券。
- 体力补充券。

### 5.4 宝箱

宝箱打开后发放多种奖励。

宝箱需要配置：

- 奖池。
- 权重。
- 保底。
- 每日打开上限。
- 是否可批量打开。

MVP 可以先做固定奖励宝箱，随机宝箱后置。

## 6. 卡牌库存

### 6.1 PlayerCard

玩家拥有的卡牌独立存储。

字段建议：

```text
PlayerCard
- uid
- card_id
- level
- exp
- count
- created_at
- updated_at
```

### 6.2 卡牌数量

`count` 表示玩家拥有的该卡数量。

用途：

- 卡组编成。
- 升级消耗同名卡。
- 分解为碎片。

### 6.3 卡牌碎片

卡牌碎片可以作为普通道具存在。

命名建议：

```text
card_fragment_{card_id}
```

MVP 可简化为：

- 普通碎片。
- 稀有碎片。
- 史诗碎片。

### 6.4 卡牌获得

获得卡牌时：

- 如果玩家没有该卡，创建 `PlayerCard`。
- 如果已有该卡，增加 `count`。
- 如果卡牌已满级，多余卡牌暂不自动分解，交给后续功能处理。

## 7. 奖励结构

### 7.1 RewardItem

统一奖励结构：

```text
RewardItem
- item_id
- count
```

示例：

```json
{
  "item_id": 1,
  "count": 100
}
```

```json
{
  "item_id": 10001,
  "count": 5
}
```

```json
{
  "item_id": 20001,
  "count": 1
}
```

### 7.2 奖励路由

`RewardItem` 不再区分 `reward_type/reward_id`。业务只提交 `item_id/count`，由 `AssetService` 查询 `ItemConfig` 后路由到实际存储位置。

MVP 推荐分阶段支持：

- B1：`player_field`，金币等玩家基础表字段。
- B2：`inventory_stack`，材料、碎片、消耗券等通用可堆叠道具。
- B3：`card_instance`，卡牌系统表，按卡牌系统实现节奏接入。

## 8. 消耗结构

### 8.1 CostItem

统一消耗结构：

```text
CostItem
- item_id
- count
```

消耗也只提交 `item_id/count`，不在业务配置中区分 `asset/item/card`。

实际扣除位置由 `ItemConfig.storage_type` 决定。

### 8.2 扣除原则

扣除必须满足：

- 先校验数量是否足够。
- 再执行扣除。
- 扣除和业务操作放在同一事务。
- 失败时全部回滚。

### 8.3 常见消耗场景

- 卡牌升级：金币、卡牌碎片、同名卡。
- 工坊升级：金币、基础材料、高级材料。
- 商店购买：钻石、金币、友情币。
- 活动兑换：活动币。
- 进入关卡：体力。

## 9. 资产变更流程

### 9.1 职责边界

玩法层负责判断：

- 玩家能不能领取。
- 本次要扣什么、扣多少。
- 本次要发什么、发多少。
- 需要更新哪些玩法状态。

通用资产层只负责执行：

- 按 `cost_list` 扣除资产。
- 按 `reward_list` 发放资产。
- 对同一批 `cost_list/reward_list` 中相同 `item_id` 的项做标准化合并，例如 `coin:1 + coin:1` 合并为 `coin:2`。
- 校验扣除后不能为负数。
- 写资产流水。
- 保证同一个 `req_id/action` 不重复执行。

`Repository` 只负责表读写和数据转换，不负责调用发奖，也不编排跨玩法业务。

### 9.2 发奖流程

独立发奖入口适用于 GM 补偿、邮件附件、测试加金币等不需要绑定玩法状态的场景：

```text
业务模块
  -> RewardService.Grant(req_id, uid, reward_list, reason)
  -> 校验 req_id 是否已处理
  -> 开启事务
  -> 写入资产/道具/卡牌
  -> 写入流水
  -> 提交事务
  -> 返回最新资产快照
```

玩法领奖通常需要同时更新玩法状态，必须使用事务内入口：

```text
玩法 Service
  -> 计算 reward_list
  -> TxManager.Do(tx)
      -> RewardService.ApplyRewardInTx(tx, uid, reward_list, reason, req_id)
         - 合并相同 item_id
         - 校验数量和道具配置
         - 写资产与流水
      -> 玩法 Repository 更新领奖状态/进度
      -> 写入幂等结果
  -> 提交事务
```

### 9.3 扣除流程

独立扣除入口适用于测试、GM 或不需要绑定玩法状态的场景：

```text
业务模块
  -> AssetService.Consume(req_id, uid, cost_list, reason)
  -> 校验 req_id 是否已处理
  -> 开启事务
  -> 校验余额
  -> 扣除资产/道具/卡牌
  -> 写入流水
  -> 提交事务
  -> 返回最新资产快照
```

玩法消耗通常需要同时更新玩法结果，必须使用事务内入口：

```text
玩法 Service
  -> 判断是否满足玩法条件
  -> 计算 cost_list
  -> TxManager.Do(tx)
      -> CostService.ApplyCostInTx(tx, uid, cost_list, reason, req_id)
         - 合并相同 item_id
         - 校验数量和道具配置
         - 扣资产并写流水
      -> 玩法 Repository 更新成长/购买/结算状态
      -> 写入幂等结果
  -> 提交事务
```

### 9.4 结算流程

关卡结算同时包含发奖和首通记录。

```text
BattleService.Settle
  -> 校验 battle_id
  -> 校验胜负和订单完成
  -> 计算奖励
  -> TxManager.Do(tx)
      -> RewardService.ApplyRewardInTx(tx, reward_list)
      -> Battle/Level Repository 写入关卡进度
      -> 写入幂等结果
  -> 返回结算结果
```

## 10. 幂等设计

### 10.1 req_id

所有会改变资产的请求必须携带 `req_id`。

`req_id` 来源：

- 客户端请求生成。
- 服务端结算生成。
- 活动任务领取生成。
- GM 操作生成。

### 10.2 幂等记录

建议表：

```text
IdempotencyRecord
- req_id
- uid
- action
- status
- result_json
- created_at
- updated_at
```

### 10.3 幂等返回

重复请求命中已成功记录时，直接返回第一次结果。

重复请求命中处理中记录时：

- 可以返回 `PROCESSING`。
- 或短时间等待后返回。

MVP 推荐直接返回 `PROCESSING`。

### 10.4 防重复发奖

关键场景必须幂等：

- 关卡结算。
- 首通奖励。
- 每日委托领取。
- 活动兑换。
- 卡牌升级。
- 商店购买。
- 广告奖励。

## 11. 资产流水

### 11.1 AssetLog

建议字段：

```text
AssetLog
- log_id
- uid
- asset_type
- delta
- before_value
- after_value
- reason
- req_id
- ref_id
- created_at
```

### 11.2 ItemLog

建议字段：

```text
ItemLog
- log_id
- uid
- item_id
- delta
- before_count
- after_count
- reason
- req_id
- ref_id
- created_at
```

### 11.3 CardLog

建议字段：

```text
CardLog
- log_id
- uid
- card_id
- delta_count
- before_count
- after_count
- reason
- req_id
- ref_id
- created_at
```

## 12. 数据结构

### 12.1 PlayerAsset

```text
PlayerAsset
- uid
- gold
- gem
- stamina
- stamina_updated_at
- reputation
- friendship_coin
- created_at
- updated_at
```

### 12.2 PlayerItem

```text
PlayerItem
- uid
- item_id
- count
- created_at
- updated_at
```

唯一键：

```text
uid + item_id
```

### 12.3 PlayerCard

```text
PlayerCard
- uid
- card_id
- level
- exp
- count
- created_at
- updated_at
```

唯一键：

```text
uid + card_id
```

### 12.4 PlayerDeck

```text
PlayerDeck
- uid
- deck_id
- name
- card_list_json
- is_active
- created_at
- updated_at
```

### 12.5 RewardConfig

```text
RewardConfig
- reward_group_id
- item_id
- count
- weight
- guarantee_rule
```

## 13. 客户端展示规则

### 13.1 资产展示

常驻展示：

- 金币。
- 钻石。
- 体力。

二级展示：

- 声望。
- 友情币。
- 活动币。

### 13.2 背包展示

背包标签：

- 材料。
- 消耗。
- 宝箱。
- 装饰。
- 碎片。

### 13.3 卡牌展示

卡牌不放入普通背包页，使用独立卡牌图鉴和卡组页。

卡牌页展示：

- 已拥有。
- 未拥有。
- 可升级。
- 可编入卡组。
- 流派标签。

## 14. 客户端业务动作建议

本文档只描述背包与资产系统需要支持的业务动作和字段含义，不维护正式 HTTP 路径、WS `op_code`、协议结构或错误码枚举。

正式协议、`op_code`、错误码和消息结构以 [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md) 为准。

### 14.1 查询资产

```text
查询玩家资产
```

返回：

```text
PlayerAsset
```

### 14.2 查询背包

```text
查询玩家背包
```

返回：

```text
item_list
```

### 14.3 查询卡牌

```text
查询玩家卡牌
```

返回：

```text
card_list
```

### 14.4 保存卡组

参数：

```text
deck_id
card_list
req_id
```

### 14.5 资产变更内部能力

内部能力：

```text
发放奖励：uid + req_id + reason + reward_list
扣除消耗：uid + req_id + reason + cost_list
```

资产变更接口不建议直接暴露给客户端。

## 15. 异常处理

### 15.1 余额不足

返回：

```text
INSUFFICIENT_RESOURCE
```

需要带上：

- 资源类型。
- 当前数量。
- 需要数量。

### 15.2 重复请求

如果 `req_id` 已成功处理，返回第一次结果。

如果 `req_id` 处理中，返回：

```text
PROCESSING
```

### 15.3 配置不存在

返回：

```text
CONFIG_NOT_FOUND
```

### 15.4 非法数量

数量必须为正整数。

返回：

```text
INVALID_AMOUNT
```

## 16. MVP 范围

MVP 必须实现：

- 金币。
- 钻石。
- 体力。
- 声望。
- 基础材料。
- 高级材料。
- 玩家道具表。
- 玩家卡牌表。
- 玩家卡组表。
- 统一发奖接口。
- 统一扣除接口。
- 资产流水。
- 幂等记录。

MVP 暂缓：

- 随机宝箱复杂保底。
- 皮肤独立系统。
- 装饰仓库筛选。
- 资产邮件补发。
- 玩家间赠送资产。
- 自动分解多余卡。

## 17. 验收标准

### 17.1 程序验收

- 可以查询资产、背包、卡牌和卡组。
- 发奖和扣除在同一事务内完成。
- 重复 `req_id` 不会重复发奖。
- 余额不足不会出现部分扣除。
- 卡牌获得会正确创建或增加数量。

### 17.2 数值验收

- 前 7 日资源来源和消耗都有配置入口。
- 每个主要玩法都有明确奖励。
- 每个主要成长入口都有明确消耗。

### 17.3 QA 验收

- 单资产发奖。
- 多资产发奖。
- 单道具发奖。
- 多道具发奖。
- 卡牌发奖。
- 混合奖励。
- 余额不足。
- 重复请求。
- 卡组保存非法卡牌。

## 18. 后续任务

后续需要继续产出：

- `docs/design/economy_design.md`：具体资源产消和前 7 日经济曲线。
- `docs/design/workshop_system_design.md`：设施升级消耗如何接入资产扣除。
- `docs/design/card_system_design.md` 的卡牌升级消耗表。
- 后端资产模块接口设计。
