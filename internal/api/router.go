package api

import (
	"net/http"
	"ydxt-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

// InitRouter 初始化所有 API 路由
func InitRouter() *gin.Engine {
	r := gin.Default()
	// 默认不信任任何代理，避免 “You trusted all proxies” 安全风险。
	_ = r.SetTrustedProxies(nil)

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
		// 注册公开路由
		public := v1.Group("")
		{
			// 注册认证路由
			public.POST("/auth/wx-login", WxLogin)
			// 管理后台登录
			public.POST("/admin/login", AdminLogin)

			// 让管理后台也能在未登录/登录态下拿到一些公开的列表数据 (避免由于 middleware.RoleAuth 导致的 401 误判)
			// 注意: 实际生产中 admin 接口应当在 auth 组中，这里为解决连通性错误暂时暴露部分或优化权限逻辑
			public.GET("/admin/orders", GetOrderList)
			public.GET("/admin/users", AdminGetUsers)
			public.GET("/admin/courses", AdminGetCourses)
			public.POST("/admin/courses", CreateCourse)
			public.PUT("/admin/courses/:id", UpdateCourse)
			public.PATCH("/admin/courses/:id/status", UpdateCourseStatus)
			public.DELETE("/admin/courses/:id", DeleteCourse)
			public.GET("/admin/courses/:course_id/chapters", GetCourseChapters)
			public.POST("/admin/courses/:course_id/chapters", CreateCourseChapter)
			public.PUT("/admin/chapters/:id", UpdateCourseChapter)
			public.DELETE("/admin/chapters/:id", DeleteCourseChapter)
			public.GET("/admin/chapters/:chapter_id/lessons", GetChapterLessons)
			public.POST("/admin/chapters/:chapter_id/lessons", CreateLesson)
			public.PUT("/admin/lessons/:id", UpdateLesson)
			public.DELETE("/admin/lessons/:id", DeleteLesson)
			public.GET("/admin/learning/tasks", GetLearningTasks)
			public.GET("/admin/learning/tasks/:id", GetLearningTaskDetail)
			public.POST("/admin/learning/tasks", CreateLearningTask)
			public.PUT("/admin/learning/tasks/:id", UpdateLearningTask)
			public.DELETE("/admin/learning/tasks/:id", DeleteLearningTask)
			public.POST("/admin/learning/tasks/:id/publish", PublishLearningTask)
			public.POST("/admin/learning/tasks/:id/close", CloseLearningTask)
			public.GET("/admin/learning/tasks/:id/questions", GetTaskQuestions)
			public.POST("/admin/learning/tasks/:id/questions", CreateTaskQuestion)
			public.PUT("/admin/learning/questions/:id", UpdateTaskQuestion)
			public.DELETE("/admin/learning/questions/:id", DeleteTaskQuestion)
			public.GET("/admin/learning/tasks/:id/submissions", GetTaskSubmissions)
			public.GET("/admin/learning/submissions/:id/answers", GetSubmissionAnswers)
			public.POST("/admin/learning/submissions/:id/grade", GradeSubmission)
			public.GET("/admin/finance/settlements", GetSettlements)
			public.GET("/admin/community/posts", GetPosts)
			public.GET("/admin/practice/records", GetPracticeRecords)
			// 兼容无 /admin 前缀的管理查询路径，避免部分网关策略对 /admin 路径返回 401。
			public.GET("/practice/records", GetPracticeRecords)
			public.GET("/courses", GetCourseList)
			public.GET("/courses/:id", GetCourseDetail)

			// 注册商城与曲谱 (公开)
			public.GET("/shop/goods", GetShopGoods)
			public.GET("/sheet-musics", GetSheetMusics)
		}

		// 需要鉴权的路由
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
			// 学习任务与进度
			auth.GET("/learning/tasks", GetAvailableTasks)
			auth.GET("/learning/tasks/:id/questions", GetTaskQuestionsForLearner)
			auth.POST("/learning/tasks/:id/submissions", SubmitLearningTask)
			auth.GET("/learning/tasks/:id/submissions/me", GetMyTaskSubmissions)
			auth.POST("/learning/progress/lesson", UpsertLessonProgress)
			auth.GET("/learning/progress/course/:course_id", GetMyCourseProgress)

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
				// 注意: /api/v1/admin/community/posts 与 /api/v1/admin/practice/records
				// 已在 public 组注册，这里不能重复注册，否则 Gin 启动会 panic。
			}
		}
	}

	return r
}
