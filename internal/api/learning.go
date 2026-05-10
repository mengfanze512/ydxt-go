package api

import (
	"net/http"
	"strconv"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

type learningTaskRequest struct {
	CourseID    uint64  `json:"course_id" binding:"required"`
	TargetType  string  `json:"target_type" binding:"required,oneof=chapter lesson"`
	TargetID    uint64  `json:"target_id" binding:"required"`
	TaskType    string  `json:"task_type" binding:"required,oneof=homework quiz"`
	Title       string  `json:"title" binding:"required"`
	Description string  `json:"description"`
	TotalScore  int     `json:"total_score"`
	PassScore   int     `json:"pass_score"`
	MaxAttempts int     `json:"max_attempts"`
	StartAt     *string `json:"start_at"`
	DueAt       *string `json:"due_at"`
	Status      string  `json:"status"`
}

type taskQuestionRequest struct {
	QuestionType  string `json:"question_type" binding:"required"`
	Stem          string `json:"stem" binding:"required"`
	OptionsJSON   string `json:"options_json"`
	AnswerKeyJSON string `json:"answer_key_json"`
	Score         int    `json:"score"`
	SortOrder     int    `json:"sort_order"`
}

type submitTaskRequest struct {
	Answers []struct {
		QuestionID uint64 `json:"question_id" binding:"required"`
		AnswerJSON string `json:"answer_json"`
	} `json:"answers" binding:"required"`
}

type lessonProgressRequest struct {
	CourseID            uint64   `json:"course_id" binding:"required"`
	ChapterID           *uint64  `json:"chapter_id"`
	LessonID            uint64   `json:"lesson_id" binding:"required"`
	ProgressPercent     float64  `json:"progress_percent"`
	LastPositionSeconds int      `json:"last_position_seconds"`
	DeltaStudySeconds   int      `json:"delta_study_seconds"`
	IsCompleted         int8     `json:"is_completed"`
}

type gradeSubmissionRequest struct {
	ManualScore int    `json:"manual_score"`
	IsPassed    int8   `json:"is_passed"`
	Feedback    string `json:"feedback"`
}

func parseTimePtr(v *string) *time.Time {
	if v == nil || *v == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02 15:04:05", *v)
	if err != nil {
		return nil
	}
	return &t
}

func parseUintParam(c *gin.Context, key string) (uint64, bool) {
	n, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return 0, false
	}
	return n, true
}

func getUserIDFromContext(c *gin.Context) (uint64, bool) {
	v, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return 0, false
	}
	userID, ok := v.(uint64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "用户信息异常"})
		return 0, false
	}
	return userID, true
}

// ----- 管理端：任务 CRUD -----

func GetLearningTaskDetail(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var task model.LearningTask
	if err := model.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": task})
}

func GetLearningTasks(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.LearningTask{}})
		return
	}

	query := model.DB.Model(&model.LearningTask{})
	if courseID := c.Query("course_id"); courseID != "" {
		query = query.Where("course_id = ?", courseID)
	}
	if targetType := c.Query("target_type"); targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if targetID := c.Query("target_id"); targetID != "" {
		query = query.Where("target_id = ?", targetID)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var tasks []model.LearningTask
	if err := query.Order("id desc").Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取任务失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": tasks})
}

func CreateLearningTask(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	var req learningTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	task := model.LearningTask{
		CourseID:    req.CourseID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		TaskType:    req.TaskType,
		Title:       req.Title,
		Description: req.Description,
		TotalScore:  req.TotalScore,
		PassScore:   req.PassScore,
		MaxAttempts: req.MaxAttempts,
		StartAt:     parseTimePtr(req.StartAt),
		DueAt:       parseTimePtr(req.DueAt),
		Status:      req.Status,
	}
	if task.TotalScore <= 0 {
		task.TotalScore = 100
	}
	if task.PassScore <= 0 {
		task.PassScore = 60
	}
	if task.MaxAttempts <= 0 {
		task.MaxAttempts = 1
	}
	if task.Status == "" {
		task.Status = "draft"
	}
	if err := model.DB.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建任务失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": task})
}

func UpdateLearningTask(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req learningTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	updates := map[string]interface{}{
		"course_id":    req.CourseID,
		"target_type":  req.TargetType,
		"target_id":    req.TargetID,
		"task_type":    req.TaskType,
		"title":        req.Title,
		"description":  req.Description,
		"total_score":  req.TotalScore,
		"pass_score":   req.PassScore,
		"max_attempts": req.MaxAttempts,
		"start_at":     parseTimePtr(req.StartAt),
		"due_at":       parseTimePtr(req.DueAt),
		"status":       req.Status,
	}
	result := model.DB.Model(&model.LearningTask{}).Where("id = ?", taskID).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新任务失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

func DeleteLearningTask(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	result := model.DB.Where("id = ?", taskID).Delete(&model.LearningTask{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除任务失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

func PublishLearningTask(c *gin.Context) {
	updateTaskStatus(c, "published")
}

func CloseLearningTask(c *gin.Context) {
	updateTaskStatus(c, "closed")
}

func updateTaskStatus(c *gin.Context, status string) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	result := model.DB.Model(&model.LearningTask{}).Where("id = ?", taskID).Update("status", status)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新任务状态失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// ----- 管理端：题目 CRUD -----

func GetTaskQuestions(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.LearningTaskQuestion{}})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var questions []model.LearningTaskQuestion
	if err := model.DB.Where("task_id = ?", taskID).Order("sort_order asc, id asc").Find(&questions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取题目失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": questions})
}

func CreateTaskQuestion(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req taskQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	question := model.LearningTaskQuestion{
		TaskID:        taskID,
		QuestionType:  req.QuestionType,
		Stem:          req.Stem,
		OptionsJSON:   req.OptionsJSON,
		AnswerKeyJSON: req.AnswerKeyJSON,
		Score:         req.Score,
		SortOrder:     req.SortOrder,
	}
	if err := model.DB.Create(&question).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建题目失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": question})
}

func UpdateTaskQuestion(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	questionID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req taskQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	updates := map[string]interface{}{
		"question_type":   req.QuestionType,
		"stem":            req.Stem,
		"options_json":    req.OptionsJSON,
		"answer_key_json": req.AnswerKeyJSON,
		"score":           req.Score,
		"sort_order":      req.SortOrder,
	}
	result := model.DB.Model(&model.LearningTaskQuestion{}).Where("id = ?", questionID).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新题目失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

func DeleteTaskQuestion(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	questionID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := model.DB.Where("id = ?", questionID).Delete(&model.LearningTaskQuestion{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除题目失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

func GetTaskSubmissions(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.LearningTaskSubmission{}})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var submissions []model.LearningTaskSubmission
	if err := model.DB.Where("task_id = ?", taskID).Order("id desc").Find(&submissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取提交记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": submissions})
}

// ----- 学员端：任务与进度 -----

func GetAvailableTasks(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.LearningTask{}})
		return
	}
	courseID := c.Query("course_id")
	targetType := c.Query("target_type")
	targetID := c.Query("target_id")
	query := model.DB.Model(&model.LearningTask{}).Where("status = ?", "published")
	if courseID != "" {
		query = query.Where("course_id = ?", courseID)
	}
	if targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if targetID != "" {
		query = query.Where("target_id = ?", targetID)
	}
	var tasks []model.LearningTask
	if err := query.Order("id desc").Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取任务失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": tasks})
}

func GetTaskQuestionsForLearner(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []gin.H{}})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var task model.LearningTask
	if err := model.DB.Where("id = ? AND status = ?", taskID, "published").First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "任务不存在或未发布"})
		return
	}
	var questions []model.LearningTaskQuestion
	if err := model.DB.Where("task_id = ?", taskID).Order("sort_order asc, id asc").Find(&questions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取题目失败"})
		return
	}
	// 学员端不返回标准答案
	type learnerQuestion struct {
		ID           uint64 `json:"id"`
		TaskID       uint64 `json:"task_id"`
		QuestionType string `json:"question_type"`
		Stem         string `json:"stem"`
		OptionsJSON  string `json:"options_json"`
		Score        int    `json:"score"`
		SortOrder    int    `json:"sort_order"`
	}
	result := make([]learnerQuestion, 0, len(questions))
	for _, q := range questions {
		result = append(result, learnerQuestion{
			ID:           q.ID,
			TaskID:       q.TaskID,
			QuestionType: q.QuestionType,
			Stem:         q.Stem,
			OptionsJSON:  q.OptionsJSON,
			Score:        q.Score,
			SortOrder:    q.SortOrder,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": result})
}

func SubmitLearningTask(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}
	var req submitTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var task model.LearningTask
	if err := model.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "任务不存在"})
		return
	}

	var last model.LearningTaskSubmission
	nextAttempt := 1
	if err := model.DB.Where("task_id = ? AND user_id = ?", taskID, userID).Order("attempt_no desc").First(&last).Error; err == nil {
		nextAttempt = last.AttemptNo + 1
	}
	if task.MaxAttempts > 0 && nextAttempt > task.MaxAttempts {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "已达到最大提交次数"})
		return
	}

	now := time.Now()
	submission := model.LearningTaskSubmission{
		TaskID:       taskID,
		UserID:       userID,
		AttemptNo:    nextAttempt,
		SubmitStatus: "submitted",
		AutoScore:    0,
		ManualScore:  0,
		FinalScore:   0,
		IsPassed:     0,
		SubmittedAt:  &now,
	}
	if err := model.DB.Create(&submission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "提交失败"})
		return
	}

	answers := make([]model.LearningTaskAnswer, 0, len(req.Answers))
	for _, item := range req.Answers {
		answers = append(answers, model.LearningTaskAnswer{
			SubmissionID: submission.ID,
			QuestionID:   item.QuestionID,
			AnswerJSON:   item.AnswerJSON,
			ScoreAwarded: 0,
		})
	}
	if len(answers) > 0 {
		if err := model.DB.Create(&answers).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "保存答案失败"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": submission})
}

func GetMyTaskSubmissions(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.LearningTaskSubmission{}})
		return
	}
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}
	var submissions []model.LearningTaskSubmission
	if err := model.DB.Where("task_id = ? AND user_id = ?", taskID, userID).Order("attempt_no desc").Find(&submissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取提交历史失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": submissions})
}

func GetSubmissionAnswers(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.LearningTaskAnswer{}})
		return
	}
	submissionID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var answers []model.LearningTaskAnswer
	if err := model.DB.Where("submission_id = ?", submissionID).Order("id asc").Find(&answers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取答案明细失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": answers})
}

func GradeSubmission(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	submissionID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req gradeSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.ManualScore < 0 {
		req.ManualScore = 0
	}

	var submission model.LearningTaskSubmission
	if err := model.DB.Where("id = ?", submissionID).First(&submission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "提交记录不存在"})
		return
	}

	graderID, _ := c.Get("userID")
	now := time.Now()
	finalScore := submission.AutoScore + req.ManualScore
	updates := map[string]interface{}{
		"manual_score":  req.ManualScore,
		"final_score":   finalScore,
		"is_passed":     req.IsPassed,
		"feedback":      req.Feedback,
		"submit_status": "graded",
		"graded_at":     &now,
	}
	if uid, ok := graderID.(uint64); ok {
		updates["grader_id"] = uid
	}
	if err := model.DB.Model(&model.LearningTaskSubmission{}).Where("id = ?", submissionID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "评分失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

func UpsertLessonProgress(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}
	var req lessonProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	now := time.Now()

	var existing model.UserLessonProgress
	err := model.DB.Where("user_id = ? AND lesson_id = ?", userID, req.LessonID).First(&existing).Error
	if err == nil {
		existing.ProgressPercent = req.ProgressPercent
		existing.LastPositionSeconds = req.LastPositionSeconds
		existing.StudySeconds = existing.StudySeconds + req.DeltaStudySeconds
		existing.IsCompleted = req.IsCompleted
		existing.LastStudyAt = &now
		if req.IsCompleted == 1 {
			existing.CompletedAt = &now
		}
		if err := model.DB.Save(&existing).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新小节进度失败"})
			return
		}
	} else {
		lp := model.UserLessonProgress{
			UserID:              userID,
			CourseID:            req.CourseID,
			ChapterID:           req.ChapterID,
			LessonID:            req.LessonID,
			ProgressPercent:     req.ProgressPercent,
			LastPositionSeconds: req.LastPositionSeconds,
			StudySeconds:        req.DeltaStudySeconds,
			IsCompleted:         req.IsCompleted,
			FirstStudyAt:        &now,
			LastStudyAt:         &now,
		}
		if req.IsCompleted == 1 {
			lp.CompletedAt = &now
		}
		if err := model.DB.Create(&lp).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "保存小节进度失败"})
			return
		}
	}

	// 回写课程总进度
	var totalLessons int64
	model.DB.Raw(`
		SELECT COUNT(1)
		FROM course_lessons cl
		INNER JOIN course_chapters cc ON cc.id = cl.chapter_id
		WHERE cc.course_id = ?`, req.CourseID).Scan(&totalLessons)

	var completedLessons int64
	model.DB.Model(&model.UserLessonProgress{}).
		Where("user_id = ? AND course_id = ? AND is_completed = 1", userID, req.CourseID).
		Count(&completedLessons)

	progress := 0.0
	status := "not_started"
	if totalLessons > 0 {
		progress = float64(completedLessons) * 100 / float64(totalLessons)
	}
	if completedLessons > 0 {
		status = "in_progress"
	}
	var completedAt *time.Time
	if totalLessons > 0 && completedLessons >= totalLessons {
		status = "completed"
		t := now
		completedAt = &t
	}

	ucp := model.UserCourseProgress{
		UserID:           userID,
		CourseID:         req.CourseID,
		Status:           status,
		ProgressPercent:  progress,
		CompletedLessons: int(completedLessons),
		TotalLessons:     int(totalLessons),
		LastLessonID:     &req.LessonID,
		LastStudyAt:      &now,
		CompletedAt:      completedAt,
	}

	if err := model.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "course_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "progress_percent", "completed_lessons", "total_lessons",
			"last_lesson_id", "last_study_at", "completed_at", "updated_at",
		}),
	}).Create(&ucp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "回写课程进度失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

func GetMyCourseProgress(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}
	courseID, ok := parseUintParam(c, "course_id")
	if !ok {
		return
	}
	var progress model.UserCourseProgress
	if err := model.DB.Where("user_id = ? AND course_id = ?", userID, courseID).First(&progress).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": progress})
}
