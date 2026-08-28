package notepad

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func (a *App) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := a.claimsFromContext(c)
		if claims == nil {
			logDebug("auth required but no valid token from %s, path=%s", c.ClientIP(), c.Request.URL.Path)
			if wantsJSON(c) {
				c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
			} else {
				c.Redirect(http.StatusFound, "/login")
			}
			c.Abort()
			return
		}

		logDebug("auth passed from %s, path=%s", c.ClientIP(), c.Request.URL.Path)
		c.Set("user", claims)
		c.Next()
	}
}

func (a *App) claimsFromContext(c *gin.Context) jwt.MapClaims {
	cookie, err := c.Request.Cookie(authCookieName)
	if err != nil {
		logDebug("no auth cookie from %s", c.ClientIP())
		return nil
	}

	return a.verifyAuthToken(cookie.Value)
}

func (a *App) verifyAuthToken(tokenString string) jwt.MapClaims {
	if tokenString == "" {
		return nil
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		logDebug("token verification failed: %v", err)
		return nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		logDebug("token claims type assertion failed")
		return nil
	}

	logDebug("token verified, exp=%v", claims["exp"])
	return claims
}

func (a *App) signAuthToken(remember bool) (string, error) {
	ttl := a.sessionTTL
	if remember {
		ttl = a.rememberTTL
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role": "user",
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
	})
	return token.SignedString(a.jwtSecret)
}