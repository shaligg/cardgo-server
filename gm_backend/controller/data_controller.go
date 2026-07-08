package controller

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// GetGameStatistics 获取游戏统计数据
func GetGameStatistics(c *gin.Context) {
	// 模拟统计数据
	statistics := map[string]interface{}{
		"total_players": 15320,
		"active_players": 8765,
		"new_players_today": 321,
		"recharge_total": 156800,
		"recharge_today": 5680,
		"server_load": 45.2,
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取成功",
		"data": statistics,
	})
}

// GetRechargeRecord 获取充值记录
func GetRechargeRecord(c *gin.Context) {
	// 获取查询参数
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 模拟充值记录
	rechargeRecords := []map[string]interface{}{
		{
			"id": 1001,
			"player_id": 10001,
			"player_name": "玩家A",
			"amount": 648,
			"time": date + " 10:23:45",
			"status": "success",
		},
		{
			"id": 1002,
			"player_id": 10002,
			"player_name": "玩家B",
			"amount": 328,
			"time": date + " 11:45:23",
			"status": "success",
		},
		{
			"id": 1003,
			"player_id": 10003,
			"player_name": "玩家C",
			"amount": 198,
			"time": date + " 14:30:12",
			"status": "success",
		},
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取成功",
		"data": gin.H{
			"date": date,
			"list": rechargeRecords,
		},
	})
}

// GetOnlinePlayers 获取在线玩家
func GetOnlinePlayers(c *gin.Context) {
	// 模拟在线玩家数据
	onlinePlayers := []map[string]interface{}{
		{
			"player_id": 10001,
			"player_name": "玩家A",
			"level": 56,
			"vip": 7,
			"server_id": 1,
			"online_time": "2小时30分",
		},
		{
			"player_id": 10002,
			"player_name": "玩家B",
			"level": 45,
			"vip": 5,
			"server_id": 2,
			"online_time": "1小时15分",
		},
		{
			"player_id": 10003,
			"player_name": "玩家C",
			"level": 67,
			"vip": 10,
			"server_id": 1,
			"online_time": "5小时20分",
		},
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取成功",
		"data": gin.H{
			"total": len(onlinePlayers),
			"list": onlinePlayers,
		},
	})
}