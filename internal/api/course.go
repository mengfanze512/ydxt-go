package api

import (
	"net/http"
	"yuedi_edu/internal/model"

	"github.com/gin-gonic/gin"
)

// GetCourseList 获取公开课程列表 (支持前台分页和分类)
func GetCourseList(c *gin.Context) {
	category := c.Query("category") // 例如: "入门", "进阶", "考级"
	level := c.Query("level")       // 例如: "system", "1v1", "vip"

	var courses []model.Course
	query := model.DB.Where("status = ?", 1).Where("is_deleted = ?", 0)

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

	// 为前台组装讲师名字(简单处理，实际应联表查询)
	for i, course := range courses {
		var teacher model.User
		if err := model.DB.Where("id = ?", course.TeacherID).First(&teacher).Error; err == nil {
			courses[i].TeacherName = teacher.Nickname
		} else {
			courses[i].TeacherName = "特邀讲师"
		}
	}

	// 如果没有数据，返回一些 mock 数据用于前端演示
	if len(courses) == 0 {
		courses = []model.Course{
			{ID: 1, Title: "零基础竹笛入门 30 天精通", Cover: "https://img.yzcdn.cn/vant/cat.jpeg", Price: 199.00, TeacherName: "张三名师", Category: "入门", Level: "system", StudentCount: 1280},
			{ID: 2, Title: "葫芦丝考级冲刺班 (七级-十级)", Cover: "https://img.yzcdn.cn/vant/cat.jpeg", Price: 299.00, TeacherName: "李四名师", Category: "考级", Level: "system", StudentCount: 860},
			{ID: 3, Title: "古筝名曲《高山流水》精讲", Cover: "https://img.yzcdn.cn/vant/cat.jpeg", Price: 99.00, TeacherName: "王五名师", Category: "进阶", Level: "system", StudentCount: 320},
			{ID: 4, Title: "二胡 1对1 在线私教陪练", Cover: "https://img.yzcdn.cn/vant/cat.jpeg", Price: 300.00, TeacherName: "特邀讲师", Category: "进阶", Level: "1v1", StudentCount: 50},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": courses,
	})
}
