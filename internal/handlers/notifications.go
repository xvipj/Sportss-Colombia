package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"sportss/internal/middleware"
	"sportss/internal/services"
)

func (a *App) APINotifications(c *gin.Context) {
	user := middleware.CurrentUser(c)
	items, err := a.notifications.List(c.Request.Context(), user.ID, 30)
	if err != nil {
		a.writeError(c, err)
		return
	}
	unread, _ := a.notifications.Unread(c.Request.Context(), user.ID)
	c.JSON(http.StatusOK, gin.H{"notifications": items, "unread": unread})
}

func (a *App) APIMarkNotificationRead(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		a.writeError(c, services.NotFound("Notificación no encontrada."))
		return
	}
	if err := a.notifications.MarkRead(c.Request.Context(), uint(id), user.ID); err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) APIMarkAllRead(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if err := a.notifications.MarkAllRead(c.Request.Context(), user.ID); err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NotificationStream mantiene una conexión SSE con el navegador.
func (a *App) NotificationStream(c *gin.Context) {
	user := middleware.CurrentUser(c)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch, unsubscribe := a.notifications.Subscribe(user.ID)
	defer unsubscribe()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	fmt.Fprint(c.Writer, "retry: 4000\n\n")
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case n := <-ch:
			payload, _ := json.Marshal(n)
			fmt.Fprintf(c.Writer, "event: notification\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		}
	}
}
