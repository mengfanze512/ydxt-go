package api

import (
	"net/http"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

type dashboardSummary struct {
	TeacherCount       int64                `json:"teacher_count"`
	StudentCount       int64                `json:"student_count"`
	TodayOrderCount    int64                `json:"today_order_count"`
	TodayRevenue       int64                `json:"today_revenue"`
	ActiveStudentCount int64                `json:"active_student_count"`
	HotCourses         []dashboardHotCourse `json:"hot_courses"`
	Notices            []dashboardNotice    `json:"notices"`
}

type dashboardHotCourse struct {
	ID    uint64  `json:"id"`
	Name  string  `json:"name"`
	Sales int     `json:"sales"`
	Price float64 `json:"price"`
}

type dashboardNotice struct {
	Title string `json:"title"`
	Time  string `json:"time"`
}

func getDayRange() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.Add(24 * time.Hour)
	return start, end
}

// GetDashboardSummary 获取真实数据看板
func GetDashboardSummary(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": dashboardSummary{
			HotCourses: []dashboardHotCourse{},
			Notices:    []dashboardNotice{},
		}})
		return
	}

	summary := dashboardSummary{
		HotCourses: []dashboardHotCourse{},
		Notices:    []dashboardNotice{},
	}
	start, end := getDayRange()
	queryDB := model.DB
	isTeacher, teacherID := currentTeacherScope(c)

	if isTeacher {
		_ = queryDB.Model(&model.Teacher{}).Where("id = ?", teacherID).Count(&summary.TeacherCount).Error

		_ = queryDB.Table("users u").
			Joins("JOIN user_courses uc ON uc.user_id = u.id").
			Joins("JOIN courses c ON c.id = uc.course_id").
			Where("u.role = ?", 1).
			Where("c.teacher_id = ?", teacherID).
			Distinct("u.id").
			Count(&summary.StudentCount).Error

		teacherOrderSubQuery := queryDB.Table("order_items oi").
			Select("DISTINCT oi.order_id").
			Joins("JOIN courses c ON c.id = oi.goods_id").
			Joins("JOIN orders o ON o.id = oi.order_id").
			Where("o.type = ?", "course").
			Where("c.teacher_id = ?", teacherID)

		_ = queryDB.Model(&model.Order{}).
			Where("id IN (?)", teacherOrderSubQuery).
			Where("created_at >= ? AND created_at < ?", start, end).
			Count(&summary.TodayOrderCount).Error

		type revenueResult struct {
			Total int64 `json:"total"`
		}
		var revenue revenueResult
		_ = queryDB.Model(&model.Order{}).
			Select("COALESCE(SUM(pay_amount), 0) as total").
			Where("id IN (?)", teacherOrderSubQuery).
			Where("created_at >= ? AND created_at < ?", start, end).
			Scan(&revenue).Error
		summary.TodayRevenue = revenue.Total

		_ = queryDB.Table("users u").
			Joins("JOIN user_courses uc ON uc.user_id = u.id").
			Joins("JOIN courses c ON c.id = uc.course_id").
			Where("u.role = ?", 1).
			Where("c.teacher_id = ?", teacherID).
			Distinct("u.id").
			Count(&summary.ActiveStudentCount).Error

		_ = queryDB.Model(&model.Course{}).
			Where("teacher_id = ?", teacherID).
			Order("student_count desc, created_at desc").
			Limit(5).
			Find(&summary.HotCourses).Error
	} else {
		_ = queryDB.Model(&model.Teacher{}).Count(&summary.TeacherCount).Error
		_ = queryDB.Model(&model.User{}).Where("role = ?", 1).Count(&summary.StudentCount).Error
		_ = queryDB.Model(&model.Order{}).
			Where("created_at >= ? AND created_at < ?", start, end).
			Count(&summary.TodayOrderCount).Error

		type revenueResult struct {
			Total int64 `json:"total"`
		}
		var revenue revenueResult
		_ = queryDB.Model(&model.Order{}).
			Select("COALESCE(SUM(pay_amount), 0) as total").
			Where("created_at >= ? AND created_at < ?", start, end).
			Scan(&revenue).Error
		summary.TodayRevenue = revenue.Total

		_ = queryDB.Table("user_courses").Distinct("user_id").Count(&summary.ActiveStudentCount).Error
		_ = queryDB.Model(&model.Course{}).
			Order("student_count desc, created_at desc").
			Limit(5).
			Find(&summary.HotCourses).Error
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": summary,
	})
}
