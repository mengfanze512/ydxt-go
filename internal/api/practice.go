package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var learningTimezone = time.FixedZone("Asia/Shanghai", 8*60*60)

func learningToday() string { return time.Now().In(learningTimezone).Format("2006-01-02") }

func checkinStreak(dates map[string]bool, today string) int {
	day, _ := time.ParseInLocation("2006-01-02", today, learningTimezone)
	if !dates[today] {
		day = day.AddDate(0, 0, -1)
	}
	count := 0
	for dates[day.Format("2006-01-02")] {
		count++
		day = day.AddDate(0, 0, -1)
	}
	return count
}

// Lock the learner row to serialize duplicate submissions without a new schema.
func DoCheckin(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}
	today := learningToday()
	var checkin model.UserCheckin
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return fmt.Errorf("核验学员账号失败: %w", err)
		}
		err := tx.Where("user_id = ? AND checkin_date = ?", userID, today).First(&checkin).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("查询今日打卡失败: %w", err)
		}
		var history []model.UserCheckin
		if err := tx.Where("user_id = ?", userID).Find(&history).Error; err != nil {
			return fmt.Errorf("查询连续打卡失败: %w", err)
		}
		dates := map[string]bool{today: true}
		for _, row := range history {
			dates[normalizeCheckinDate(row.CheckinDate)] = true
		}
		checkin = model.UserCheckin{UserID: userID, CheckinDate: today, ContinuousDays: checkinStreak(dates, today)}
		if err := tx.Create(&checkin).Error; err != nil {
			return fmt.Errorf("写入打卡记录失败: %w", err)
		}
		return nil
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "打卡失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "打卡成功", "data": checkin})
}

func normalizeCheckinDate(value string) string {
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}

type learningCalendarEvent struct {
	Date        string `json:"date"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	CourseID    uint64 `json:"course_id"`
	CourseTitle string `json:"course_title"`
	CohortID    uint64 `json:"cohort_id,omitempty"`
	TaskID      uint64 `json:"task_id,omitempty"`
}

func GetLearningCalendar(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}
	month := c.Query("month")
	if month == "" {
		month = learningToday()[:7]
	}
	start, err := time.ParseInLocation("2006-01", month, learningTimezone)
	if err != nil || len(month) != 7 {
		c.JSON(400, gin.H{"code": 400, "msg": "月份格式必须为 YYYY-MM"})
		return
	}
	end := start.AddDate(0, 1, 0)
	var history []model.UserCheckin
	if err := model.DB.Where("user_id = ?", userID).Order("checkin_date DESC").Find(&history).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "获取打卡记录失败: " + err.Error()})
		return
	}
	dates := map[string]bool{}
	checkins := []string{}
	for _, item := range history {
		date := normalizeCheckinDate(item.CheckinDate)
		if !dates[date] && len(date) >= 7 && date[:7] == month {
			checkins = append(checkins, date)
		}
		dates[date] = true
	}
	events := []learningCalendarEvent{}
	var cohorts []struct {
		ID          uint64
		CourseID    uint64
		Name        string
		CourseTitle string
		StartAt     time.Time
		EndAt       time.Time
	}
	err = model.DB.Table("course_cohorts co").
		Joins("JOIN courses c ON c.id = co.course_id").
		Where("EXISTS (SELECT 1 FROM course_enrollments e WHERE e.user_id = ? AND e.course_id = co.course_id AND e.cohort_id = co.id)", userID).
		Where("(co.start_at >= ? AND co.start_at < ?) OR (co.end_at >= ? AND co.end_at < ?)", start, end, start, end).
		Select("co.id, co.course_id, co.name, co.start_at, co.end_at, c.title AS course_title").Scan(&cohorts).Error
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "获取已报名班期失败: " + err.Error()})
		return
	}
	for _, cohort := range cohorts {
		for _, point := range []struct {
			At          time.Time
			Kind, Label string
		}{{cohort.StartAt, "cohort_start", "班期开课"}, {cohort.EndAt, "cohort_end", "班期结束"}} {
			date := point.At.In(learningTimezone).Format("2006-01-02")
			if !point.At.IsZero() && date[:7] == month {
				events = append(events, learningCalendarEvent{Date: date, Kind: point.Kind, Title: cohort.Name + " · " + point.Label, CourseID: cohort.CourseID, CourseTitle: cohort.CourseTitle, CohortID: cohort.ID})
			}
		}
	}
	var tasks []struct {
		ID          uint64
		CourseID    uint64
		Title       string
		CourseTitle string
		DueAt       time.Time
	}
	err = model.DB.Table("learning_tasks t").Joins("JOIN courses c ON c.id = t.course_id").
		Where("t.task_type = 'homework' AND t.status = 'published' AND t.due_at >= ? AND t.due_at < ?", start, end).
		Where("EXISTS (SELECT 1 FROM course_enrollments e WHERE e.user_id = ? AND e.course_id = t.course_id AND (e.expires_at IS NULL OR e.expires_at > ?))", userID, time.Now()).
		Select("t.id, t.course_id, t.title, t.due_at, c.title AS course_title").Scan(&tasks).Error
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "获取作业截止提醒失败: " + err.Error()})
		return
	}
	for _, task := range tasks {
		events = append(events, learningCalendarEvent{Date: task.DueAt.In(learningTimezone).Format("2006-01-02"), Kind: "homework_due", Title: task.Title, CourseID: task.CourseID, CourseTitle: task.CourseTitle, TaskID: task.ID})
	}
	today := learningToday()
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"month": month, "today": today, "checked_today": dates[today], "continuous_days": checkinStreak(dates, today), "month_count": len(checkins), "checkin_dates": checkins, "events": events}})
}
