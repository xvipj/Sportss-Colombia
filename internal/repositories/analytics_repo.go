package repositories

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"sportss/internal/models"
)

type analyticsRepository struct{ db *gorm.DB }

func NewAnalyticsRepository(db *gorm.DB) AnalyticsRepository {
	return &analyticsRepository{db: db}
}

func (r *analyticsRepository) Summary(ctx context.Context, ownerID *uint, from, to, now time.Time) (*AnalyticsSummary, error) {
	out := &AnalyticsSummary{}
	db := r.db.WithContext(ctx)
	fromDate, toDate := dateOnly(from), dateOnly(to)

	baseWhere := " WHERE b.date >= ? AND b.date <= ?"
	baseArgs := []any{fromDate, toDate}
	if ownerID != nil {
		baseWhere += " AND c.owner_id = ?"
		baseArgs = append(baseArgs, *ownerID)
	}

	var rev struct {
		Total      int64
		Commission int64
		Net        int64
		Bookings   int64
		Confirmed  int64
		Cancelled  int64
	}
	revSQL := `SELECT
		COALESCE(SUM(CASE WHEN b.status IN ('confirmada','completada') THEN b.total_amount ELSE 0 END),0) AS total,
		COALESCE(SUM(CASE WHEN b.status IN ('confirmada','completada') THEN b.commission_amount ELSE 0 END),0) AS commission,
		COALESCE(SUM(CASE WHEN b.status IN ('confirmada','completada') THEN b.net_amount ELSE 0 END),0) AS net,
		COUNT(*) AS bookings,
		COALESCE(SUM(CASE WHEN b.status = 'confirmada' THEN 1 ELSE 0 END),0) AS confirmed,
		COALESCE(SUM(CASE WHEN b.status = 'cancelada' THEN 1 ELSE 0 END),0) AS cancelled
		FROM bookings b JOIN courts c ON c.id = b.court_id AND c.deleted_at IS NULL` + baseWhere
	if err := db.Raw(revSQL, baseArgs...).Scan(&rev).Error; err != nil {
		return nil, fmt.Errorf("resumen de ingresos: %w", err)
	}
	out.Revenue = RevenueSummary{
		Total: rev.Total, Commission: rev.Commission, Net: rev.Net,
		Bookings: rev.Bookings, Confirmed: rev.Confirmed, Cancelled: rev.Cancelled,
	}

	dailySQL := `SELECT DATE(b.date) AS date,
		COUNT(*) AS count,
		COALESCE(SUM(CASE WHEN b.status IN ('confirmada','completada') THEN b.hours ELSE 0 END),0) AS hours,
		COALESCE(SUM(CASE WHEN b.status IN ('confirmada','completada') THEN b.total_amount ELSE 0 END),0) AS revenue
		FROM bookings b JOIN courts c ON c.id = b.court_id AND c.deleted_at IS NULL` + baseWhere + `
		GROUP BY DATE(b.date) ORDER BY DATE(b.date) ASC`
	if err := db.Raw(dailySQL, baseArgs...).Scan(&out.Daily).Error; err != nil {
		return nil, fmt.Errorf("serie diaria: %w", err)
	}
	for i := range out.Daily {
		if t, err := time.Parse("2006-01-02", out.Daily[i].Date); err == nil {
			out.Daily[i].Label = t.Format("02 Jan")
		}
	}

	courtSQL := `SELECT c.id AS court_id, c.name AS name,
		COUNT(*) AS bookings,
		COALESCE(SUM(CASE WHEN b.status IN ('confirmada','completada') THEN b.hours ELSE 0 END),0) AS hours,
		COALESCE(SUM(CASE WHEN b.status IN ('confirmada','completada') THEN b.total_amount ELSE 0 END),0) AS revenue,
		COALESCE(SUM(CASE WHEN b.status IN ('confirmada','completada') THEN b.commission_amount ELSE 0 END),0) AS commission,
		COALESCE(SUM(CASE WHEN b.status IN ('confirmada','completada') THEN b.net_amount ELSE 0 END),0) AS net
		FROM bookings b JOIN courts c ON c.id = b.court_id AND c.deleted_at IS NULL` + baseWhere + `
		GROUP BY c.id, c.name ORDER BY revenue DESC`
	if err := db.Raw(courtSQL, baseArgs...).Scan(&out.CourtStats).Error; err != nil {
		return nil, fmt.Errorf("métricas por cancha: %w", err)
	}
	if len(out.CourtStats) > 0 {
		most := out.CourtStats[0]
		out.MostBooked = &most
	}

	peakSQL := `SELECT b.start_time AS hour, COUNT(*) AS count
		FROM bookings b JOIN courts c ON c.id = b.court_id AND c.deleted_at IS NULL` +
		baseWhere + ` AND b.status IN ('confirmada','completada')
		GROUP BY b.start_time ORDER BY count DESC, b.start_time ASC LIMIT 6`
	if err := db.Raw(peakSQL, baseArgs...).Scan(&out.PeakHours).Error; err != nil {
		return nil, fmt.Errorf("horas pico: %w", err)
	}

	ratingSQL := `SELECT COALESCE(AVG(r.rating),0) FROM reviews r
		JOIN courts c ON c.id = r.court_id AND c.deleted_at IS NULL
		WHERE r.hidden = 0`
	ratingArgs := []any{}
	if ownerID != nil {
		ratingSQL += " AND c.owner_id = ?"
		ratingArgs = append(ratingArgs, *ownerID)
	}
	if err := db.Raw(ratingSQL, ratingArgs...).Scan(&out.AvgRating).Error; err != nil {
		return nil, fmt.Errorf("calificación promedio: %w", err)
	}

	courtCount := db.Model(&models.Court{})
	if ownerID != nil {
		courtCount = courtCount.Where("owner_id = ?", *ownerID)
	}
	if err := courtCount.Count(&out.TotalCourts).Error; err != nil {
		return nil, fmt.Errorf("total canchas: %w", err)
	}

	nowDate := dateOnly(now)
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := nowDate.AddDate(0, 0, -(weekday - 1))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	countFor := func(cond string, arg any) int64 {
		sql := `SELECT COUNT(*) FROM bookings b JOIN courts c ON c.id = b.court_id AND c.deleted_at IS NULL
			WHERE b.date = ?`
		args := []any{arg}
		if ownerID != nil {
			sql += " AND c.owner_id = ?"
			args = append(args, *ownerID)
		}
		var n int64
		_ = db.Raw(sql, args...).Scan(&n).Error
		return n
	}
	out.TodayBookings = countFor("=", nowDate)
	out.WeekBookings = countRange(db, ownerID, weekStart, nowDate)
	out.MonthBookings = countRange(db, ownerID, monthStart, nowDate)

	return out, nil
}

func countRange(db *gorm.DB, ownerID *uint, from, to time.Time) int64 {
	sql := `SELECT COUNT(*) FROM bookings b JOIN courts c ON c.id = b.court_id AND c.deleted_at IS NULL
		WHERE b.date >= ? AND b.date <= ?`
	args := []any{dateOnly(from), dateOnly(to)}
	if ownerID != nil {
		sql += " AND c.owner_id = ?"
		args = append(args, *ownerID)
	}
	var n int64
	_ = db.Raw(sql, args...).Scan(&n).Error
	return n
}
