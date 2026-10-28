package models

import (
	"time"

	"gorm.io/gorm"
)

// Roles de plataforma.
const (
	RolePlayer   = "player"
	RoleBusiness = "business"
	RoleAdmin    = "admin"
)

// Tipos de cuenta visibles al registrarse.
const (
	AccountPerson   = "persona"
	AccountBusiness = "empresa"
)

// Estados de una reserva.
const (
	BookingPending   = "pendiente_pago"
	BookingConfirmed = "confirmada"
	BookingCancelled = "cancelada"
	BookingExpired   = "expirada"
	BookingCompleted = "completada"
)

// Estados de una cancha.
const (
	CourtActive = "activa"
	CourtPaused = "pausada"
)

// Estados de pago.
const (
	PaymentPending  = "pendiente"
	PaymentApproved = "aprobado"
	PaymentFailed   = "fallido"
	PaymentRefunded = "reembolsado"
)

// User representa a un jugador, una cuenta empresa o un super administrador.
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:120;not null" json:"name"`
	Email        string         `gorm:"size:180;uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	Role         string         `gorm:"size:20;not null;index" json:"role"`
	AccountType  string         `gorm:"size:20;not null" json:"account_type"`
	BusinessName string         `gorm:"size:160" json:"business_name,omitempty"`
	NIT          string         `gorm:"size:40" json:"nit,omitempty"`
	Phone        string         `gorm:"size:40" json:"phone,omitempty"`
	AvatarPath   string         `gorm:"size:255" json:"avatar_path,omitempty"`
	EmailNotify  bool           `gorm:"default:true" json:"email_notify"`
	Active       bool           `gorm:"default:true;index" json:"active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (u User) IsBusiness() bool { return u.Role == RoleBusiness || u.Role == RoleAdmin }
func (u User) IsAdmin() bool    { return u.Role == RoleAdmin }

// Court es una cancha publicada por una cuenta empresa.
type Court struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	OwnerID       uint           `gorm:"not null;index" json:"owner_id"`
	Owner         User           `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Name          string         `gorm:"size:160;not null" json:"name"`
	Description   string         `gorm:"type:text" json:"description"`
	Sport         string         `gorm:"size:60;not null;index" json:"sport"`
	Surface       string         `gorm:"size:80;not null" json:"surface"`
	Length        float64        `json:"length"`
	Width         float64        `json:"width"`
	Covered       bool           `gorm:"index" json:"covered"`
	Floodlights   bool           `json:"floodlights"`
	Parking       bool           `json:"parking"`
	Bathrooms     bool           `json:"bathrooms"`
	Lockers       bool           `json:"lockers"`
	Cafeteria     bool           `json:"cafeteria"`
	PricePerHour  int64          `gorm:"not null;index" json:"price_per_hour"`
	OpenTime      string         `gorm:"size:5;not null" json:"open_time"`
	CloseTime     string         `gorm:"size:5;not null" json:"close_time"`
	AvailableDays string         `gorm:"size:40;not null" json:"available_days"`
	Department    string         `gorm:"size:80;index" json:"department"`
	City          string         `gorm:"size:100;index" json:"city"`
	Neighborhood  string         `gorm:"size:120;index" json:"neighborhood"`
	Address       string         `gorm:"size:220" json:"address"`
	Latitude      float64        `json:"latitude"`
	Longitude     float64        `json:"longitude"`
	Status        string         `gorm:"size:20;not null;index" json:"status"`
	RatingAvg     float64        `gorm:"default:0" json:"rating_avg"`
	RatingCount   int            `gorm:"default:0" json:"rating_count"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	Photos        []CourtPhoto   `gorm:"foreignKey:CourtID" json:"photos,omitempty"`
}

// CourtPhoto guarda la ruta pública de una foto de cancha.
type CourtPhoto struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CourtID   uint      `gorm:"not null;index" json:"court_id"`
	Path      string    `gorm:"size:255;not null" json:"path"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// Booking es una reserva de una o varias horas consecutivas.
type Booking struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	CourtID          uint       `gorm:"not null;index" json:"court_id"`
	Court            Court      `gorm:"foreignKey:CourtID" json:"court,omitempty"`
	UserID           uint       `gorm:"not null;index" json:"user_id"`
	User             User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Date             time.Time  `gorm:"not null;index" json:"date"`
	StartTime        string     `gorm:"size:5;not null" json:"start_time"`
	EndTime          string     `gorm:"size:5;not null" json:"end_time"`
	Hours            int        `gorm:"not null" json:"hours"`
	TotalAmount      int64      `gorm:"not null" json:"total_amount"`
	CommissionAmount int64      `gorm:"not null" json:"commission_amount"`
	NetAmount        int64      `gorm:"not null" json:"net_amount"`
	Status           string     `gorm:"size:24;not null;index" json:"status"`
	CancelledAt      *time.Time `json:"cancelled_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	ReminderSent     bool       `gorm:"default:false" json:"reminder_sent"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Payment registra la transacción asociada a una reserva.
type Payment struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	BookingID    uint      `gorm:"uniqueIndex;not null" json:"booking_id"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`
	Provider     string    `gorm:"size:30;not null" json:"provider"`
	ExternalID   string    `gorm:"size:120;index" json:"external_id"`
	PreferenceID string    `gorm:"size:180;index" json:"preference_id"`
	CheckoutURL  string    `gorm:"size:500" json:"checkout_url,omitempty"`
	Status       string    `gorm:"size:30;not null;index" json:"status"`
	Amount       int64     `gorm:"not null" json:"amount"`
	Currency     string    `gorm:"size:10;not null" json:"currency"`
	Method       string    `gorm:"size:60" json:"method"`
	RawPayload   string    `gorm:"type:text" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Invoice es la factura PDF generada al confirmar el pago.
type Invoice struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	BookingID        uint      `gorm:"uniqueIndex;not null" json:"booking_id"`
	UserID           uint      `gorm:"not null;index" json:"user_id"`
	CourtID          uint      `gorm:"not null;index" json:"court_id"`
	Number           string    `gorm:"size:40;uniqueIndex;not null" json:"number"`
	Path             string    `gorm:"size:255;not null" json:"path"`
	TotalAmount      int64     `gorm:"not null" json:"total_amount"`
	CommissionAmount int64     `gorm:"not null" json:"commission_amount"`
	NetAmount        int64     `gorm:"not null" json:"net_amount"`
	IssuedAt         time.Time `json:"issued_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// Review es una reseña verificada de una reserva completada.
type Review struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	BookingID     uint      `gorm:"uniqueIndex;not null" json:"booking_id"`
	CourtID       uint      `gorm:"not null;index" json:"court_id"`
	UserID        uint      `gorm:"not null;index" json:"user_id"`
	User          User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Court         Court     `gorm:"foreignKey:CourtID" json:"court,omitempty"`
	Rating        int       `gorm:"not null" json:"rating"`
	Comment       string    `gorm:"type:text" json:"comment"`
	OwnerResponse string    `gorm:"type:text" json:"owner_response"`
	Hidden        bool      `gorm:"default:false;index" json:"hidden"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Notification se entrega por SSE y se conserva en la base de datos.
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Type      string    `gorm:"size:50;not null" json:"type"`
	Title     string    `gorm:"size:160;not null" json:"title"`
	Message   string    `gorm:"size:500;not null" json:"message"`
	Link      string    `gorm:"size:255" json:"link"`
	Read      bool      `gorm:"default:false;index" json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// Tipos de notificación.
const (
	NotifyBookingCreated = "reserva_creada"
	NotifyPaymentOK      = "pago_confirmado"
	NotifyReminder       = "recordatorio"
	NotifyCancelled      = "reserva_cancelada"
	NotifyNewReview      = "nueva_resena"
	NotifyNewBooking     = "nueva_reserva"
	NotifyCompleted      = "reserva_completada"
)
