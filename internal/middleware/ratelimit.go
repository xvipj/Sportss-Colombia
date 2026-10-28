package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type attemptWindow struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	lastGC   time.Time
}

// LoginRateLimit limita intentos por IP para dificultar ataques de fuerza bruta.
func LoginRateLimit(maxAttempts int, window time.Duration) gin.HandlerFunc {
	w := &attemptWindow{attempts: make(map[string][]time.Time), lastGC: time.Now()}
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		w.mu.Lock()
		if now.Sub(w.lastGC) > 10*time.Minute {
			for key, list := range w.attempts {
				if len(list) == 0 || now.Sub(list[len(list)-1]) > window {
					delete(w.attempts, key)
				}
			}
			w.lastGC = now
		}
		list := w.attempts[ip]
		valid := list[:0]
		for _, t := range list {
			if now.Sub(t) < window {
				valid = append(valid, t)
			}
		}
		if len(valid) >= maxAttempts {
			w.attempts[ip] = valid
			w.mu.Unlock()
			c.Header("Retry-After", "600")
			if IsAPI(c) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Demasiados intentos. Espera unos minutos."})
			} else {
				c.String(http.StatusTooManyRequests, "Demasiados intentos de acceso. Espera unos minutos e inténtalo de nuevo.")
			}
			return
		}
		valid = append(valid, now)
		w.attempts[ip] = valid
		w.mu.Unlock()
		c.Next()
	}
}
