package controller

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

// 模拟系统配置
var systemConfig = map[string]interface{}{
	"server_name": "GM后台服务器",
	"maintenance_mode": false,
	"maintenance_time": "",
	"max_players": 50000,
	"rate_limit": 100,
	"log_level": "info",
}

// 模拟系统日志
var systemLogs = []map[string]interface{}{
	{
		"id": 1,
		"time": time.Now().Add(-1 * time.Hour).Format("2006-01-02 15:04:05"),
		"level": "info",
		"message": "服务器启动成功",
		"operator": "system",
	},
	{
		"id": 2,
		"time": time.Now().Add(-45 * time.Minute).Format("2006-01-02 15:04:05"),
		"level": "warning",
		"message": "CPU使用率过高: 85%",
		"operator": "system",
	},
	{
		"id": 3,
		"time": time.Now().Add(-30 * time.Minute).Format("2006-01-02 15:04:05"),
		"level": "info",
		"message": "管理员登录成功",
		"operator": "admin",
	},
}

// GetSystemConfig 获取系统配置
func GetSystemConfig(c *gin.Context) {
	// 返回系统配置
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取成功",
		"data": systemConfig,
	})
}

// UpdateSystemConfig 更新系统配置
func UpdateSystemConfig(c *gin.Context) {
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

	// 更新系统配置
	for k, v := range updateReq {
		systemConfig[k] = v
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "更新成功",
		"data": systemConfig,
	})
}

// GetSystemLogs 获取系统日志
func GetSystemLogs(c *gin.Context) {
	// 获取查询参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "10")
	level := c.Query("level")

	// 转换分页参数
	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	// 过滤日志
	filteredLogs := systemLogs
	if level != "" {
		var temp []map[string]interface{}
		for _, log := range systemLogs {
			if log["level"] == level {
				temp = append(temp, log)
			}
		}
		filteredLogs = temp
	}

	// 分页处理
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(filteredLogs) {
		start = len(filteredLogs)
	}
	if end > len(filteredLogs) {
		end = len(filteredLogs)
	}
	pagedLogs := filteredLogs[start:end]

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取成功",
		"data": gin.H{
			"total": len(filteredLogs),
			"list": pagedLogs,
		},
	})
}