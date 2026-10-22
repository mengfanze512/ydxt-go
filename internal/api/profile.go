package api

import (
	"strings"
	"time"
	"unicode/utf8"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
)

func UpdateMyProfile(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	value, _ := c.Get("accountIdentity")
	identity, ok := value.(*utils.AccountIdentity)
	if !ok {
		c.JSON(401, gin.H{"code": 401, "msg": "账号身份缺失，请重新登录"})
		return
	}
	var req struct {
		Nickname *string `json:"nickname"`
		Avatar   *string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "资料参数无效: " + err.Error()})
		return
	}
	updates := map[string]any{}
	table, nameColumn := "", "name"
	switch identity.AccountType {
	case "user":
		table, nameColumn = "users", "nickname"
	case "teacher":
		table = "teachers"
	case "admin":
		table = "admins"
	default:
		c.JSON(403, gin.H{"code": 403, "msg": "当前账号不支持修改资料"})
		return
	}
	if req.Nickname != nil {
		name := strings.TrimSpace(*req.Nickname)
		if name == "" || utf8.RuneCountInString(name) > 32 {
			c.JSON(400, gin.H{"code": 400, "msg": "用户名不能为空，且不能超过32个字符"})
			return
		}
		updates[nameColumn] = name
	}
	if req.Avatar != nil {
		if identity.AccountType == "admin" {
			c.JSON(400, gin.H{"code": 400, "msg": "管理员账号暂不支持设置头像"})
			return
		}
		avatar := strings.TrimSpace(*req.Avatar)
		if !strings.HasPrefix(avatar, "cloud://") || len(avatar) > 255 {
			c.JSON(400, gin.H{"code": 400, "msg": "头像必须是已上传的微信云文件，地址不能超过255字节"})
			return
		}
		updates["avatar"] = avatar
	}
	if len(updates) == 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "请提供要修改的用户名或头像"})
		return
	}
	updates["updated_at"] = time.Now()
	if err := model.DB.Table(table).Where("id = ?", identity.ID).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "保存个人资料失败: " + err.Error()})
		return
	}
	profile, err := utils.LoadAccount(identity.AccountType, identity.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "读取更新后的资料失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": profile})
}
