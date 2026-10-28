package repositories

import (
	"context"
	"time"

	"sportss/internal/models"
)

// UserRepository define la persistencia de usuarios.
type UserRepository interface {
	Create(ctx context.Context, u *models.User) error
	Update(ctx context.Context, u *models.User) error
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	List(ctx context.Context, search string, offset, limit int) ([]models.User, error)
	Count(ctx context.Context, search string) (int64, error)
	SetActive(ctx context.Context, id uint, active bool) error
	SetPassword(ctx context.Context, id uint, hash string) error
	CountAll(ctx context.Context) (int64, error)
}

// CourtSearchParams agrupa los filtros del buscador y del panel.
type CourtSearchParams struct {
	Query      string
	Sport      string
	City       string
	Department string
	MinPrice   int64
	MaxPrice   int64
	Date       *time.Time
	Hour       string
	Covered    *bool
	Sort       string
	Page       int
	Limit      int
}

// CourtRepository define la persistencia de canchas.
type CourtRepository interface {
	Create(ctx context.Context, c *models.Court) error
	Update(ctx context.Context, c *models.Court) error
	FindByID(ctx context.Context, id uint) (*models.Court, error)
	Delete(ctx context.Context, id uint) error
	ListByOwner(ctx context.Context, ownerID uint) ([]models.Court, error)
	Search(ctx context.Context, p CourtSearchParams) ([]models.Court, int64, error)
	Featured(ctx context.Context, limit int) ([]models.Court, error)
	UpdateRating(ctx context.Context, courtID uint) error
	SetStatus(ctx context.Context, id uint, status string) error
	ListAll(ctx context.Context, search, status string, offset, limit int) ([]models.Court, int64, error)
	CountAll(ctx context.Context) (int64, error)
	CountByOwner(ctx context.Context, ownerID uint) (int64, error)
	AddPhoto(ctx context.Context, photo *models.CourtPhoto) error
	DeletePhoto(ctx context.Context, courtID, photoID uint) (string, error)
	CountPhotos(ctx context.Context, courtID uint) (int64, error)
}

// BookingRepository define la persistencia de reservas.
type BookingRepository interface {
	WithTx(ctx context.Context, fn func(BookingRepository) error) error
	Create(ctx context.Context, b *models.Booking) error
	Update(ctx context.Context, id uint, fields map[string]any) error
	FindByID(ctx context.Context, id uint) (*models.Booking, error)
	FindOverlapping(ctx context.Context, courtID uint, date time.Time, start, end string) (bool, error)
	ActiveForDate(ctx context.Context, courtID uint, date time.Time) ([]models.Booking, error)
	ListByUser(ctx context.Context, userID uint, status string, offset, limit int) ([]models.Booking, int64, error)
	ListByOwner(ctx context.Context, ownerID uint, status, date string, offset, limit int) ([]models.Booking, int64, error)
	ListAll(ctx context.Context, status, date string, offset, limit int) ([]models.Booking, int64, error)
	ExpirePending(ctx context.Context, cutoff time.Time) ([]models.Booking, error)
	ConfirmedStartingBetween(ctx context.Context, from, to time.Time) ([]models.Booking, error)
	MarkReminderSent(ctx context.Context, id uint) error
	AutoCompletePast(ctx context.Context, now time.Time) ([]models.Booking, error)
}

// PaymentRepository define la persistencia de pagos.
type PaymentRepository interface {
	Create(ctx context.Context, p *models.Payment) error
	Update(ctx context.Context, p *models.Payment) error
	FindByBooking(ctx context.Context, bookingID uint) (*models.Payment, error)
	FindByExternalID(ctx context.Context, externalID string) (*models.Payment, error)
	ListByUser(ctx context.Context, userID uint, limit int) ([]models.Payment, error)
}

// InvoiceRepository define la persistencia de facturas.
type InvoiceRepository interface {
	Create(ctx context.Context, i *models.Invoice) error
	Update(ctx context.Context, i *models.Invoice) error
	FindByID(ctx context.Context, id uint) (*models.Invoice, error)
	FindByBooking(ctx context.Context, bookingID uint) (*models.Invoice, error)
	ListByUser(ctx context.Context, userID uint, limit int) ([]models.Invoice, error)
}

// ReviewRepository define la persistencia de reseñas.
type ReviewRepository interface {
	Create(ctx context.Context, r *models.Review) error
	Update(ctx context.Context, r *models.Review) error
	FindByID(ctx context.Context, id uint) (*models.Review, error)
	FindByBooking(ctx context.Context, bookingID uint) (*models.Review, error)
	ListByCourt(ctx context.Context, courtID uint, includeHidden bool, limit int) ([]models.Review, error)
	ListByOwner(ctx context.Context, ownerID uint, offset, limit int) ([]models.Review, int64, error)
	ListAll(ctx context.Context, search string, offset, limit int) ([]models.Review, int64, error)
	AverageByCourt(ctx context.Context, courtID uint) (float64, int, error)
	CountByOwner(ctx context.Context, ownerID uint) (int64, error)
}

// NotificationRepository define la persistencia de notificaciones.
type NotificationRepository interface {
	Create(ctx context.Context, n *models.Notification) error
	ListByUser(ctx context.Context, userID uint, limit int) ([]models.Notification, error)
	CountUnread(ctx context.Context, userID uint) (int64, error)
	MarkRead(ctx context.Context, id, userID uint) error
	MarkAllRead(ctx context.Context, userID uint) error
}

// RevenueSummary resume ingresos de un periodo.
type RevenueSummary struct {
	Total      int64
	Commission int64
	Net        int64
	Bookings   int64
	Confirmed  int64
	Cancelled  int64
}

// DailyPoint es un punto de la serie de tiempo para gráficas.
type DailyPoint struct {
	Date    string
	Label   string
	Count   int64
	Hours   int64
	Revenue int64
}

// CourtStat agrega métricas por cancha.
type CourtStat struct {
	CourtID    uint
	Name       string
	Bookings   int64
	Hours      int64
	Revenue    int64
	Commission int64
	Net        int64
}

// PeakHour muestra las horas con más reservas.
type PeakHour struct {
	Hour  string
	Count int64
}

// AnalyticsSummary contiene todos los datos del dashboard.
type AnalyticsSummary struct {
	Revenue       RevenueSummary
	Daily         []DailyPoint
	CourtStats    []CourtStat
	PeakHours     []PeakHour
	MostBooked    *CourtStat
	AvgRating     float64
	TotalCourts   int64
	TodayBookings int64
	WeekBookings  int64
	MonthBookings int64
	OccupancyRate float64
}

// AnalyticsRepository provee consultas agregadas para el panel.
type AnalyticsRepository interface {
	Summary(ctx context.Context, ownerID *uint, from, to, now time.Time) (*AnalyticsSummary, error)
}
