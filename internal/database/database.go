package database

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sportss/internal/models"
)

// Connect abre SQLite sin CGO y aplica migraciones idempotentes.
func Connect(path string, appEnv string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("preparar directorio de base de datos: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", filepath.ToSlash(path))
	logLevel := logger.Warn
	if appEnv == "development" {
		logLevel = logger.Error
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("abrir SQLite: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1) // SQLite: una conexión de escritura evita locks intermitentes.
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	if err := migrate(db); err != nil {
		return nil, err
	}
	slog.Info("base de datos SQLite lista", "path", path)
	return db, nil
}

func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.Court{},
		&models.CourtPhoto{},
		&models.Booking{},
		&models.Payment{},
		&models.Invoice{},
		&models.Review{},
		&models.Notification{},
	); err != nil {
		return fmt.Errorf("migraciones: %w", err)
	}

	// Restricción única parcial: solo bloquea horas de reservas activas.
	// Las reservas canceladas o expiradas liberan inmediatamente el horario.
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_active_booking_slot
		 ON bookings(court_id, date, start_time)
		 WHERE status IN ('pendiente_pago','confirmada','completada')`,
		`CREATE INDEX IF NOT EXISTS idx_booking_status_date ON bookings(status, date)`,
		`CREATE INDEX IF NOT EXISTS idx_booking_created_at ON bookings(created_at)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("crear índice: %w", err)
		}
	}
	return nil
}

// AutoCompletePast finaliza reservas confirmadas cuya hora final ya pasó.
func AutoCompletePast(db *gorm.DB, now time.Time) ([]models.Booking, error) {
	var candidates []models.Booking
	if err := db.Where("status = ? AND date <= ?", models.BookingConfirmed, now).Find(&candidates).Error; err != nil {
		return nil, err
	}
	var completed []models.Booking
	for _, b := range candidates {
		end, err := time.ParseInLocation("2006-01-02 15:04", b.Date.Format("2006-01-02")+" "+b.EndTime, now.Location())
		if err != nil || end.After(now) {
			continue
		}
		res := db.Model(&models.Booking{}).
			Where("id = ? AND status = ?", b.ID, models.BookingConfirmed).
			Updates(map[string]any{"status": models.BookingCompleted, "completed_at": now})
		if res.Error == nil && res.RowsAffected > 0 {
			b.Status = models.BookingCompleted
			completed = append(completed, b)
		}
	}
	return completed, nil
}
