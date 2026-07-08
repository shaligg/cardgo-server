package controller

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// 模拟玩家数据
var players = []map[string]interface{}{
	{
		"id": 1,
		"name": "玩家1",
		"level": 30,
		"vip": 5,
		"ban": false,
		"last_login": "2023-10-10 10:00:00",
	},
	{
		"id": 2,
		"name": "玩家2",
		"level": 45,
		"vip": 8,
		"ban": true,
		"last_login": "2023-10-09 15:30:00",
	},
}

// GetPlayerList 获取玩家列表
func GetPlayerList(c *gin.Context) {
	// 获取查询参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	// 计算分页
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(players) {
		end = len(players)
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取成功",
		"data": gin.H{
			"total": len(players),
			"list":  players[start:end],
		},
	})
}

// GetPlayerInfo 获取玩家详情
func GetPlayerInfo(c *gin.Context) {
	// 获取玩家ID
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "无效的玩家ID",
			"data": nil,
		})
		return
	}

	// 查找玩家
	var player map[string]interface{}
	for _, p := range players {
		if p["id"] == id {
			player = p
			break
		}
	}

	if player == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "玩家不存在",
			"data": nil,
		})
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取成功",
		"data": player,
	})
}

// UpdatePlayerInfo 更新玩家信息
func UpdatePlayerInfo(c *gin.Context) {
	// 获取玩家ID
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "无效的玩家ID",
			"data": nil,
		})
		return
	}

	// 查找玩家
	var playerIndex int = -1
	for i, p := range players {
		if p["id"] == id {
			playerIndex = i
			break
		}
	}

	if playerIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "玩家不存在",
			"data": nil,
		})
		return
	}

	// 解析请求参数
	var updateReq map[string]interface{}
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "请求参数错误",
			"data": nil,
		})
		return
	}

	// 更新玩家信息
	for k, v := range updateReq {
		players[playerIndex][k] = v
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "更新成功",
		"data": players[playerIndex],
	})
}

// BanPlayer 封禁玩家
func BanPlayer(c *gin.Context) {
	// 获取玩家ID
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "无效的玩家ID",
			"data": nil,
		})
		return
	}

	// 查找玩家
	var playerIndex int = -1
	for i, p := range players {
		if p["id"] == id {
			playerIndex = i
			break
		}
	}

	if playerIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "玩家不存在",
			"data": nil,
		})
		return
	}

	// 封禁玩家
	players[playerIndex]["ban"] = true

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "封禁成功",
		"data": players[playerIndex],
	})
}

// UnbanPlayer 解封玩家
func UnbanPlayer(c *gin.Context) {
	// 获取玩家ID
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "无效的玩家ID",
			"data": nil,
		})
		return
	}

	// 查找玩家
	var playerIndex int = -1
	for i, p := range players {
		if p["id"] == id {
			playerIndex = i
			break
		}
	}

	if playerIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "玩家不存在",
			"data": nil,
		})
		return
	}

	// 解封玩家
	players[playerIndex]["ban"] = false

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "解封成功",
		"data": players[playerIndex],
	})
}