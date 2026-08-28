package notepad

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (a *App) handleNotePage(c *gin.Context) {
	id := c.Param("id")
	logDebug("open note page: %s from %s", id, c.ClientIP())
	note, content, found, err := a.store.getNoteContent(id)
	if err != nil {
		logError("open note page failed: %s, err: %v", id, err)
		a.renderError(c, http.StatusInternalServerError, "服务器错误", "读取笔记失败，请稍后再试。", nil)
		return
	}
	if !found {
		logDebug("open note page: note %s not found", id)
		a.renderError(c, http.StatusNotFound, "笔记不存在", "该笔记可能已被删除或不存在。", &Action{Href: "/", Text: "返回首页"})
		return
	}

	notes, err := a.store.listNotes()
	if err != nil {
		logError("list notes for page failed: %v", err)
		a.renderError(c, http.StatusInternalServerError, "服务器错误", "读取笔记列表失败，请稍后再试。", nil)
		return
	}

	c.HTML(http.StatusOK, "note.ejs", gin.H{
		"AppName":      appName,
		"Note":         content,
		"Notes":        notes,
		"CurrentID":    id,
		"CurrentTitle": note.Title,
	})
}

func (a *App) handleNoteContent(c *gin.Context) {
	id := c.Param("id")
	note, content, found, err := a.store.getNoteContent(id)
	if err != nil {
		logError("get note content failed: %s, err: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal Server Error"})
		return
	}
	if !found {
		logDebug("get note content: note %s not found", id)
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"id":        note.ID,
		"title":     note.Title,
		"content":   content,
		"createdAt": note.CreatedAt,
		"updatedAt": effectiveUpdatedAt(note),
	})
}

func (a *App) handleNotesList(c *gin.Context) {
	notes, err := a.store.listNotes()
	if err != nil {
		logError("list notes failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal Server Error"})
		return
	}

	logDebug("api list notes: %d items", len(notes))
	c.JSON(http.StatusOK, notes)
}

func (a *App) handleCreateNote(c *gin.Context) {
	var payload struct {
		Title string `json:"title" form:"title"`
	}
	_ = c.ShouldBind(&payload)

	note, err := a.store.createNote(payload.Title)
	if err != nil {
		logError("create note failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal Server Error"})
		return
	}

	logInfo("note created, id=%s, title=%s", note.ID, note.Title)
	c.JSON(http.StatusOK, note)
}

func (a *App) handleDeleteNote(c *gin.Context) {
	id := c.Param("id")
	ok, err := a.store.deleteNote(id)
	if err != nil {
		logError("delete note %s failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal Server Error"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Not found"})
		return
	}

	logInfo("note deleted, id=%s", id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (a *App) handleRenameNote(c *gin.Context) {
	var payload struct {
		Title string `json:"title" form:"title"`
	}
	_ = c.ShouldBind(&payload)

	id := c.Param("id")
	title, ok, err := a.store.renameNote(id, payload.Title)
	if err != nil {
		logError("rename note %s failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal Server Error"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Not found"})
		return
	}

	logInfo("note renamed, id=%s, title=%s", id, title)
	c.JSON(http.StatusOK, gin.H{"success": true, "title": title})
}

func (a *App) handleSaveNote(c *gin.Context) {
	id := c.Param("id")
	ok, err := a.store.saveNoteContent(id, c.PostForm("note"))
	if err != nil {
		logError("save note %s failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal Server Error"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Not found"})
		return
	}

	logInfo("note saved, id=%s", id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (a *App) handleUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "No file"})
		return
	}
	if file.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "File too large"})
		return
	}

	if err := os.MkdirAll(a.store.uploadsDir, 0o755); err != nil {
		logError("create uploads dir failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal Server Error"})
		return
	}

	originalName := filepath.Base(file.Filename)
	storedName := uuid.NewString() + strings.ToLower(filepath.Ext(originalName))
	destination := filepath.Join(a.store.uploadsDir, storedName)
	if err := c.SaveUploadedFile(file, destination); err != nil {
		logError("save uploaded file failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Upload failed"})
		return
	}

	noteID := c.PostForm("noteId")
	if noteID != "" {
		if err := a.store.addAttachment(noteID, storedName); err != nil {
			_ = os.Remove(destination)
			logError("add attachment to note %s failed: %v", noteID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal Server Error"})
			return
		}
	}

	logInfo("file uploaded, original=%s, stored=%s, size=%d", originalName, storedName, file.Size)

	url := "/uploads/" + storedName
	markdown := fmt.Sprintf("[%s](%s)", originalName, url)
	if imagePattern.MatchString(originalName) {
		markdown = fmt.Sprintf("![%s](%s)", originalName, url)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"url":          url,
		"filename":     storedName,
		"originalname": originalName,
		"markdown":     markdown,
	})
}