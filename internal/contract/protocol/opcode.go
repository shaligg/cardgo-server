// Package protocol 维护客户端与服务端共享的业务协议号。
//
// 新增业务协议时，先在这里登记 op_code，再到 handler/routes.go 绑定实现函数。
package protocol

const (
	OpPlayerGetProfile  int32 = 1001
	OpPlayerAddGold     int32 = 1002
	OpPlayerConsumeGold int32 = 1003

	OpAssetGrantItem    int32 = 1101
	OpAssetGetInventory int32 = 1102
	OpAssetConsumeItem  int32 = 1103

	OpCardGetCards int32 = 1201
	OpCardSaveDeck int32 = 1202
	OpCardUpgrade  int32 = 1203

	OpLevelStart    int32 = 1301
	OpLevelPlayCard int32 = 1302
	OpLevelSettle   int32 = 1303
	OpLevelEndTurn  int32 = 1304

	OpWorkshopGetOverview     int32 = 1401
	OpWorkshopUpgradeFacility int32 = 1402
	OpWorkshopClaimOffline    int32 = 1403

	OpWebSearch int32 = 1501

	OpFriendApply   int32 = 1601
	OpFriendApprove int32 = 1602
	OpFriendRemove  int32 = 1603
	OpFriendList    int32 = 1604

	OpGuildCreate           int32 = 1701
	OpGuildSearch           int32 = 1702
	OpGuildApplyJoin        int32 = 1703
	OpGuildApproveJoin      int32 = 1704
	OpGuildLeave            int32 = 1705
	OpGuildGet              int32 = 1706
	OpGuildListApplications int32 = 1707

	OpChatSend    int32 = 1801
	OpChatHistory int32 = 1802
)
