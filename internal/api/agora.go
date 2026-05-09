package api

import (
	"fmt"
	"net/http"
	"time"
	"ydxt-go/internal/config"

	rtctokenbuilder "github.com/AgoraIO/Tools/DynamicKey/AgoraDynamicKey/go/src/rtctokenbuilder2"
	"github.com/gin-gonic/gin"
)

type TokenRequest struct {
	ChannelName string `json:"channel_name" binding:"required"`
	Uid         uint32 `json:"uid"`
	Role        uint16 `json:"role"` // 1=主播, 2=观众
}

// GenerateRTCToken 声网音视频进房鉴权 Token 生成
func GenerateRTCToken(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数有误"})
		return
	}

	appID := config.GlobalConfig.Agora.AppID
	appCertificate := config.GlobalConfig.Agora.AppCertificate

	if appID == "" || appCertificate == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "声网配置未初始化"})
		return
	}

	// Token 有效期设为 2 小时
	expireTimeInSeconds := uint32(7200)
	currentTimestamp := uint32(time.Now().Unix())
	privilegeExpiredTs := currentTimestamp + expireTimeInSeconds

	// 转换为 rtctokenbuilder2 的角色定义
	var role rtctokenbuilder.Role
	if req.Role == 1 {
		role = rtctokenbuilder.RolePublisher
	} else {
		role = rtctokenbuilder.RoleSubscriber
	}

	// 生成 Token (rtctokenbuilder2 的方法签名有所不同，通常多了一个 tokenExpire 参数)
	// 在 rtctokenbuilder2 中，通常使用 BuildTokenWithUid
	token, err := rtctokenbuilder.BuildTokenWithUid(
		appID, appCertificate, req.ChannelName, req.Uid,
		role, privilegeExpiredTs, privilegeExpiredTs,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": fmt.Sprintf("Token 生成失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"token": token,
		},
	})
}
