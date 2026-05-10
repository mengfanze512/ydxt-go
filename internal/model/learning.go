package model

import "time"

// LearningTask 作业/测验任务主表
type LearningTask struct {
	ID          uint64     `gorm:"primaryKey;column:id" json:"id"`
	CourseID    uint64     `gorm:"column:course_id" json:"course_id"`
	TargetType  string     `gorm:"column:target_type" json:"target_type"` // chapter/lesson
	TargetID    uint64     `gorm:"column:target_id" json:"target_id"`
	TaskType    string     `gorm:"column:task_type" json:"task_type"` // homework/quiz
	Title       string     `gorm:"column:title" json:"title"`
	Description string     `gorm:"column:description" json:"description"`
	TotalScore  int        `gorm:"column:total_score" json:"total_score"`
	PassScore   int        `gorm:"column:pass_score" json:"pass_score"`
	MaxAttempts int        `gorm:"column:max_attempts" json:"max_attempts"`
	StartAt     *time.Time `gorm:"column:start_at" json:"start_at"`
	DueAt       *time.Time `gorm:"column:due_at" json:"due_at"`
	Status      string     `gorm:"column:status" json:"status"` // draft/published/closed
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (LearningTask) TableName() string {
	return "learning_tasks"
}

// LearningTaskQuestion 任务题目表
type LearningTaskQuestion struct {
	ID            uint64    `gorm:"primaryKey;column:id" json:"id"`
	TaskID        uint64    `gorm:"column:task_id" json:"task_id"`
	QuestionType  string    `gorm:"column:question_type" json:"question_type"`
	Stem          string    `gorm:"column:stem" json:"stem"`
	OptionsJSON   string    `gorm:"column:options_json;type:json" json:"options_json"`
	AnswerKeyJSON string    `gorm:"column:answer_key_json;type:json" json:"answer_key_json"`
	Score         int       `gorm:"column:score" json:"score"`
	SortOrder     int       `gorm:"column:sort_order" json:"sort_order"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (LearningTaskQuestion) TableName() string {
	return "learning_task_questions"
}

// LearningTaskSubmission 任务提交记录
type LearningTaskSubmission struct {
	ID           uint64     `gorm:"primaryKey;column:id" json:"id"`
	TaskID       uint64     `gorm:"column:task_id" json:"task_id"`
	UserID       uint64     `gorm:"column:user_id" json:"user_id"`
	AttemptNo    int        `gorm:"column:attempt_no" json:"attempt_no"`
	SubmitStatus string     `gorm:"column:submit_status" json:"submit_status"`
	AutoScore    int        `gorm:"column:auto_score" json:"auto_score"`
	ManualScore  int        `gorm:"column:manual_score" json:"manual_score"`
	FinalScore   int        `gorm:"column:final_score" json:"final_score"`
	IsPassed     int8       `gorm:"column:is_passed" json:"is_passed"`
	SubmittedAt  *time.Time `gorm:"column:submitted_at" json:"submitted_at"`
	GradedAt     *time.Time `gorm:"column:graded_at" json:"graded_at"`
	GraderID     *uint64    `gorm:"column:grader_id" json:"grader_id"`
	Feedback     string     `gorm:"column:feedback" json:"feedback"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (LearningTaskSubmission) TableName() string {
	return "learning_task_submissions"
}

// LearningTaskAnswer 任务答案明细
type LearningTaskAnswer struct {
	ID            uint64    `gorm:"primaryKey;column:id" json:"id"`
	SubmissionID  uint64    `gorm:"column:submission_id" json:"submission_id"`
	QuestionID    uint64    `gorm:"column:question_id" json:"question_id"`
	AnswerJSON    string    `gorm:"column:answer_json;type:json" json:"answer_json"`
	IsCorrect     *int8     `gorm:"column:is_correct" json:"is_correct"`
	ScoreAwarded  int       `gorm:"column:score_awarded" json:"score_awarded"`
	TeacherComment string   `gorm:"column:teacher_comment" json:"teacher_comment"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (LearningTaskAnswer) TableName() string {
	return "learning_task_answers"
}

// UserCourseProgress 学员课程级进度
type UserCourseProgress struct {
	ID               uint64     `gorm:"primaryKey;column:id" json:"id"`
	UserID           uint64     `gorm:"column:user_id" json:"user_id"`
	CourseID         uint64     `gorm:"column:course_id" json:"course_id"`
	Status           string     `gorm:"column:status" json:"status"` // not_started/in_progress/completed
	ProgressPercent  float64    `gorm:"column:progress_percent" json:"progress_percent"`
	CompletedLessons int        `gorm:"column:completed_lessons" json:"completed_lessons"`
	TotalLessons     int        `gorm:"column:total_lessons" json:"total_lessons"`
	LastLessonID     *uint64    `gorm:"column:last_lesson_id" json:"last_lesson_id"`
	LastStudyAt      *time.Time `gorm:"column:last_study_at" json:"last_study_at"`
	CompletedAt      *time.Time `gorm:"column:completed_at" json:"completed_at"`
	CreatedAt        time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (UserCourseProgress) TableName() string {
	return "user_course_progress"
}

// UserLessonProgress 学员小节级进度
type UserLessonProgress struct {
	ID                  uint64     `gorm:"primaryKey;column:id" json:"id"`
	UserID              uint64     `gorm:"column:user_id" json:"user_id"`
	CourseID            uint64     `gorm:"column:course_id" json:"course_id"`
	ChapterID           *uint64    `gorm:"column:chapter_id" json:"chapter_id"`
	LessonID            uint64     `gorm:"column:lesson_id" json:"lesson_id"`
	ProgressPercent     float64    `gorm:"column:progress_percent" json:"progress_percent"`
	LastPositionSeconds int        `gorm:"column:last_position_seconds" json:"last_position_seconds"`
	StudySeconds        int        `gorm:"column:study_seconds" json:"study_seconds"`
	IsCompleted         int8       `gorm:"column:is_completed" json:"is_completed"`
	FirstStudyAt        *time.Time `gorm:"column:first_study_at" json:"first_study_at"`
	LastStudyAt         *time.Time `gorm:"column:last_study_at" json:"last_study_at"`
	CompletedAt         *time.Time `gorm:"column:completed_at" json:"completed_at"`
	CreatedAt           time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (UserLessonProgress) TableName() string {
	return "user_lesson_progress"
}
