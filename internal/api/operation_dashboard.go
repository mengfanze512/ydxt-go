package api

import (
	"errors"
	"net/http"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

type operationDashboardOverview struct {
	RangeStart       string  `json:"range_start"`
	RangeEnd         string  `json:"range_end"`
	NewStudentCount  int64   `json:"new_student_count"`
	ActiveStudentCnt int64   `json:"active_student_count"`
	OrderCount       int64   `json:"order_count"`
	PaidOrderCount   int64   `json:"paid_order_count"`
	PaidUserCount    int64   `json:"paid_user_count"`
	GMV              int64   `json:"gmv"`
	RefundOrderCount int64   `json:"refund_order_count"`
	RefundAmount     int64   `json:"refund_amount"`
	NetGMV           int64   `json:"net_gmv"`
	RepurchaseRate   float64 `json:"repurchase_rate"`
	OrderToPayRate   float64 `json:"order_to_pay_rate"`
}

type operationDashboardTrendItem struct {
	Date         string `json:"date"`
	NewStudents  int64  `json:"new_students"`
	Orders       int64  `json:"orders"`
	PaidOrders   int64  `json:"paid_orders"`
	GMV          int64  `json:"gmv"`
	RefundAmount int64  `json:"refund_amount"`
}

type operationDashboardTopCourse struct {
	CourseID       uint64 `json:"course_id"`
	Title          string `json:"title"`
	Revenue        int64  `json:"revenue"`
	PaidOrderCount int64  `json:"paid_order_count"`
	PaidUserCount  int64  `json:"paid_user_count"`
}

type operationDashboardTopTeacher struct {
	TeacherID      uint64 `json:"teacher_id"`
	Name           string `json:"name"`
	Revenue        int64  `json:"revenue"`
	PaidOrderCount int64  `json:"paid_order_count"`
	PaidUserCount  int64  `json:"paid_user_count"`
}

func parseDashboardRange(c *gin.Context) (time.Time, time.Time, bool) {
	const layout = "2006-01-02"
	now := time.Now()
	defaultEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(24 * time.Hour)
	defaultStart := defaultEnd.AddDate(0, 0, -7)

	startStr := c.Query("start_date")
	endStr := c.Query("end_date")
	if startStr == "" && endStr == "" {
		return defaultStart, defaultEnd, true
	}
	if startStr == "" || endStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "start_date/end_date 参数缺失"})
		return time.Time{}, time.Time{}, false
	}
	start, err := time.ParseInLocation(layout, startStr, now.Location())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "start_date 格式错误"})
		return time.Time{}, time.Time{}, false
	}
	endDay, err := time.ParseInLocation(layout, endStr, now.Location())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "end_date 格式错误"})
		return time.Time{}, time.Time{}, false
	}
	end := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), 0, 0, 0, 0, endDay.Location()).Add(24 * time.Hour)
	if !end.After(start) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "日期范围不合法"})
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func GetOperationDashboardOverview(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	if model.DB == nil {
		start, end, _ := parseDashboardRange(c)
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": operationDashboardOverview{
			RangeStart:       start.Format("2006-01-02"),
			RangeEnd:         end.AddDate(0, 0, -1).Format("2006-01-02"),
			RepurchaseRate:   0,
			OrderToPayRate:   0,
			NewStudentCount:  0,
			ActiveStudentCnt: 0,
			OrderCount:       0,
			PaidOrderCount:   0,
			PaidUserCount:    0,
			GMV:              0,
			RefundOrderCount: 0,
			RefundAmount:     0,
			NetGMV:           0,
		}})
		return
	}

	start, end, ok := parseDashboardRange(c)
	if !ok {
		return
	}

	data := operationDashboardOverview{
		RangeStart: start.Format("2006-01-02"),
		RangeEnd:   end.AddDate(0, 0, -1).Format("2006-01-02"),
	}

	queryDB := model.DB

	_ = queryDB.Model(&model.User{}).
		Where("role = ?", 1).
		Where("created_at >= ? AND created_at < ?", start, end).
		Count(&data.NewStudentCount).Error

	_ = queryDB.Table("user_lesson_progress").
		Where("last_study_at IS NOT NULL").
		Where("last_study_at >= ? AND last_study_at < ?", start, end).
		Distinct("user_id").
		Count(&data.ActiveStudentCnt).Error

	_ = queryDB.Model(&model.Order{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Count(&data.OrderCount).Error

	_ = queryDB.Model(&model.Order{}).
		Where("status = ?", "paid").
		Where("created_at >= ? AND created_at < ?", start, end).
		Count(&data.PaidOrderCount).Error

	_ = queryDB.Table("orders").
		Where("status = ?", "paid").
		Where("created_at >= ? AND created_at < ?", start, end).
		Distinct("user_id").
		Count(&data.PaidUserCount).Error

	type sumResult struct {
		Total int64 `json:"total"`
	}
	var paidSum sumResult
	_ = queryDB.Model(&model.Order{}).
		Select("COALESCE(SUM(pay_amount), 0) as total").
		Where("status = ?", "paid").
		Where("created_at >= ? AND created_at < ?", start, end).
		Scan(&paidSum).Error
	data.GMV = paidSum.Total

	_ = queryDB.Model(&model.Order{}).
		Where("status = ?", "refunded").
		Where("created_at >= ? AND created_at < ?", start, end).
		Count(&data.RefundOrderCount).Error

	var refundSum sumResult
	_ = queryDB.Model(&model.Order{}).
		Select("COALESCE(SUM(pay_amount), 0) as total").
		Where("status = ?", "refunded").
		Where("created_at >= ? AND created_at < ?", start, end).
		Scan(&refundSum).Error
	data.RefundAmount = refundSum.Total
	data.NetGMV = data.GMV - data.RefundAmount

	if data.PaidUserCount > 0 {
		var repurchaseUsers int64
		repurchaseSub := queryDB.Table("orders").
			Select("user_id").
			Where("status = ?", "paid").
			Where("created_at >= ? AND created_at < ?", start, end).
			Group("user_id").
			Having("COUNT(*) >= 2")
		_ = queryDB.Table("(?) as t", repurchaseSub).Count(&repurchaseUsers).Error
		data.RepurchaseRate = float64(repurchaseUsers) / float64(data.PaidUserCount)
	}

	if data.OrderCount > 0 {
		data.OrderToPayRate = float64(data.PaidOrderCount) / float64(data.OrderCount)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": data})
}

func GetOperationDashboardTrends(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []operationDashboardTrendItem{}})
		return
	}
	start, end, ok := parseDashboardRange(c)
	if !ok {
		return
	}
	queryDB := model.DB

	type dateCountRow struct {
		Date  string `gorm:"column:date"`
		Count int64  `gorm:"column:count"`
	}
	type dateSumRow struct {
		Date  string `gorm:"column:date"`
		Total int64  `gorm:"column:total"`
	}

	newStudentMap := map[string]int64{}
	var newStudents []dateCountRow
	_ = queryDB.Table("users").
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("role = ?", 1).
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("DATE(created_at)").
		Scan(&newStudents).Error
	for _, r := range newStudents {
		newStudentMap[r.Date] = r.Count
	}

	orderMap := map[string]int64{}
	var orders []dateCountRow
	_ = queryDB.Table("orders").
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("DATE(created_at)").
		Scan(&orders).Error
	for _, r := range orders {
		orderMap[r.Date] = r.Count
	}

	paidOrderMap := map[string]int64{}
	var paidOrders []dateCountRow
	_ = queryDB.Table("orders").
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("status = ?", "paid").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("DATE(created_at)").
		Scan(&paidOrders).Error
	for _, r := range paidOrders {
		paidOrderMap[r.Date] = r.Count
	}

	gmvMap := map[string]int64{}
	var gmvRows []dateSumRow
	_ = queryDB.Table("orders").
		Select("DATE(created_at) as date, COALESCE(SUM(pay_amount), 0) as total").
		Where("status = ?", "paid").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("DATE(created_at)").
		Scan(&gmvRows).Error
	for _, r := range gmvRows {
		gmvMap[r.Date] = r.Total
	}

	refundMap := map[string]int64{}
	var refundRows []dateSumRow
	_ = queryDB.Table("orders").
		Select("DATE(created_at) as date, COALESCE(SUM(pay_amount), 0) as total").
		Where("status = ?", "refunded").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("DATE(created_at)").
		Scan(&refundRows).Error
	for _, r := range refundRows {
		refundMap[r.Date] = r.Total
	}

	items := make([]operationDashboardTrendItem, 0)
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		items = append(items, operationDashboardTrendItem{
			Date:         dateStr,
			NewStudents:  newStudentMap[dateStr],
			Orders:       orderMap[dateStr],
			PaidOrders:   paidOrderMap[dateStr],
			GMV:          gmvMap[dateStr],
			RefundAmount: refundMap[dateStr],
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": items})
}

func GetOperationDashboardTopCourses(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []operationDashboardTopCourse{}})
		return
	}
	start, end, ok := parseDashboardRange(c)
	if !ok {
		return
	}

	limit := 10
	if v := c.Query("limit"); v != "" {
		if parsed, err := parsePositiveInt(v, 10, 100); err == nil {
			limit = parsed
		}
	}

	type row struct {
		CourseID       uint64 `gorm:"column:course_id"`
		Title          string `gorm:"column:title"`
		Revenue        int64  `gorm:"column:revenue"`
		PaidOrderCount int64  `gorm:"column:paid_order_count"`
		PaidUserCount  int64  `gorm:"column:paid_user_count"`
	}
	var rows []row
	_ = model.DB.Table("order_items oi").
		Select("c.id as course_id, c.title as title, COALESCE(SUM(oi.price * oi.quantity), 0) as revenue, COUNT(DISTINCT o.id) as paid_order_count, COUNT(DISTINCT o.user_id) as paid_user_count").
		Joins("JOIN orders o ON o.id = oi.order_id").
		Joins("JOIN courses c ON c.id = oi.goods_id").
		Where("o.type = ?", "course").
		Where("o.status = ?", "paid").
		Where("o.created_at >= ? AND o.created_at < ?", start, end).
		Group("c.id, c.title").
		Order("revenue desc, paid_order_count desc, c.id desc").
		Limit(limit).
		Scan(&rows).Error

	out := make([]operationDashboardTopCourse, 0, len(rows))
	for _, r := range rows {
		out = append(out, operationDashboardTopCourse{
			CourseID:       r.CourseID,
			Title:          r.Title,
			Revenue:        r.Revenue,
			PaidOrderCount: r.PaidOrderCount,
			PaidUserCount:  r.PaidUserCount,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": out})
}

func GetOperationDashboardTopTeachers(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []operationDashboardTopTeacher{}})
		return
	}
	start, end, ok := parseDashboardRange(c)
	if !ok {
		return
	}

	limit := 10
	if v := c.Query("limit"); v != "" {
		if parsed, err := parsePositiveInt(v, 10, 100); err == nil {
			limit = parsed
		}
	}

	type row struct {
		TeacherID      uint64 `gorm:"column:teacher_id"`
		Name           string `gorm:"column:name"`
		Revenue        int64  `gorm:"column:revenue"`
		PaidOrderCount int64  `gorm:"column:paid_order_count"`
		PaidUserCount  int64  `gorm:"column:paid_user_count"`
	}
	var rows []row
	_ = model.DB.Table("order_items oi").
		Select("t.id as teacher_id, t.name as name, COALESCE(SUM(oi.price * oi.quantity), 0) as revenue, COUNT(DISTINCT o.id) as paid_order_count, COUNT(DISTINCT o.user_id) as paid_user_count").
		Joins("JOIN orders o ON o.id = oi.order_id").
		Joins("JOIN courses c ON c.id = oi.goods_id").
		Joins("JOIN teachers t ON t.id = c.teacher_id").
		Where("o.type = ?", "course").
		Where("o.status = ?", "paid").
		Where("o.created_at >= ? AND o.created_at < ?", start, end).
		Group("t.id, t.name").
		Order("revenue desc, paid_order_count desc, t.id desc").
		Limit(limit).
		Scan(&rows).Error

	out := make([]operationDashboardTopTeacher, 0, len(rows))
	for _, r := range rows {
		out = append(out, operationDashboardTopTeacher{
			TeacherID:      r.TeacherID,
			Name:           r.Name,
			Revenue:        r.Revenue,
			PaidOrderCount: r.PaidOrderCount,
			PaidUserCount:  r.PaidUserCount,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": out})
}

func parsePositiveInt(raw string, def int, max int) (int, error) {
	var v int
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return def, errors.New("invalid int")
		}
		v = v*10 + int(raw[i]-'0')
	}
	if v <= 0 {
		return def, errors.New("invalid int")
	}
	if v > max {
		return max, nil
	}
	return v, nil
}
