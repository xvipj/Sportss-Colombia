package services

import (
	"context"
	"time"

	"sportss/internal/config"
	"sportss/internal/repositories"
)

type AnalyticsService struct {
	cfg    *config.Config
	repo   repositories.AnalyticsRepository
	courts repositories.CourtRepository
}

func NewAnalyticsService(cfg *config.Config, repo repositories.AnalyticsRepository, courts repositories.CourtRepository) *AnalyticsService {
	return &AnalyticsService{cfg: cfg, repo: repo, courts: courts}
}

func (s *AnalyticsService) Summary(ctx context.Context, ownerID *uint, from, to time.Time) (*repositories.AnalyticsSummary, error) {
	now := time.Now().In(s.cfg.Location)
	summary, err := s.repo.Summary(ctx, ownerID, from, to, now)
	if err != nil {
		return nil, Internal("No fue posible calcular las analíticas", err)
	}

	// Tasa de ocupación: horas reservadas / horas de atención disponibles.
	var courtList []modelsCourtSchedule
	if ownerID != nil {
		courts, err := s.courts.ListByOwner(ctx, *ownerID)
		if err == nil {
			for _, c := range courts {
				courtList = append(courtList, modelsCourtSchedule{available: c.AvailableDays, open: c.OpenTime, close: c.CloseTime})
			}
		}
	} else {
		courts, total, err := s.courts.ListAll(ctx, "", "", 0, 500)
		if err == nil && total > 0 {
			for _, c := range courts {
				courtList = append(courtList, modelsCourtSchedule{available: c.AvailableDays, open: c.OpenTime, close: c.CloseTime})
			}
		}
	}
	var capacityHours float64
	for d := dateOnlyUTC(from); !d.After(dateOnlyUTC(to)); d = d.AddDate(0, 0, 1) {
		for _, c := range courtList {
			if dayAvailable(c.available, int(d.Weekday())) {
				start, err1 := time.Parse("15:04", c.open)
				end, err2 := time.Parse("15:04", c.close)
				if err1 == nil && err2 == nil && end.After(start) {
					capacityHours += end.Sub(start).Hours()
				}
			}
		}
	}
	var bookedHours float64
	for _, cs := range summary.CourtStats {
		bookedHours += float64(cs.Hours)
	}
	if capacityHours > 0 {
		summary.OccupancyRate = bookedHours / capacityHours * 100
		if summary.OccupancyRate > 100 {
			summary.OccupancyRate = 100
		}
	}
	return summary, nil
}

type modelsCourtSchedule struct {
	available string
	open      string
	close     string
}
