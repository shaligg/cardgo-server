package controller

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// 定义JWT密钥
var jwtSecret = []byte("your-secret-key")

// Login 处理登录请求
func Login(c *gin.Context) {
	var loginReq struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// 绑定请求参数
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "请求参数错误",
			"data": nil,
		})
		return
	}

	// 验证用户名和密码 (实际应用中应该从数据库验证)
	if loginReq.Username != "admin" || loginReq.Password != "admin123" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "用户名或密码错误",
			"data": nil,
		})
		return
	}

	// 生成JWT令牌
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
		Subject:   loginReq.Username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "生成令牌失败",
			"data": nil,
		})
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "登录成功",
		"data": gin.H{
			"token": tokenString,
		},
	})
}

// Logout 处理注销请求
func Logout(c *gin.Context) {
	// 实际应用中可能需要将token加入黑名单
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "注销成功",
		"data": nil,
	})
}

// AuthMiddleware JWT认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "未提供认证token",
				"data": nil,
			})
			c.Abort()
			return
		}

		// 解析token
		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(authHeader, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "无效的token",
				"data": nil,
			})
			c.Abort()
			return
		}

		// 将用户名存入上下文
		c.Set("username", claims.Subject)
		c.Next()
	}
}