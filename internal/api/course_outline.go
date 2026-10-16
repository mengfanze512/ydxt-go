package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
	"ydxt-go/internal/model"

	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type chapterRequest struct {
	Title     string `json:"title" binding:"required"`
	SortOrder int    `json:"sort_order"`
}

type lessonRequest struct {
	CohortID      uint64 `json:"cohort_id"`
	Title         string `json:"title" binding:"required"`
	Type          int8   `json:"type"` // 1=视频,2=图文
	MediaURL      string `json:"media_url"`
	MediaFileID   string `json:"media_file_id"`
	DetailContent string `json:"detail_content"`
	ExpireAt      string `json:"expire_at"` // YYYY-MM-DD HH:mm:ss
	Duration      int    `json:"duration"`
	IsFree        int8   `json:"is_free"`
	SortOrder     int    `json:"sort_order"`
}

func parseUint(c *gin.Context, keys ...string) (uint64, bool) {
	for _, key := range keys {
		raw := c.Param(key)
		if raw == "" {
			continue
		}
		v, err := strconv.ParseUint(raw, 10, 64)
		if err == nil {
			return v, true
		}
	}
	c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
	return 0, false
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	location, _ := time.LoadLocation("Asia/Shanghai")
	if location == nil {
		location = time.FixedZone("CST", 8*3600)
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, location)
	if err != nil {
		return nil
	}
	return &t
}

func normalizeLessonRequest(req *lessonRequest) {
	if req.Type == 0 {
		req.Type = 1
	}
	if req.IsFree != 1 {
		req.IsFree = 0
	}
	switch req.Type {
	case 1:
		req.DetailContent = ""
	case 2:
		req.MediaURL = ""
		req.MediaFileID = ""
	}
}

func validateDeliveryLesson(courseID uint64, req lessonRequest) error {
	if req.Type != 1 && req.Type != 2 {
		return errors.New("课时仅支持视频或图文，请重新配置课时内容")
	}
	var course model.Course
	if err := model.DB.First(&course, courseID).Error; err != nil {
		return err
	}
	if req.ExpireAt != "" && parseTime(req.ExpireAt) == nil {
		return errors.New("失效时间格式错误")
	}
	if course.Level == "class" {
		var cohort model.CourseCohort
		if err := model.DB.First(&cohort, "id = ? AND course_id = ?", req.CohortID, courseID).Error; err != nil {
			return errors.New("请先选择该课程的班期")
		}
	} else if req.CohortID != 0 {
		return errors.New("非班课不能关联班期")
	}
	return nil
}

func redactDeliveryLessons(c *gin.Context, courseID uint64, lessons []model.CourseLesson) {
	if deliveryStaff(c, courseID) {
		return
	}
	for i := range lessons {
		lessons[i].MediaURL = ""
		lessons[i].MediaFileID = ""
		lessons[i].DetailContent = ""
	}
}

// GetCourseChapters 获取课程章节
func GetCourseChapters(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.CourseChapter{}})
		return
	}
	courseID, ok := parseUint(c, "id", "course_id")
	if !ok {
		return
	}
	if _, ok := ensureCoursePermission(c, courseID); !ok {
		return
	}
	var chapters []model.CourseChapter
	if err := model.DB.Where("course_id = ?", courseID).Order("sort_order asc, id asc").Find(&chapters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取章节失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": chapters})
}

// CreateCourseChapter 新增章节
func CreateCourseChapter(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	courseID, ok := parseUint(c, "id", "course_id")
	if !ok {
		return
	}
	course, ok := ensureCoursePermission(c, courseID)
	if !ok {
		return
	}
	if course.Level == "class" || course.Level == "single" {
		deliveryReply(c, nil, fmt.Errorf("班课和单课不支持新增章节，请使用对应的授课管理入口"))
		return
	}
	var req chapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	chapter := model.CourseChapter{
		CourseID:  courseID,
		Title:     req.Title,
		SortOrder: req.SortOrder,
	}
	if err := model.DB.Create(&chapter).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "新增章节失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": chapter})
}

// UpdateCourseChapter 编辑章节
func UpdateCourseChapter(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	chapterID, ok := parseUint(c, "id")
	if !ok {
		return
	}
	var chapter model.CourseChapter
	if err := model.DB.Where("id = ?", chapterID).First(&chapter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "章节不存在"})
		return
	}
	if _, ok := ensureCoursePermission(c, chapter.CourseID); !ok {
		return
	}
	var req chapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	result := model.DB.Model(&model.CourseChapter{}).
		Where("id = ?", chapterID).
		Updates(map[string]interface{}{"title": req.Title, "sort_order": req.SortOrder})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新章节失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "章节不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// DeleteCourseChapter 删除章节（物理删除，同时删除其下小节）
func DeleteCourseChapter(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	chapterID, ok := parseUint(c, "id")
	if !ok {
		return
	}
	var chapter model.CourseChapter
	if err := model.DB.Where("id = ?", chapterID).First(&chapter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "章节不存在"})
		return
	}
	if _, ok := ensureCoursePermission(c, chapter.CourseID); !ok {
		return
	}
	err := deleteDeliveryContent(chapter.CourseID, func(tx *gorm.DB) error {
		if err := tx.Where("chapter_id = ?", chapterID).Delete(&model.CourseLesson{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", chapterID).Delete(&model.CourseChapter{}).Error
	})
	if err != nil {
		deliveryReply(c, nil, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// GetChapterLessons 获取章节小节
func GetChapterLessons(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.CourseLesson{}})
		return
	}
	chapterID, ok := parseUint(c, "id", "chapter_id")
	if !ok {
		return
	}
	var chapter model.CourseChapter
	if err := model.DB.Where("id = ?", chapterID).First(&chapter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "章节不存在"})
		return
	}
	if _, ok := ensureCoursePermission(c, chapter.CourseID); !ok {
		return
	}
	var lessons []model.CourseLesson
	query := model.DB.Where("chapter_id = ?", chapterID)
	if !deliveryStaff(c, chapter.CourseID) {
		query = query.Where("type IN (1, 2)")
	}
	if err := query.Order("sort_order asc, id asc").Find(&lessons).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取小节失败"})
		return
	}
	redactDeliveryLessons(c, chapter.CourseID, lessons)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": lessons})
}

// CreateLesson 新增小节
func CreateLesson(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	chapterID, ok := parseUint(c, "id", "chapter_id")
	if !ok {
		return
	}
	var chapter model.CourseChapter
	if err := model.DB.Where("id = ?", chapterID).First(&chapter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "章节不存在"})
		return
	}
	if _, ok := ensureCoursePermission(c, chapter.CourseID); !ok {
		return
	}
	var req lessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	normalizeLessonRequest(&req)
	if err := validateDeliveryLesson(chapter.CourseID, req); err != nil {
		deliveryReply(c, nil, err)
		return
	}
	lesson := model.CourseLesson{
		CohortID:      req.CohortID,
		ChapterID:     chapterID,
		Title:         req.Title,
		Type:          req.Type,
		MediaURL:      req.MediaURL,
		MediaFileID:   req.MediaFileID,
		DetailContent: req.DetailContent,
		ExpireAt:      parseTime(req.ExpireAt),
		Duration:      req.Duration,
		IsFree:        req.IsFree,
		SortOrder:     req.SortOrder,
	}
	if lesson.SortOrder <= 0 {
		var maxSortOrder int
		model.DB.Model(&model.CourseLesson{}).
			Where("chapter_id = ?", chapterID).
			Select("COALESCE(MAX(sort_order), 0)").
			Scan(&maxSortOrder)
		lesson.SortOrder = maxSortOrder + 1
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		var course model.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&course, chapter.CourseID).Error; err != nil {
			return err
		}
		if course.Level == "single" {
			var count int64
			if err := tx.Table("course_lessons l").Joins("JOIN course_chapters ch ON ch.id = l.chapter_id").Where("ch.course_id = ?", course.ID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("单课仅允许一个课时")
			}
		}
		return tx.Create(&lesson).Error
	}); err != nil {
		deliveryReply(c, nil, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": lesson})
}

// UpdateLesson 编辑小节
func UpdateLesson(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	lessonID, ok := parseUint(c, "id")
	if !ok {
		return
	}
	var lesson model.CourseLesson
	if err := model.DB.Where("id = ?", lessonID).First(&lesson).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "小节不存在"})
		return
	}
	var chapter model.CourseChapter
	if err := model.DB.Where("id = ?", lesson.ChapterID).First(&chapter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "章节不存在"})
		return
	}
	if _, ok := ensureCoursePermission(c, chapter.CourseID); !ok {
		return
	}
	var req lessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	normalizeLessonRequest(&req)
	if err := validateDeliveryLesson(chapter.CourseID, req); err != nil {
		deliveryReply(c, nil, err)
		return
	}
	if lesson.CohortID != req.CohortID {
		deliveryReply(c, nil, errors.New("已有课时不能转移班期"))
		return
	}
	updates := map[string]interface{}{
		"cohort_id":      req.CohortID,
		"title":          req.Title,
		"type":           req.Type,
		"media_url":      req.MediaURL,
		"media_file_id":  req.MediaFileID,
		"detail_content": req.DetailContent,
		"expire_at":      parseTime(req.ExpireAt),
		"duration":       req.Duration,
		"is_free":        req.IsFree,
	}
	if req.SortOrder > 0 {
		updates["sort_order"] = req.SortOrder
	}
	result := model.DB.Model(&model.CourseLesson{}).Where("id = ?", lessonID).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新小节失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "小节不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// DeleteLesson 删除小节（物理删除）
func DeleteLesson(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	lessonID, ok := parseUint(c, "id")
	if !ok {
		return
	}
	var lesson model.CourseLesson
	if err := model.DB.Where("id = ?", lessonID).First(&lesson).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "小节不存在"})
		return
	}
	var chapter model.CourseChapter
	if err := model.DB.Where("id = ?", lesson.ChapterID).First(&chapter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "章节不存在"})
		return
	}
	if _, ok := ensureCoursePermission(c, chapter.CourseID); !ok {
		return
	}
	err := deleteDeliveryContent(chapter.CourseID, func(tx *gorm.DB) error {
		return tx.Where("id = ?", lessonID).Delete(&model.CourseLesson{}).Error
	})
	if err != nil {
		deliveryReply(c, nil, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}
