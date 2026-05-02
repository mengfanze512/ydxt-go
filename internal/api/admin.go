package api

import (
	"net/http"
	"strconv"
	"yuedi_edu/internal/model"

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

// AdminGetCourses 获取课程列表(包含下架的)
func AdminGetCourses(c *gin.Context) {
	var courses []model.Course
	if err := model.DB.Where("is_deleted = ?", 0).Order("created_at desc").Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取课程列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": courses,
	})
}

// AdminCreateCourse 新增课程
func AdminCreateCourse(c *gin.Context) {
	var req model.Course
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	if err := model.DB.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "新增课程失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "新增课程成功", "data": req})
}

// AdminUpdateCourse 编辑课程
func AdminUpdateCourse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "课程ID无效"})
		return
	}

	var req model.Course
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var course model.Course
	if err := model.DB.First(&course, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课程不存在"})
		return
	}

	// 允许更新的字段
	updates := map[string]interface{}{
		"title":       req.Title,
		"cover_url":   req.Cover,
		"price":       req.Price,
		"teacher_id":  req.TeacherID,
		"category":    req.Category,
		"difficulty":  req.Difficulty,
		"type":        req.Type,
		"status":      req.Status,
	}

	if err := model.DB.Model(&course).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新课程失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "更新课程成功"})
}

// AdminDeleteCourse 删除课程 (软删除)
func AdminDeleteCourse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "课程ID无效"})
		return
	}

	var course model.Course
	if err := model.DB.First(&course, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课程不存在"})
		return
	}

	if err := model.DB.Model(&course).Update("is_deleted", 1).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除课程失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "删除课程成功"})
}

// AdminUpdateCourseStatus 更新课程上下架状态
func AdminUpdateCourseStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "课程ID无效"})
		return
	}

	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var course model.Course
	if err := model.DB.First(&course, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课程不存在"})
		return
	}

	if err := model.DB.Model(&course).Update("status", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新课程状态失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "更新状态成功"})
}
