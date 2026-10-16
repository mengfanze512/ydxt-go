package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type teacherHomeworkItem struct {
	model.HomeworkSubmission
	CourseID    uint64 `json:"course_id"`
	CourseTitle string `json:"course_title"`
	TaskTitle   string `json:"task_title"`
	StudentName string `json:"student_name"`
	TotalScore  int    `json:"total_score"`
	PassScore   int    `json:"pass_score"`
}

// A missing teacher identity must never fall back to administrator scope.
func teacherScopeID(c *gin.Context) (uint64, bool) {
	role, _ := c.Get("role")
	if role == int8(9) {
		return 0, true
	}
	if scoped, id := currentTeacherScope(c); role == int8(2) && scoped {
		return id, true
	}
	c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "教师身份无效，请重新登录教师账号"})
	return 0, false
}

func teacherCourseQuery(teacherID uint64) *gorm.DB {
	query := model.DB.Model(&model.Course{})
	if teacherID > 0 {
		query = query.Where("teacher_id = ?", teacherID)
	}
	return query
}

func teacherHomeworkQuery(teacherID uint64) *gorm.DB {
	query := model.DB.Table("homework_submissions h").
		Joins("JOIN learning_tasks t ON t.id = h.task_id").
		Joins("JOIN courses c ON c.id = t.course_id").
		Joins("LEFT JOIN users u ON u.id = h.user_id")
	if teacherID > 0 {
		query = query.Where("c.teacher_id = ?", teacherID)
	}
	return query
}

const teacherHomeworkColumns = "h.*, t.course_id, c.title AS course_title, t.title AS task_title, COALESCE(u.nickname, '') AS student_name, CASE WHEN t.total_score > 0 THEN t.total_score ELSE 100 END AS total_score, t.pass_score"

func teacherPagination(c *gin.Context) (int, int, bool) {
	page, size := 1, 10
	for key, dest := range map[string]*int{"page": &page, "page_size": &size} {
		if raw := c.Query(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || (key == "page_size" && value > 50) || (key == "page" && value > 100000) {
				deliveryReply(c, nil, fmt.Errorf("%s 参数无效", key))
				return 0, 0, false
			}
			*dest = value
		}
	}
	return page, size, true
}

func GetTeacherWorkbench(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := teacherScopeID(c)
	if !ok {
		return
	}
	var result struct {
		CourseCount    int64                 `json:"course_count"`
		PublishedCount int64                 `json:"published_count"`
		PendingCount   int64                 `json:"pending_count"`
		GradedCount    int64                 `json:"graded_count"`
		Courses        []model.Course        `json:"courses"`
		RecentHomework []teacherHomeworkItem `json:"recent_homework"`
	}
	result.Courses = []model.Course{}
	result.RecentHomework = []teacherHomeworkItem{}
	queries := []struct {
		Query *gorm.DB
		Count *int64
	}{
		{teacherCourseQuery(id), &result.CourseCount},
		{teacherCourseQuery(id).Where("status = 1"), &result.PublishedCount},
		{teacherHomeworkQuery(id).Where("h.status = 'submitted'"), &result.PendingCount},
		{teacherHomeworkQuery(id).Where("h.status = 'graded'"), &result.GradedCount},
	}
	for _, item := range queries {
		if err := item.Query.Count(item.Count).Error; err != nil {
			deliveryReply(c, nil, fmt.Errorf("获取教学概览失败: %w", err))
			return
		}
	}
	if err := teacherCourseQuery(id).Order("updated_at DESC, id DESC").Limit(4).Find(&result.Courses).Error; err != nil {
		deliveryReply(c, nil, fmt.Errorf("获取任教课程失败: %w", err))
		return
	}
	normalizeCoursePrices(result.Courses)
	if err := teacherHomeworkQuery(id).Select(teacherHomeworkColumns).
		Where("h.status = 'submitted'").Order("h.id DESC").Limit(3).Scan(&result.RecentHomework).Error; err != nil {
		deliveryReply(c, nil, fmt.Errorf("获取待批作业失败: %w", err))
		return
	}
	deliveryReply(c, result, nil)
}

func GetTeacherCourses(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := teacherScopeID(c)
	if !ok {
		return
	}
	page, size, ok := teacherPagination(c)
	if !ok {
		return
	}
	query := teacherCourseQuery(id)
	if level := c.Query("level"); level != "" {
		switch level {
		case "system", "class", "single", "1v1", "vip":
			query = query.Where("level = ?", level)
		default:
			deliveryReply(c, nil, errors.New("课程类型无效"))
			return
		}
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		query = query.Where("title LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		deliveryReply(c, nil, fmt.Errorf("获取课程数量失败: %w", err))
		return
	}
	items := []model.Course{}
	if err := query.Order("updated_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		deliveryReply(c, nil, fmt.Errorf("获取任教课程失败: %w", err))
		return
	}
	normalizeCoursePrices(items)
	deliveryReply(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, nil)
}

func GetTeacherHomework(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := teacherScopeID(c)
	if !ok {
		return
	}
	page, size, ok := teacherPagination(c)
	if !ok {
		return
	}
	query := teacherHomeworkQuery(id)
	if raw := c.Query("course_id"); raw != "" {
		courseID, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || courseID == 0 {
			deliveryReply(c, nil, errors.New("课程 ID 无效"))
			return
		}
		query = query.Where("t.course_id = ?", courseID)
	}
	var pending, graded int64
	if err := query.Session(&gorm.Session{}).Where("h.status = 'submitted'").Count(&pending).Error; err != nil {
		deliveryReply(c, nil, fmt.Errorf("获取待批数量失败: %w", err))
		return
	}
	if err := query.Session(&gorm.Session{}).Where("h.status = 'graded'").Count(&graded).Error; err != nil {
		deliveryReply(c, nil, fmt.Errorf("获取已批数量失败: %w", err))
		return
	}
	if status := c.Query("status"); status != "" {
		if status != "submitted" && status != "graded" {
			deliveryReply(c, nil, errors.New("作业状态无效"))
			return
		}
		query = query.Where("h.status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		deliveryReply(c, nil, fmt.Errorf("获取作业数量失败: %w", err))
		return
	}
	items := []teacherHomeworkItem{}
	if err := query.Select(teacherHomeworkColumns).Order("h.id DESC").Offset((page - 1) * size).Limit(size).Scan(&items).Error; err != nil {
		deliveryReply(c, nil, fmt.Errorf("获取作业列表失败: %w", err))
		return
	}
	deliveryReply(c, gin.H{"items": items, "total": total, "page": page, "page_size": size, "pending_count": pending, "graded_count": graded}, nil)
}

func scopedTeacherHomework(c *gin.Context) (*teacherHomeworkItem, bool) {
	if !deliveryDB(c) {
		return nil, false
	}
	teacherID, ok := teacherScopeID(c)
	if !ok {
		return nil, false
	}
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil, false
	}
	var item teacherHomeworkItem
	if err := teacherHomeworkQuery(teacherID).Select(teacherHomeworkColumns).Where("h.id = ?", id).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "作业不存在或不属于您的课程"})
		} else {
			deliveryReply(c, nil, fmt.Errorf("获取作业失败: %w", err))
		}
		return nil, false
	}
	return &item, true
}

func GradeTeacherHomework(c *gin.Context) {
	item, ok := scopedTeacherHomework(c)
	if !ok {
		return
	}
	var req struct {
		Score    *int   `json:"score"`
		IsPassed *int8  `json:"is_passed"`
		Feedback string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		deliveryReply(c, nil, fmt.Errorf("批改参数错误: %w", err))
		return
	}
	if req.Score == nil || *req.Score < 0 || *req.Score > item.TotalScore {
		deliveryReply(c, nil, fmt.Errorf("分数必须在 0 到 %d 之间", item.TotalScore))
		return
	}
	if req.IsPassed == nil || (*req.IsPassed != 0 && *req.IsPassed != 1) || strings.TrimSpace(req.Feedback) == "" {
		deliveryReply(c, nil, errors.New("请填写评语并选择是否通过"))
		return
	}
	graderID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}
	if err := model.DB.Model(&model.HomeworkSubmission{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
		"score": *req.Score, "is_passed": *req.IsPassed, "feedback": strings.TrimSpace(req.Feedback),
		"status": "graded", "graded_at": time.Now(), "grader_id": graderID,
	}).Error; err != nil {
		deliveryReply(c, nil, fmt.Errorf("保存批改失败: %w", err))
		return
	}
	deliveryReply(c, nil, nil)
}

func GetTeacherHomeworkPlayURL(c *gin.Context) {
	item, ok := scopedTeacherHomework(c)
	if !ok {
		return
	}
	if item.MediaFileID == "" {
		deliveryReply(c, nil, errors.New("该作业没有可播放的附件"))
		return
	}
	query := c.Request.URL.Query()
	query.Set("file_id", item.MediaFileID)
	c.Request.URL.RawQuery = query.Encode()
	GetHomeworkMediaPlayURL(c)
}
