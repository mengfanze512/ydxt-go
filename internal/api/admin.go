package api

import (
	"net/http"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

// AdminGetUsers 获取用户列表
func AdminGetUsers(c *gin.Context) {
	var users []model.User
	// 排除管理员自身，或者展示所有
	if err := model.DB.Order("created_at desc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取用户列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": users,
	})
}

// AdminGetCourses 获取全部课程（后台）
func AdminGetCourses(c *gin.Context) {
	GetCourseList(c)
}
