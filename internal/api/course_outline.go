package api

import (
	"net/http"
	"strconv"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

type chapterRequest struct {
	Title     string `json:"title" binding:"required"`
	SortOrder int    `json:"sort_order"`
}

type lessonRequest struct {
	Title         string `json:"title" binding:"required"`
	Type          int8   `json:"type"` // 1=视频,2=图文,3=直播
	MediaURL      string `json:"media_url"`
	DetailContent string `json:"detail_content"`
	LiveRoomID    string `json:"live_room_id"`
	LiveStart     string `json:"live_start"` // YYYY-MM-DD HH:mm:ss
	ExpireAt      string `json:"expire_at"`  // YYYY-MM-DD HH:mm:ss
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
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return nil
	}
	return &t
}

func normalizeLessonRequest(req *lessonRequest) {
	if req.Type < 1 || req.Type > 3 {
		req.Type = 1
	}
	if req.IsFree != 1 {
		req.IsFree = 0
	}
	switch req.Type {
	case 1:
		req.DetailContent = ""
		req.LiveRoomID = ""
		req.LiveStart = ""
	case 2:
		req.MediaURL = ""
		req.LiveRoomID = ""
		req.LiveStart = ""
	case 3:
		req.DetailContent = ""
		req.ExpireAt = ""
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
	if _, ok := ensureCoursePermission(c, courseID); !ok {
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
	if err := model.DB.Where("chapter_id = ?", chapterID).Delete(&model.CourseLesson{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除小节失败"})
		return
	}
	result := model.DB.Where("id = ?", chapterID).Delete(&model.CourseChapter{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除章节失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "章节不存在"})
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
	if err := model.DB.Where("chapter_id = ?", chapterID).Order("sort_order asc, id asc").Find(&lessons).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取小节失败"})
		return
	}
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
	lesson := model.CourseLesson{
		ChapterID:     chapterID,
		Title:         req.Title,
		Type:          req.Type,
		MediaURL:      req.MediaURL,
		DetailContent: req.DetailContent,
		LiveRoomID:    req.LiveRoomID,
		LiveStart:     parseTime(req.LiveStart),
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
	if err := model.DB.Create(&lesson).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "新增小节失败"})
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
	updates := map[string]interface{}{
		"title":          req.Title,
		"type":           req.Type,
		"media_url":      req.MediaURL,
		"detail_content": req.DetailContent,
		"live_room_id":   req.LiveRoomID,
		"live_start":     parseTime(req.LiveStart),
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
	result := model.DB.Where("id = ?", lessonID).Delete(&model.CourseLesson{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除小节失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "小节不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}
