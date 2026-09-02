# Prototype 验收记录

## 1. 结论

- 验收日期：2026-07-03
- 验收范围：Prototype 主链路功能闭环
- 验收脚本：`scripts/loadtest/ws_prototype_smoke`、`scripts/loadtest/ws_offline_reward_smoke`
- 验收结果：历史功能链路 PASS；切换 MySQL 后待重新执行 smoke

Prototype 已跑通以下闭环：

```text
新账号
  -> 登录
  -> WS 鉴权
  -> 获取玩家资料
  -> 进入第 1 关
  -> 出牌完成订单
  -> 结算奖励
  -> 查询卡牌
  -> 升级卡牌
  -> 查询工坊
  -> 升级工坊设施
  -> 领取离线收益
  -> 重新登录后恢复玩家数据
```

## 2. 验收命令

启动服务：

```bash
GAME_DB_DSN="$GAME_DB_DSN" GAME_TICKET_SECRET="$GAME_TICKET_SECRET" go run ./cmd/gameserver
```

执行 Prototype smoke：

```bash
go run ./scripts/loadtest/ws_prototype_smoke
```

执行可控离线收益 smoke：

```bash
go run ./scripts/loadtest/ws_offline_reward_smoke
```

编译与单元测试：

```bash
go test ./...
```

## 3. 环境

- API 地址：`http://127.0.0.1:8080`
- WS 地址：`ws://127.0.0.1:8081/ws`
- 数据库：MySQL，由 `GAME_DB_DSN` 指定
- 配置目录：`configs/gamedata`

## 4. 协议覆盖

| 步骤 | op_code | 协议 | 验收点 |
| --- | ---: | --- | --- |
| 登录 | HTTP | `/api/login` | 返回 `ws_addr` 和 `enter_ticket` |
| WS 鉴权 | - | `auth_req` | 返回 `auth_ack` 和 `session_id` |
| 玩家资料 | 1001 | `player.get_profile` | 新账号自动创建玩家基础数据 |
| 开始关卡 | 1301 | `level.start` | 返回 `level_session_id`、手牌、订单 |
| 出牌 | 1302 | `level.play_card` | 打出卡牌后订单完成 |
| 结算 | 1303 | `level.settle` | 返回金币和道具奖励 |
| 测试准备 | 1002 | `player.add_gold` | 补足升级所需金币 |
| 查询卡牌 | 1201 | `card.get_cards` | 新账号补齐初始卡牌 |
| 升级卡牌 | 1203 | `card.upgrade` | 扣金币并提升卡牌等级 |
| 查询工坊 | 1401 | `workshop.get_overview` | 返回默认工坊和离线收益预览 |
| 升级设施 | 1402 | `workshop.upgrade_facility` | 扣金币并提升设施等级 |
| 领取离线收益 | 1403 | `workshop.claim_offline_reward` | 返回领取结果，短时离线奖励为 0 |
| 重新登录 | HTTP + WS | `/api/login` + `auth_req` | `auth_ack.resync` 返回玩家数据 |

## 5. 通过标准

- 所有步骤返回 `auth_ack` 或 `biz_ack`。
- 不出现 `error` 包。
- `level.start` 必须返回 `level_session_id`。
- `level.play_card` 后订单可完成。
- `level.settle` 返回奖励列表。
- `card.upgrade` 返回卡牌等级提升到 2。
- `workshop.upgrade_facility` 返回设施等级提升到 2。
- 重新登录后 `auth_ack.resync` 能返回玩家基础数据。

## 6. 实际结果摘要

最近一次 smoke 验证结果：

| 检查项 | 结果 |
| --- | --- |
| 登录 | PASS |
| WS 鉴权 | PASS |
| 玩家资料 | PASS |
| 关卡开始 | PASS |
| 出牌完成订单 | PASS |
| 关卡结算 | PASS |
| 卡牌查询 | PASS |
| 卡牌升级 | PASS |
| 工坊总览 | PASS |
| 工坊设施升级 | PASS |
| 离线收益领取 | PASS |
| 非 0 离线收益领取 | PASS |
| 重新登录恢复 | PASS |

关键返回摘要：

```text
auth => type: auth_ack, ok: true
player.get_profile => type: biz_ack, op_code: 1001
level.start => type: biz_ack, op_code: 1301, has level_session_id
level.play_card => type: biz_ack, op_code: 1302, completed_orders: 1
level.settle => type: biz_ack, op_code: 1303, rewards returned
card.upgrade => type: biz_ack, op_code: 1203, card level: 2, costs: 50 gold
workshop.get_overview => type: biz_ack, op_code: 1401
workshop.upgrade_facility => type: biz_ack, op_code: 1402, facility level: 2, costs: 100 gold + 2 basic_material
workshop.claim_offline_reward => type: biz_ack, op_code: 1403, short offline reward: 0
ws_offline_reward_smoke.claim => type: biz_ack, op_code: 1403, rewards: 40 gold + 2 basic_material
relogin.auth => type: auth_ack, has resync
```

## 7. 已知说明

- 第 1 关首通奖励不足以同时升级卡牌和工坊，smoke 中使用 `player.add_gold(1002)` 作为测试准备。
- `player.add_gold(1002)` 当前属于 demo/debug 能力，仅在 `debug.enable_ws_debug_ops=true` 时注册；staging/prod 配置默认关闭。
- 离线收益在 smoke 中通常为 0，因为领取发生在创建工坊后短时间内，未达到 5 分钟最小离线阈值。
- 当前验收是功能闭环 smoke，不等同于 2000 在线容量压测。

## 8. 后续事项

当前 MVP 不新增测试隔离代码。长期本地验证如果产生历史数据干扰，按以下方式处理：

1. 本地开发使用测试账号，必要时删号重建或清理测试库。
2. 联调环境使用专门测试服，允许按需清库，不和正式环境混用。
3. 自动化 smoke 继续使用固定测试账号；如果后续进入 CI 或多人并发测试，再评估独立数据库 DSN、临时库或测试数据命名空间。
