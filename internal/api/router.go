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
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())

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
			public.GET("/admin/storage/preview", AdminPreviewCourseImage)
			// 教学平台公开内容数据
			public.GET("/courses", GetCourseList)
			public.GET("/courses/:id", GetCourseDetail)
			public.GET("/courses/:id/chapters", GetCourseChapters)
			public.GET("/chapters/:id/lessons", GetChapterLessons)
			public.GET("/shop/goods", GetShopGoods)
			public.GET("/shop/goods/:id", GetShopGoodsDetail)
			public.GET("/sheet-musics", GetSheetMusics)
			public.GET("/sheet-musics/:id", GetSheetMusicDetail)
		}

		// === 需要登录鉴权的接口 ===
		// 使用我们在 middleware 编写的 JWTAuth 中间件
		auth := v1.Group("")
		auth.Use(middleware.JWTAuth())
		{
			auth.GET("/courses/purchased", GetMyPurchasedCourses)

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
			}

			shopGroup := auth.Group("/shop")
			{
				shopGroup.GET("/cart", GetCartItems)
				shopGroup.POST("/cart", AddToCart)
				shopGroup.PATCH("/cart/:id", UpdateCartItem)
				shopGroup.DELETE("/cart/:id", DeleteCartItem)
				shopGroup.POST("/orders", CreateShopOrder)
				shopGroup.GET("/orders", GetMyOrders)
			}

			communityGroup := auth.Group("/community")
			{
				communityGroup.GET("/posts", GetCommunityPosts)
				communityGroup.GET("/posts/:id", GetCommunityPostDetail)
				communityGroup.POST("/posts", CreateCommunityPost)
				communityGroup.PUT("/posts/:id", UpdateCommunityPost)
				communityGroup.DELETE("/posts/:id", DeleteCommunityPost)
				communityGroup.GET("/posts/:id/comments", GetCommunityPostComments)
				communityGroup.POST("/posts/:id/comments", CreateCommunityPostComment)
				communityGroup.DELETE("/comments/:id", DeleteCommunityPostComment)
				communityGroup.POST("/posts/:id/like", LikeCommunityPost)
				communityGroup.DELETE("/posts/:id/like", UnlikeCommunityPost)
				communityGroup.POST("/posts/:id/favorite", FavoriteCommunityPost)
				communityGroup.DELETE("/posts/:id/favorite", UnfavoriteCommunityPost)
			}

			// === 仅限教师操作的接口 ===
			teacherGroup := auth.Group("/teacher")
			teacherGroup.Use(middleware.RoleAuth(2, 9)) // 仅允许 role=2(讲师) 或 9(管理员) 访问
			{
				teacherGroup.GET("/my-classes", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "这里是教师专有的班级数据"})
				})
			}

			// 管理后台业务接口
			adminGroup := auth.Group("/admin")
			adminGroup.Use(middleware.RoleAuth(2, 9)) // 管理员和讲师都可进入后台，数据范围由业务层再限制
			{
				adminGroup.GET("/dashboard/summary", GetDashboardSummary)

				adminGroup.GET("/users", AdminGetUsers)

				adminGroup.GET("/teachers", GetTeacherList)
				adminGroup.POST("/teachers", CreateTeacher)
				adminGroup.PUT("/teachers/:id", UpdateTeacher)

				adminGroup.GET("/admins", GetAdminList)
				adminGroup.POST("/admins", CreateAdmin)
				adminGroup.PUT("/admins/:id", UpdateAdmin)

				adminGroup.GET("/courses", AdminGetCourses)
				adminGroup.POST("/courses", CreateCourse)
				adminGroup.PUT("/courses/:id", UpdateCourse)
				adminGroup.PATCH("/courses/:id/status", UpdateCourseStatus)
				adminGroup.PATCH("/courses/:id/featured", UpdateCourseFeatured)
				adminGroup.DELETE("/courses/:id", DeleteCourse)
				adminGroup.POST("/courses/upload-image", AdminUploadCourseImage)
				adminGroup.GET("/courses/:id/chapters", GetCourseChapters)
				adminGroup.POST("/courses/:id/chapters", CreateCourseChapter)

				adminGroup.PUT("/chapters/:id", UpdateCourseChapter)
				adminGroup.DELETE("/chapters/:id", DeleteCourseChapter)
				adminGroup.GET("/chapters/:id/lessons", GetChapterLessons)
				adminGroup.POST("/chapters/:id/lessons", CreateLesson)

				adminGroup.PUT("/lessons/:id", UpdateLesson)
				adminGroup.DELETE("/lessons/:id", DeleteLesson)

				adminGroup.GET("/orders", GetOrderList)

				adminGroup.GET("/shop/goods", AdminGetShopGoods)
				adminGroup.POST("/shop/goods", AdminCreateShopGoods)
				adminGroup.PUT("/shop/goods/:id", AdminUpdateShopGoods)
				adminGroup.PATCH("/shop/goods/:id/status", AdminUpdateShopGoodsStatus)
				adminGroup.DELETE("/shop/goods/:id", AdminDeleteShopGoods)

				adminGroup.GET("/sheet-musics", AdminGetSheetMusics)
				adminGroup.GET("/sheet-musics/:id", AdminGetSheetMusicDetail)
				adminGroup.POST("/sheet-musics", AdminCreateSheetMusic)
				adminGroup.PUT("/sheet-musics/:id", AdminUpdateSheetMusic)
				adminGroup.PATCH("/sheet-musics/:id/recommended", AdminUpdateSheetMusicRecommended)
				adminGroup.DELETE("/sheet-musics/:id", AdminDeleteSheetMusic)

				adminGroup.GET("/learning/tasks", GetLearningTasks)
				adminGroup.POST("/learning/tasks", CreateLearningTask)
				adminGroup.PUT("/learning/tasks/:id", UpdateLearningTask)
				adminGroup.DELETE("/learning/tasks/:id", DeleteLearningTask)
				adminGroup.POST("/learning/tasks/:id/publish", PublishLearningTask)
				adminGroup.POST("/learning/tasks/:id/close", CloseLearningTask)
				adminGroup.GET("/learning/tasks/:id/questions", GetTaskQuestions)
				adminGroup.POST("/learning/tasks/:id/questions", CreateTaskQuestion)
				adminGroup.PUT("/learning/questions/:id", UpdateTaskQuestion)
				adminGroup.DELETE("/learning/questions/:id", DeleteTaskQuestion)
				adminGroup.GET("/learning/tasks/:id/submissions", GetTaskSubmissions)

				adminGroup.GET("/finance/settlements", GetSettlements)
				adminGroup.GET("/community/posts", AdminGetCommunityPosts)
				adminGroup.GET("/community/posts/:id", AdminGetCommunityPostDetail)
				adminGroup.POST("/community/posts/:id/approve", AdminApproveCommunityPost)
				adminGroup.POST("/community/posts/:id/reject", AdminRejectCommunityPost)
				adminGroup.PATCH("/community/posts/:id/flags", AdminUpdateCommunityPostFlags)
				adminGroup.DELETE("/community/posts/:id", AdminDeleteCommunityPost)
				adminGroup.GET("/community/comments", AdminGetCommunityComments)
				adminGroup.DELETE("/community/comments/:id", AdminDeleteCommunityComment)
				adminGroup.GET("/community/sensitive-words", AdminGetCommunitySensitiveWords)
				adminGroup.PUT("/community/sensitive-words", AdminUpdateCommunitySensitiveWords)
			}

			// 管理端复用的非 /admin 前缀接口
			adminCompatGroup := auth.Group("")
			adminCompatGroup.Use(middleware.RoleAuth(2, 9))
			{
				adminCompatGroup.GET("/practice/records", GetPracticeRecords)
			}
		}
	}

	return r
}
