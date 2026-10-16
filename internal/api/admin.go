package api

import (
	"net/http"
	"strconv"
	"strings"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type adminUserListItem struct {
	ID        uint64 `json:"id"`
	Phone     string `json:"phone"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Status    int8   `json:"status"`
	CreatedAt string `json:"created_at"`
}

type teacherUpsertRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password"`
	Name      string `json:"name" binding:"required"`
	Mobile    string `json:"mobile"`
	Avatar    string `json:"avatar"`
	Bio       string `json:"bio"`
	Intro     string `json:"intro"`
	Specialty string `json:"specialty"`
	Status    int8   `json:"status"`
}

type adminUpsertRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password"`
	Name     string `json:"name" binding:"required"`
	Mobile   string `json:"mobile"`
	RoleCode string `json:"role_code" binding:"required"`
	Status   int8   `json:"status"`
}

func currentRoleCode(c *gin.Context) string {
	roleCode, _ := c.Get("roleCode")
	if value, ok := roleCode.(string); ok {
		return value
	}
	return ""
}

func requireSuperAdmin(c *gin.Context) bool {
	if currentRoleCode(c) != model.AdminRoleSuper {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "仅超级管理员可操作"})
		return false
	}
	return true
}

func requireAdminAccount(c *gin.Context) bool {
	accountType, _ := c.Get("accountType")
	if accountType != accountTypeAdmin {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "仅管理员可操作"})
		return false
	}
	return true
}

// AdminGetUsers 获取学员列表
func AdminGetUsers(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []adminUserListItem{}})
		return
	}
	var users []adminUserListItem
	query := model.DB.Table("users").
		Select("users.id, users.phone, users.nickname, users.avatar, users.status, DATE_FORMAT(users.created_at, '%Y-%m-%d %H:%i:%s') as created_at").
		Where("users.role = ?", 1).
		Order("users.created_at desc")
	if isTeacher, teacherID := currentTeacherScope(c); isTeacher {
		query = query.Joins("JOIN user_courses uc ON uc.user_id = users.id").
			Joins("JOIN courses c ON c.id = uc.course_id").
			Where("c.teacher_id = ?", teacherID).
			Group("users.id, users.phone, users.nickname, users.avatar, users.status, users.created_at")
	}
	if err := query.Scan(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取用户列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": users,
	})
}

// GetTeacherList 获取讲师列表
func GetTeacherList(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.Teacher{}})
		return
	}
	var teachers []model.Teacher
	query := model.DB.Order("created_at desc")
	if scoped, id := currentTeacherScope(c); scoped {
		query = query.Where("id = ?", id)
	}
	if err := query.Find(&teachers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取讲师列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": teachers})
}

// CreateTeacher 创建讲师账号
func CreateTeacher(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	var req teacherUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if !validateStaffAccount(c, accountTypeTeacher, 0, req.Username, req.Mobile, req.Password, req.Status, true) {
		return
	}
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "密码加密失败: " + err.Error()})
		return
	}
	teacher := model.Teacher{
		Username:     req.Username,
		PasswordHash: string(hashedPwd),
		Name:         req.Name,
		Mobile:       req.Mobile,
		Avatar:       req.Avatar,
		Bio:          req.Bio,
		Intro:        req.Intro,
		Specialty:    req.Specialty,
		Status:       req.Status,
	}
	if err := model.DB.Create(&teacher).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建讲师失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": teacher})
}

// UpdateTeacher 更新讲师账号
func UpdateTeacher(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "讲师ID无效"})
		return
	}
	var req teacherUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if !validateStaffAccount(c, accountTypeTeacher, id, req.Username, req.Mobile, req.Password, req.Status, false) {
		return
	}
	updates := map[string]interface{}{
		"username":  req.Username,
		"name":      req.Name,
		"mobile":    req.Mobile,
		"avatar":    req.Avatar,
		"bio":       req.Bio,
		"intro":     req.Intro,
		"specialty": req.Specialty,
		"status":    req.Status,
	}
	if req.Password != "" {
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		updates["password_hash"] = string(hashedPwd)
	}
	result := model.DB.Model(&model.Teacher{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新讲师失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "讲师不存在"})
		return
	}
	var teacher model.Teacher
	_ = model.DB.Where("id = ?", id).First(&teacher).Error
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": teacher})
}

// GetAdminList 获取管理员列表
func GetAdminList(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	var admins []model.Admin
	if err := model.DB.Order("created_at desc").Find(&admins).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取管理员列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": admins})
}

// CreateAdmin 创建管理员
func CreateAdmin(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	var req adminUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.RoleCode != model.AdminRoleSuper && req.RoleCode != model.AdminRoleNormal {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "管理员角色无效"})
		return
	}
	if !validateStaffAccount(c, accountTypeAdmin, 0, req.Username, req.Mobile, req.Password, req.Status, true) {
		return
	}
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "密码加密失败: " + err.Error()})
		return
	}
	admin := model.Admin{
		Username:     req.Username,
		PasswordHash: string(hashedPwd),
		Name:         req.Name,
		Mobile:       req.Mobile,
		RoleCode:     req.RoleCode,
		Status:       req.Status,
	}
	if err := model.DB.Create(&admin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建管理员失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": admin})
}

// UpdateAdmin 更新管理员
func UpdateAdmin(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "管理员ID无效"})
		return
	}
	var req adminUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.RoleCode != model.AdminRoleSuper && req.RoleCode != model.AdminRoleNormal {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "管理员角色无效"})
		return
	}
	if !validateStaffAccount(c, accountTypeAdmin, id, req.Username, req.Mobile, req.Password, req.Status, false) {
		return
	}
	updates := map[string]interface{}{
		"username":  req.Username,
		"name":      req.Name,
		"mobile":    req.Mobile,
		"role_code": req.RoleCode,
		"status":    req.Status,
	}
	if req.Password != "" {
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		updates["password_hash"] = string(hashedPwd)
	}
	result := model.DB.Model(&model.Admin{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新管理员失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "管理员不存在"})
		return
	}
	var admin model.Admin
	_ = model.DB.Where("id = ?", id).First(&admin).Error
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": admin})
}

func validateStaffAccount(c *gin.Context, accountType string, id uint64, username, mobile, password string, status int8, creating bool) bool {
	if strings.TrimSpace(username) == "" || username != strings.TrimSpace(username) || mobile != strings.TrimSpace(mobile) {
		c.JSON(400, gin.H{"code": 400, "msg": "账号不能为空，账号和手机号首尾不能有空格"})
		return false
	}
	if status != 0 && status != 1 {
		c.JSON(400, gin.H{"code": 400, "msg": "账号状态只能为启用或禁用"})
		return false
	}
	if (creating || password != "") && (len(password) < 8 || len(password) > 72 || strings.TrimSpace(password) == "") {
		c.JSON(400, gin.H{"code": 400, "msg": "请设置 8 至 72 字节的账号密码"})
		return false
	}
	for _, item := range []struct{ Table, Type string }{{"admins", accountTypeAdmin}, {"teachers", accountTypeTeacher}} {
		query := model.DB.Table(item.Table).Where("(username = ? OR mobile = ?)", username, username)
		if mobile != "" {
			query = model.DB.Table(item.Table).Where("(username IN ? OR mobile IN ?)", []string{username, mobile}, []string{username, mobile})
		}
		if item.Type == accountType && id > 0 {
			query = query.Where("id <> ?", id)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			c.JSON(500, gin.H{"code": 500, "msg": "查询账号唯一性失败: " + err.Error()})
			return false
		}
		if count > 0 {
			c.JSON(409, gin.H{"code": 409, "msg": "账号或手机号已被其他管理员或讲师使用"})
			return false
		}
	}
	return true
}

// AdminGetCourses 获取全部课程（后台）
func AdminGetCourses(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.Course{}})
		return
	}

	var courses []model.Course
	query := model.DB.Order("created_at desc")
	if isTeacher, teacherID := currentTeacherScope(c); isTeacher {
		query = query.Where("teacher_id = ?", teacherID)
	}
	if err := query.Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取课程列表失败"})
		return
	}

	normalizeCoursePrices(courses)
	fillTeacherNames(courses)

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": courses})
}
