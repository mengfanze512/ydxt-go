package api

import (
	"net/http"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

// GetCourseList 获取公开课程列表 (支持前台分页和分类)
func GetCourseList(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.Course{}})
		return
	}
	category := c.Query("category") // 例如: "入门", "进阶", "考级"
	level := c.Query("level")       // 例如: "system", "1v1", "vip"

	var courses []model.Course
	query := model.DB.Where("status = ?", 1).Where("is_deleted = ?", 0)

	if category != "" && category != "全部" {
		query = query.Where("category = ?", category)
	}
	if level != "" {
		query = query.Where("level = ?", level)
	}

	if err := query.Order("created_at desc").Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取课程列表失败"})
		return
	}

	// 为前台组装讲师名字(简单处理，实际应联表查询)
	for i, course := range courses {
		var teacher model.User
		if err := model.DB.Where("id = ?", course.TeacherID).First(&teacher).Error; err == nil {
			courses[i].TeacherName = teacher.Nickname
		} else {
			courses[i].TeacherName = "特邀讲师"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": courses,
	})
}

// GetCourseDetail 获取单个课程详情
func GetCourseDetail(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}

	id := c.Param("id")
	var course model.Course
	if err := model.DB.Where("id = ? AND is_deleted = ?", id, 0).First(&course).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课程不存在", "data": nil})
		return
	}

	var teacher model.User
	if err := model.DB.Where("id = ?", course.TeacherID).First(&teacher).Error; err == nil {
		course.TeacherName = teacher.Nickname
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": course})
}
