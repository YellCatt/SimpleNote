package notepad

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

func newRouter(app *App, assetsDir string) *gin.Engine {
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(requestLogger())
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logError("panic recovered: %v", recovered)
		app.renderError(c, http.StatusInternalServerError, "服务器错误", "服务遇到问题，请稍后再试。", nil)
	}))
	router.MaxMultipartMemory = maxUploadSize

	if err := router.SetTrustedProxies(nil); err != nil {
		logError("set trusted proxies failed: %v", err)
	}

	router.LoadHTMLGlob(filepath.Join(assetsDir, "views", "*"))
	router.Static("/css", filepath.Join(assetsDir, "public", "css"))
	router.Static("/image", filepath.Join(assetsDir, "public", "image"))
	router.Static("/svg", filepath.Join(assetsDir, "public", "svg"))
	router.Static("/js", filepath.Join(assetsDir, "public", "js"))
	router.Static("/uploads", app.store.uploadsDir)
	router.StaticFile("/favicon.ico", filepath.Join(assetsDir, "public", "image", "favicon.png"))

	router.GET("/login", app.handleLoginPage)
	router.POST("/login", app.handleLogin)
	router.GET("/logout", app.handleLogout)
	logDebug("public routes registered: /login (GET/POST), /logout (GET)")

	auth := router.Group("/")
	auth.Use(app.requireAuth())
	{
		auth.GET("/", app.handleHome)
		auth.GET("/note/:id", app.handleNotePage)
		auth.GET("/api/notes/:id/content", app.handleNoteContent)
		auth.GET("/api/notes", app.handleNotesList)
		auth.POST("/api/notes", app.handleCreateNote)
		auth.DELETE("/api/notes/:id", app.handleDeleteNote)
		auth.PUT("/api/notes/:id/title", app.handleRenameNote)
		auth.POST("/save/:id", app.handleSaveNote)
		auth.POST("/api/upload", app.handleUpload)
	}
	logDebug("auth routes registered: 9 endpoints")

	router.NoRoute(func(c *gin.Context) {
		logDebug("404 not found: %s %s", c.Request.Method, c.Request.URL.Path)
		app.renderError(c, http.StatusNotFound, "页面不存在", "你访问的地址不存在或已被移除。", nil)
	})
	router.NoMethod(func(c *gin.Context) {
		logDebug("405 method not allowed: %s %s", c.Request.Method, c.Request.URL.Path)
		app.renderError(c, http.StatusMethodNotAllowed, "请求方式不支持", "请检查请求方式后重试。", nil)
	})

	logDebug("router initialized, static dir: %s", assetsDir)
	return router
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			path += "?" + c.Request.URL.RawQuery
		}
		logInfo("[%s] %s %d %v %s", clientIP, method, status, latency, path)
	}
}