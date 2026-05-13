package api

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

func currentTeacherScope(c *gin.Context) (bool, uint64) {
	accountType, _ := c.Get("accountType")
	if accountType == accountTypeTeacher {
		if teacherID, ok := c.Get("teacherID"); ok {
			if id, okCast := teacherID.(uint64); okCast && id > 0 {
				return true, id
			}
		}
	}
	return false, 0
}

func fillTeacherNames(courses []model.Course) {
	for i, course := range courses {
		var teacher model.Teacher
		if err := model.DB.Where("id = ?", course.TeacherID).First(&teacher).Error; err == nil {
			courses[i].TeacherName = teacher.Name
		} else {
			courses[i].TeacherName = "讲师"
		}
	}
}

func ensureCoursePermission(c *gin.Context, courseID uint64) (*model.Course, bool) {
	var course model.Course
	if err := model.DB.Where("id = ?", courseID).First(&course).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课程不存在"})
		return nil, false
	}
	if isTeacher, teacherID := currentTeacherScope(c); isTeacher && course.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "只能操作自己的课程"})
		return nil, false
	}
	return &course, true
}

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

	normalizeCoursePrices(courses)

	// 为前台组装讲师名字(简单处理，实际应联表查询)
	fillTeacherNames(courses)

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

	normalizeCoursePrice(&course)

	var teacher model.Teacher
	if err := model.DB.Where("id = ?", course.TeacherID).First(&teacher).Error; err == nil {
		course.TeacherName = teacher.Name
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": course})
}

type courseUpsertRequest struct {
	Title          string  `json:"title" binding:"required"`
	Cover          string  `json:"cover"`
	Price          float64 `json:"price"`
	TeacherID      uint64  `json:"teacher_id"`
	Category       string  `json:"category"`
	Difficulty     int     `json:"difficulty"`
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
	if isTeacher, teacherID := currentTeacherScope(c); isTeacher {
		req.TeacherID = teacherID
	}
	if err := validateCourseTeacher(req.TeacherID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
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
	if hasColumn(columnTypes, "difficulty") && req.Difficulty > 0 {
		insertData["difficulty"] = convertDifficultyValue(req.Difficulty, columnTypes["difficulty"])
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "新增课程失败: " + err.Error()})
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

	if _, ok := ensureCoursePermission(c, id); !ok {
		return
	}

	var req courseUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if isTeacher, teacherID := currentTeacherScope(c); isTeacher {
		req.TeacherID = teacherID
	}
	if err := validateCourseTeacher(req.TeacherID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
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
	if hasColumn(columnTypes, "difficulty") && req.Difficulty > 0 {
		updates["difficulty"] = convertDifficultyValue(req.Difficulty, columnTypes["difficulty"])
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新课程失败: " + result.Error.Error()})
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
	if _, ok := ensureCoursePermission(c, id); !ok {
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

	if _, ok := ensureCoursePermission(c, id); !ok {
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

func validateCourseTeacher(teacherID uint64) error {
	if teacherID == 0 {
		return fmt.Errorf("请选择讲师")
	}
	var count int64
	if err := model.DB.Model(&model.Teacher{}).Where("id = ?", teacherID).Count(&count).Error; err != nil {
		return fmt.Errorf("校验讲师失败: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("所选讲师不存在")
	}
	return nil
}

func normalizeCoursePrices(courses []model.Course) {
	priceColumnType := getCoursePriceColumnType()
	for i := range courses {
		courses[i].Price = normalizeCoursePriceValue(courses[i].Price, priceColumnType)
	}
}

func normalizeCoursePrice(course *model.Course) {
	if course == nil {
		return
	}
	course.Price = normalizeCoursePriceValue(course.Price, getCoursePriceColumnType())
}

func getCoursePriceColumnType() string {
	return getCourseColumnTypes()["price"]
}

func normalizeCoursePriceValue(price float64, columnType string) float64 {
	if strings.Contains(columnType, "int") {
		return price / 100
	}
	return price
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
		case "笛子":
			return 1
		case "箫":
			return 2
		case "葫芦丝":
			return 3
		case "埙":
			return 4
		case "古筝":
			return 5
		case "其他":
			return 6
		default:
			if n, err := strconv.Atoi(category); err == nil {
				return n
			}
			return 0
		}
	}
	return category
}

func convertDifficultyValue(difficulty int, columnType string) interface{} {
	if strings.Contains(columnType, "int") {
		return difficulty
	}
	switch difficulty {
	case 1:
		return "初学者"
	case 2:
		return "有基础"
	case 3:
		return "进阶提升"
	case 4:
		return "考级提升"
	default:
		return ""
	}
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
