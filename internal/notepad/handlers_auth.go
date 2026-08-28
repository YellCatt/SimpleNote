package notepad

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (a *App) handleLoginPage(c *gin.Context) {
	if a.claimsFromContext(c) != nil {
		logDebug("login page: already authenticated, redirect to /")
		c.Redirect(http.StatusFound, "/")
		return
	}

	logDebug("login page from %s", c.ClientIP())
	c.HTML(http.StatusOK, "login.ejs", gin.H{
		"AppName": appName,
	})
}

func (a *App) handleLogin(c *gin.Context) {
	password := c.PostForm("password")
	remember := c.PostForm("rememberMe") == "on"
	clientIP := c.ClientIP()
	if password != a.password {
		logInfo("login failed, wrong password from %s", clientIP)
		a.renderError(c, http.StatusUnauthorized, "密码错误", "请检查密码后再试。", &Action{Href: "/login", Text: "返回登录页"})
		return
	}

	token, err := a.signAuthToken(remember)
	if err != nil {
		logError("login failed, token generation error: %v", err)
		a.renderError(c, http.StatusInternalServerError, "登录失败", "登录状态生成失败，请稍后再试。", &Action{Href: "/login", Text: "返回登录页"})
		return
	}

	logInfo("login success from %s, remember=%v", clientIP, remember)

	cookie := &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
	}
	if remember {
		cookie.MaxAge = rememberCookieMaxAgeSecond
		cookie.Expires = time.Now().Add(rememberCookieMaxAgeSecond * time.Second)
	}
	http.SetCookie(c.Writer, cookie)
	c.Redirect(http.StatusFound, "/")
}

func (a *App) handleLogout(c *gin.Context) {
	logInfo("logout from %s", c.ClientIP())
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
	})
	c.Redirect(http.StatusFound, "/login")
}

func (a *App) handleHome(c *gin.Context) {
	id, err := a.store.ensureLandingNote()
	if err != nil {
		logError("ensure landing note failed: %v", err)
		a.renderError(c, http.StatusInternalServerError, "服务器错误", "初始化笔记失败，请稍后再试。", nil)
		return
	}

	logDebug("home redirect to note: %s", id)
	c.Redirect(http.StatusFound, "/note/"+id)
}