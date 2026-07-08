package router

import (
	"github.com/gin-gonic/gin"
	"github.com/bigfish/gm_backend/controller"
)

// SetupRoutes 配置路由
func SetupRoutes(r *gin.Engine) {
	// 首页路由
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "GM后台系统欢迎您",
			"data": gin.H{
				"version": "1.0.0",
				"api_documentation": "访问 /api 路径使用相关功能",
				"login_endpoint": "/api/auth/login",
				"admin_endpoint": "/api/admin",
			},
		})
	})

	// 基础路由组
	api := r.Group("/api")
	{
		// 登录相关路由
		auth := api.Group("/auth")
		{
			auth.POST("/login", controller.Login)
			auth.POST("/logout", controller.Logout)
		}

		// 需要认证的路由组
		admin := api.Group("/admin")
		admin.Use(controller.AuthMiddleware())
		{
			// 玩家管理
			player := admin.Group("/player")
			{
				player.GET("/list", controller.GetPlayerList)
				player.GET("/info/:id", controller.GetPlayerInfo)
				player.PUT("/info/:id", controller.UpdatePlayerInfo)
				player.POST("/ban/:id", controller.BanPlayer)
				player.POST("/unban/:id", controller.UnbanPlayer)
			}

			// 游戏数据
			data := admin.Group("/data")
			{
				data.GET("/statistics", controller.GetGameStatistics)
				data.GET("/recharge", controller.GetRechargeRecord)
				data.GET("/online", controller.GetOnlinePlayers)
			}

			// 系统设置
			system := admin.Group("/system")
			{
				system.GET("/config", controller.GetSystemConfig)
				system.PUT("/config", controller.UpdateSystemConfig)
				system.GET("/logs", controller.GetSystemLogs)
			}
		}
	}
}