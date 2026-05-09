package api

import (
	"net/http"
	"ydxt-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

// InitRouter 初始化所有 API 路由
func InitRouter() *gin.Engine {
	r := gin.Default()

	// 全局中间件：跨域、恢复等
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 测试健康检查接口 (微信云托管非常需要这个来探测容器存活)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "pong",
			"data": "Yuedi Edu API is running...",
		})
	})

	// V1 版本接口路由组
	v1 := r.Group("/api/v1")
	{
		// === 公开接口 (免登录) ===
		public := v1.Group("")
		{
			// 微信小程序静默登录换取 Token
			public.POST("/auth/wx-login", WxLogin)
			// H5/App 手机号密码/验证码登录
			public.POST("/auth/phone-login", PhoneLogin)
			// 管理后台登录
			public.POST("/admin/login", AdminLogin)
			// 获取公开课程列表
			public.GET("/courses", GetCourseList)

			// 注册商城与曲谱 (公开)
			public.GET("/shop/goods", GetShopGoods)
			public.GET("/sheet-musics", GetSheetMusics)
		}

		// === 需要登录鉴权的接口 ===
		// 使用我们在 middleware 编写的 JWTAuth 中间件
		auth := v1.Group("")
		auth.Use(middleware.JWTAuth())
		{
			// 购物车
			auth.GET("/cart", GetCartItems)
			auth.POST("/cart/add", AddToCart)

			// 订单
			auth.GET("/orders", GetOrderList)

			// 打卡
			auth.POST("/practice/checkin", DoCheckin)

			// 社区
			auth.GET("/community/posts", GetPosts)

			// 用户模块
			userGroup := auth.Group("/users")
			{
				userGroup.GET("/profile", func(c *gin.Context) {
					userID, _ := c.Get("userID")
					role, _ := c.Get("role")
					c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{"user_id": userID, "role": role}})
				})
				// 修改密码
				userGroup.POST("/change-password", ChangePassword)
			}

			// 音视频直播模块
			rtcGroup := auth.Group("/rtc")
			{
				// 获取声网进房 Token
				rtcGroup.POST("/token", GenerateRTCToken)
				// 获取直播间状态
				rtcGroup.GET("/room/status", GetLiveRoomStatus)
				
				// 连麦相关 (学生/老师通用)
				rtcGroup.POST("/mic/apply", ApplyMic) // 学生举手
				rtcGroup.POST("/mic/end", EndMic)     // 结束连麦
			}

			// === 仅限教师操作的接口 ===
			teacherGroup := auth.Group("/teacher")
			teacherGroup.Use(middleware.RoleAuth(2, 9)) // 仅允许 role=2(讲师) 或 9(管理员) 访问
			{
				teacherGroup.GET("/my-classes", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "这里是教师专有的班级数据"})
				})
				
				// 教师开播、下播
				teacherGroup.POST("/live/start", StartLive)
				teacherGroup.POST("/live/end/:id", EndLive)
				
				// 教师审批连麦
				teacherGroup.GET("/mic/queue", GetMicQueue)   // 获取举手列表
				teacherGroup.POST("/mic/handle", HandleMic)   // 同意/拒绝
			}

			// === 仅限管理员操作的接口 ===
			adminGroup := auth.Group("/admin")
			adminGroup.Use(middleware.RoleAuth(9)) // 仅允许管理员访问
			{
				adminGroup.GET("/users", AdminGetUsers)
				
				// 课程管理相关接口
				adminGroup.GET("/courses", AdminGetCourses)
				adminGroup.POST("/courses", AdminCreateCourse)
				adminGroup.PUT("/courses/:id", AdminUpdateCourse)
				adminGroup.DELETE("/courses/:id", AdminDeleteCourse)
				adminGroup.PUT("/courses/:id/status", AdminUpdateCourseStatus)

				// 财务结算
				adminGroup.GET("/finance/settlements", GetSettlements)
			}
		}
	}

	return r
}
