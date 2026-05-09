package api

import (
	"net/http"
	"strconv"
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

type courseUpsertRequest struct {
	Title       string  `json:"title" binding:"required"`
	Cover       string  `json:"cover"`
	Price       float64 `json:"price"`
	TeacherID   uint64  `json:"teacher_id"`
	Category    string  `json:"category"`
	Level       string  `json:"level"`
	Desc        string  `json:"description"`
	StudentCount int    `json:"student_count"`
	Status      int8    `json:"status"`
}

type courseStatusRequest struct {
	Status int8 `json:"status" binding:"required,oneof=0 1"`
}

// CreateCourse 后台新增课程
func CreateCourse(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}

	var req courseUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	course := model.Course{
		Title:        req.Title,
		Cover:        req.Cover,
		Price:        req.Price,
		TeacherID:    req.TeacherID,
		Category:     req.Category,
		Level:        req.Level,
		Desc:         req.Desc,
		StudentCount: req.StudentCount,
		Status:       req.Status,
		IsDeleted:    0,
	}

	if err := model.DB.Create(&course).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "新增课程失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": course})
}

// UpdateCourse 后台更新课程
func UpdateCourse(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "课程ID无效"})
		return
	}

	var req courseUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	updates := map[string]interface{}{
		"title":         req.Title,
		"cover":         req.Cover,
		"price":         req.Price,
		"teacher_id":    req.TeacherID,
		"category":      req.Category,
		"level":         req.Level,
		"description":   req.Desc,
		"student_count": req.StudentCount,
		"status":        req.Status,
	}
	result := model.DB.Model(&model.Course{}).Where("id = ? AND is_deleted = ?", id, 0).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新课程失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课程不存在"})
		return
	}

	var course model.Course
	_ = model.DB.Where("id = ?", id).First(&course).Error
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": course})
}

// UpdateCourseStatus 后台修改课程状态（上架/下架）
func UpdateCourseStatus(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "课程ID无效"})
		return
	}

	var req courseStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	result := model.DB.Model(&model.Course{}).Where("id = ? AND is_deleted = ?", id, 0).Update("status", req.Status)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新课程状态失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课程不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// DeleteCourse 后台删除课程（软删除）
func DeleteCourse(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "课程ID无效"})
		return
	}

	result := model.DB.Model(&model.Course{}).Where("id = ? AND is_deleted = ?", id, 0).Update("is_deleted", 1)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除课程失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课程不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}
