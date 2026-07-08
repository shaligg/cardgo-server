# 游戏公司GM后台系统

基于Golang Gin框架开发的游戏公司GM后台系统，提供玩家管理、数据统计、系统设置等功能。

## 项目结构
```
gm_backend/
├── main.go         # 项目入口文件
├── go.mod          # Go模块文件
├── go.sum          # 依赖包列表
├── router/
│   └── router.go   # 路由配置
├── controller/
│   ├── auth_controller.go    # 认证控制器
│   ├── player_controller.go  # 玩家管理控制器
│   ├── data_controller.go    # 数据统计控制器
│   └── system_controller.go  # 系统设置控制器
└── README.md       # 项目说明
```

## 功能模块
1. **认证模块**：登录、注销、JWT认证中间件
2. **玩家管理**：玩家列表、玩家详情、修改玩家信息、封禁/解封玩家
3. **数据统计**：游戏统计数据、充值记录、在线玩家
4. **系统设置**：系统配置、系统日志

## 使用说明
### 环境要求
- Go 1.16+ 
- Gin 1.10.1+
- Golang-jwt v4+

### 安装依赖
```bash
cd gm_backend
go mod tidy
```

### 运行项目
```bash
# 开发环境运行
go run main.go

# 编译运行
go build -o gm_backend
./gm_backend
```

### API接口文档
服务启动后，访问 http://localhost:8080/api 可以调用相关接口。

#### 认证接口
- POST /api/auth/login - 登录
- POST /api/auth/logout - 注销

#### 玩家管理接口
- GET /api/admin/player/list - 获取玩家列表
- GET /api/admin/player/info/:id - 获取玩家详情
- PUT /api/admin/player/info/:id - 更新玩家信息
- POST /api/admin/player/ban/:id - 封禁玩家
- POST /api/admin/player/unban/:id - 解封玩家

#### 数据统计接口
- GET /api/admin/data/statistics - 获取游戏统计数据
- GET /api/admin/data/recharge - 获取充值记录
- GET /api/admin/data/online - 获取在线玩家

#### 系统设置接口
- GET /api/admin/system/config - 获取系统配置
- PUT /api/admin/system/config - 更新系统配置
- GET /api/admin/system/logs - 获取系统日志

## 注意事项
1. 本项目为示例代码，实际应用中需要连接真实数据库
2. 生产环境中请修改JWT密钥，并使用HTTPS
3. 请根据实际需求扩展功能和优化代码

## 版本历史
- v1.0.0: 基础功能实现