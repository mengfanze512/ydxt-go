package api

import (
	"net/http"
	"strings"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

func AdminGetCheckins(c *gin.Context) {
	if !requireAdminAccount(c) || !deliveryDB(c) {
		return
	}
	page, size, ok := teacherPagination(c)
	if !ok {
		return
	}
	query := model.DB.Table("user_checkins ch").
		Joins("LEFT JOIN users u ON u.id = ch.user_id")
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		query = query.Where("(u.nickname LIKE ? OR u.phone LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	for _, filter := range []struct{ Param, Operator string }{{"start_date", ">="}, {"end_date", "<="}} {
		if date := c.Query(filter.Param); date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				c.JSON(400, gin.H{"code": 400, "msg": filter.Param + " 日期格式必须为 YYYY-MM-DD"})
				return
			}
			query = query.Where("ch.checkin_date "+filter.Operator+" ?", date)
		}
	}
	if start, end := c.Query("start_date"), c.Query("end_date"); start != "" && end != "" && start > end {
		c.JSON(400, gin.H{"code": 400, "msg": "开始日期不能晚于结束日期"})
		return
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "获取打卡记录数量失败: " + err.Error()})
		return
	}
	type item struct {
		model.UserCheckin
		Nickname string `json:"nickname"`
		Phone    string `json:"phone"`
	}
	items := []item{}
	if err := query.Select("ch.*, COALESCE(u.nickname, '') AS nickname, COALESCE(u.phone, '') AS phone").
		Order("ch.checkin_date DESC, ch.id DESC").Offset((page - 1) * size).Limit(size).Scan(&items).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "获取打卡记录失败: " + err.Error()})
		return
	}
	for i := range items {
		items[i].CheckinDate = normalizeCheckinDate(items[i].CheckinDate)
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"items": items, "total": total}})
}

func AdminDeleteCheckin(c *gin.Context) {
	if !requireAdminAccount(c) || !deliveryDB(c) {
		return
	}
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	result := model.DB.Where("id = ?", id).Delete(&model.UserCheckin{})
	if result.Error != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "删除打卡记录失败: " + result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"code": 404, "msg": "打卡记录不存在或已被删除"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "打卡记录已删除"})
}

// GetPracticeRecords 获取学员AI练习记录
func GetPracticeRecords(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.PracticeRecord{}})
		return
	}
	var records []model.PracticeRecord
	// 后台管理端可以查所有
	if err := model.DB.Order("created_at desc").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取练习记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": records,
	})
}
