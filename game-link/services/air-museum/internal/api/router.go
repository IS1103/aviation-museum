package api

import (
	"github.com/gin-gonic/gin"
)

// SetupRouter 設定 Gin 路由（HTTP API）；目前無對外路由，僅佔用設定檔中的 http_port。
func SetupRouter() *gin.Engine {
	return gin.Default()
}
