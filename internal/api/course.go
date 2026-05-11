package api

import (
	"math"
	"net/http"
	"strconv"
	"strings"
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
	query := model.DB.Where("status = ?", 1)

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
	if err := model.DB.Where("id = ?", id).First(&course).Error; err != nil {
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
	Title          string  `json:"title" binding:"required"`
	Cover          string  `json:"cover"`
	Price          float64 `json:"price"`
	TeacherID      uint64  `json:"teacher_id"`
	Category       string  `json:"category"`
	Level          string  `json:"level"`
	Desc           string  `json:"description"`
	DetailContent  string  `json:"detail_content"`
	CarouselImages string  `json:"carousel_images"`
	StudentCount   int     `json:"student_count"`
	Status         int8    `json:"status"`
}

type courseStatusRequest struct {
	Status int8 `json:"status" binding:"oneof=0 1"`
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

	columnTypes := getCourseColumnTypes()
	insertData := map[string]interface{}{}
	insertData["title"] = req.Title

	if hasColumn(columnTypes, "cover") {
		insertData["cover"] = req.Cover
	}
	if hasColumn(columnTypes, "cover_url") {
		insertData["cover_url"] = req.Cover
	}
	if hasColumn(columnTypes, "teacher_id") {
		insertData["teacher_id"] = req.TeacherID
	}
	if hasColumn(columnTypes, "category") {
		insertData["category"] = convertCategoryValue(req.Category, columnTypes["category"])
	}
	if hasColumn(columnTypes, "level") {
		insertData["level"] = req.Level
	}
	if hasColumn(columnTypes, "type") {
		insertData["type"] = convertLevelToType(req.Level)
	}
	if hasColumn(columnTypes, "description") {
		insertData["description"] = req.Desc
	}
	if hasColumn(columnTypes, "detail_content") {
		insertData["detail_content"] = req.DetailContent
	}
	if hasColumn(columnTypes, "carousel_images") {
		insertData["carousel_images"] = req.CarouselImages
	}
	if hasColumn(columnTypes, "student_count") {
		insertData["student_count"] = req.StudentCount
	}
	if hasColumn(columnTypes, "sales_count") {
		insertData["sales_count"] = req.StudentCount
	}
	if hasColumn(columnTypes, "price") {
		insertData["price"] = convertPriceValue(req.Price, columnTypes["price"])
	}
	if hasColumn(columnTypes, "status") {
		insertData["status"] = req.Status
	}
	if err := model.DB.Table("courses").Create(insertData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "新增课程失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": insertData})
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

	columnTypes := getCourseColumnTypes()
	updates := map[string]interface{}{}
	updates["title"] = req.Title
	if hasColumn(columnTypes, "cover") {
		updates["cover"] = req.Cover
	}
	if hasColumn(columnTypes, "cover_url") {
		updates["cover_url"] = req.Cover
	}
	if hasColumn(columnTypes, "teacher_id") {
		updates["teacher_id"] = req.TeacherID
	}
	if hasColumn(columnTypes, "category") {
		updates["category"] = convertCategoryValue(req.Category, columnTypes["category"])
	}
	if hasColumn(columnTypes, "level") {
		updates["level"] = req.Level
	}
	if hasColumn(columnTypes, "type") {
		updates["type"] = convertLevelToType(req.Level)
	}
	if hasColumn(columnTypes, "description") {
		updates["description"] = req.Desc
	}
	if hasColumn(columnTypes, "detail_content") {
		updates["detail_content"] = req.DetailContent
	}
	if hasColumn(columnTypes, "carousel_images") {
		updates["carousel_images"] = req.CarouselImages
	}
	if hasColumn(columnTypes, "student_count") {
		updates["student_count"] = req.StudentCount
	}
	if hasColumn(columnTypes, "sales_count") {
		updates["sales_count"] = req.StudentCount
	}
	if hasColumn(columnTypes, "price") {
		updates["price"] = convertPriceValue(req.Price, columnTypes["price"])
	}
	if hasColumn(columnTypes, "status") {
		updates["status"] = req.Status
	}

	result := model.DB.Table("courses").Where("id = ?", id).Updates(updates)
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

	result := model.DB.Model(&model.Course{}).Where("id = ?", id).Update("status", req.Status)
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

// DeleteCourse 后台删除课程（物理删除）
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

	result := model.DB.Table("courses").Where("id = ?", id).Delete(nil)
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

func getCourseColumnTypes() map[string]string {
	types := map[string]string{}
	rows, err := model.DB.Raw("SHOW COLUMNS FROM courses").Rows()
	if err != nil {
		return types
	}
	defer rows.Close()

	for rows.Next() {
		var field, colType, nullable, key, extra string
		var defaultValue interface{}
		if err := rows.Scan(&field, &colType, &nullable, &key, &defaultValue, &extra); err == nil {
			types[field] = strings.ToLower(colType)
		}
	}
	return types
}

func hasColumn(columnTypes map[string]string, name string) bool {
	_, ok := columnTypes[name]
	return ok
}

func convertPriceValue(price float64, columnType string) interface{} {
	if strings.Contains(columnType, "int") {
		return int(math.Round(price * 100))
	}
	return price
}

func convertCategoryValue(category string, columnType string) interface{} {
	if strings.Contains(columnType, "int") {
		switch category {
		case "入门":
			return 1
		case "进阶":
			return 2
		case "考级":
			return 3
		default:
			if n, err := strconv.Atoi(category); err == nil {
				return n
			}
			return 0
		}
	}
	return category
}

func convertLevelToType(level string) int {
	switch level {
	case "1v1":
		return 2
	case "vip":
		return 3
	default:
		return 1
	}
}
