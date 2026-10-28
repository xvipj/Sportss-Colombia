package database

import (
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"sportss/internal/models"
)

// Seed crea datos demostrativos una sola vez.
func Seed(db *gorm.DB, loc *time.Location) error {
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		slog.Info("seed omitido: la base de datos ya contiene usuarios")
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		hash := func(p string) string {
			b, _ := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
			return string(b)
		}

		admin := models.User{
			Name: "Super Admin", Email: "admin@sportss.co", PasswordHash: hash("Admin123!"),
			Role: models.RoleAdmin, AccountType: models.AccountBusiness, BusinessName: "Sportss Colombia",
			Phone: "300 000 0001", Active: true, EmailNotify: true,
		}
		owner := models.User{
			Name: "Carlos Mendoza", Email: "dueno@sportss.co", PasswordHash: hash("Dueno123!"),
			Role: models.RoleBusiness, AccountType: models.AccountBusiness, BusinessName: "Complejo Deportivo El Gol",
			NIT: "900.123.456-7", Phone: "310 555 0101", Active: true, EmailNotify: true,
		}
		owner2 := models.User{
			Name: "Laura Ríos", Email: "duena2@sportss.co", PasswordHash: hash("Dueno123!"),
			Role: models.RoleBusiness, AccountType: models.AccountBusiness, BusinessName: "Arena Pacífico",
			NIT: "901.987.654-3", Phone: "315 555 0202", Active: true, EmailNotify: true,
		}
		player := models.User{
			Name: "Andrés Torres", Email: "jugador@sportss.co", PasswordHash: hash("Jugador123!"),
			Role: models.RolePlayer, AccountType: models.AccountPerson, Phone: "320 555 0202",
			Active: true, EmailNotify: true,
		}
		for _, u := range []*models.User{&admin, &owner, &owner2, &player} {
			if err := tx.Create(u).Error; err != nil {
				return err
			}
		}

		courts := []models.Court{
			{
				OwnerID: owner.ID, Name: "El Campín Fútbol 5", Sport: "Fútbol 5", Surface: "Césped sintético",
				Description: "Cancha techada de última generación, ideal para partidos después del trabajo. Incluye balón y petos.",
				Length:      40, Width: 20, Covered: true, Floodlights: true, Parking: true, Bathrooms: true,
				Lockers: true, Cafeteria: true, PricePerHour: 120000, OpenTime: "07:00", CloseTime: "23:00",
				AvailableDays: "0,1,2,3,4,5,6", Department: "Cundinamarca", City: "Bogotá",
				Neighborhood: "Salitre", Address: "Cra. 68 # 40-12", Latitude: 4.6584, Longitude: -74.1048,
				Status: models.CourtActive, RatingAvg: 5, RatingCount: 1,
				Photos: []models.CourtPhoto{{Path: "/static/img/court-futbol.svg", SortOrder: 0}},
			},
			{
				OwnerID: owner.ID, Name: "La Bombonera 7", Sport: "Fútbol 7", Surface: "Césped natural",
				Description: "Nuestro orgullo: grama natural certificada, iluminación LED y zona de hidratación.",
				Length:      60, Width: 40, Covered: false, Floodlights: true, Parking: true, Bathrooms: true,
				Lockers: true, Cafeteria: true, PricePerHour: 180000, OpenTime: "06:00", CloseTime: "22:00",
				AvailableDays: "1,2,3,4,5,6,0", Department: "Antioquia", City: "Medellín",
				Neighborhood: "Laureles", Address: "Calle 33 # 74-20", Latitude: 6.2442, Longitude: -75.5812,
				Status: models.CourtActive, RatingAvg: 4.8, RatingCount: 6,
				Photos: []models.CourtPhoto{{Path: "/static/img/court-futbol7.svg", SortOrder: 0}},
			},
			{
				OwnerID: owner.ID, Name: "Arena Norte Basketball", Sport: "Baloncesto", Surface: "Cemento",
				Description: "Cancha cubierta con tableros de vidrio y aros profesionales. Perfecta para torneos 3x3.",
				Length:      28, Width: 15, Covered: true, Floodlights: true, Parking: true, Bathrooms: true,
				Lockers: false, Cafeteria: true, PricePerHour: 95000, OpenTime: "08:00", CloseTime: "22:00",
				AvailableDays: "1,2,3,4,5,6", Department: "Cundinamarca", City: "Bogotá",
				Neighborhood: "Usaquén", Address: "Calle 140 # 15-30", Latitude: 4.7109, Longitude: -74.0310,
				Status: models.CourtActive, RatingAvg: 4.5, RatingCount: 1,
				Photos: []models.CourtPhoto{{Path: "/static/img/court-basket.svg", SortOrder: 0}},
			},
			{
				OwnerID: owner2.ID, Name: "Club Saque Tenis", Sport: "Tenis", Surface: "Arcilla",
				Description: "Dos canchas de polvo de ladrillo en el sur de Cali. Profesor disponible con reserva.",
				Length:      23.77, Width: 10.97, Covered: false, Floodlights: true, Parking: true, Bathrooms: true,
				Lockers: true, Cafeteria: false, PricePerHour: 80000, OpenTime: "06:00", CloseTime: "20:00",
				AvailableDays: "1,2,3,4,5,6,0", Department: "Valle del Cauca", City: "Cali",
				Neighborhood: "Ciudad Jardín", Address: "Calle 9 # 50-45", Latitude: 3.4516, Longitude: -76.5320,
				Status: models.CourtActive, RatingAvg: 4.2, RatingCount: 4,
				Photos: []models.CourtPhoto{{Path: "/static/img/court-tennis.svg", SortOrder: 0}},
			},
			{
				OwnerID: owner2.ID, Name: "Pádel Vista Hermosa", Sport: "Pádel", Surface: "Césped sintético",
				Description: "Pista panorámica con paredes de cristal. Alquiler de palas y bolas disponible.",
				Length:      20, Width: 10, Covered: true, Floodlights: true, Parking: false, Bathrooms: true,
				Lockers: true, Cafeteria: true, PricePerHour: 110000, OpenTime: "07:00", CloseTime: "23:00",
				AvailableDays: "0,1,2,3,4,5,6", Department: "Antioquia", City: "Medellín",
				Neighborhood: "El Poblado", Address: "Cra. 43A # 10-50", Latitude: 6.2090, Longitude: -75.5700,
				Status: models.CourtActive, RatingAvg: 4.9, RatingCount: 4,
				Photos: []models.CourtPhoto{{Path: "/static/img/court-padel.svg", SortOrder: 0}},
			},
			{
				OwnerID: owner.ID, Name: "Voleibol Playa Caribe", Sport: "Voleibol", Surface: "Arena",
				Description: "Arena fina importada, red oficial y ambiente caribeño. Ideal para grupos y cumpleaños.",
				Length:      18, Width: 9, Covered: false, Floodlights: true, Parking: true, Bathrooms: true,
				Lockers: false, Cafeteria: true, PricePerHour: 70000, OpenTime: "08:00", CloseTime: "21:00",
				AvailableDays: "0,1,2,3,4,5,6", Department: "Bolívar", City: "Cartagena",
				Neighborhood: "Bocagrande", Address: "Cra. 1 # 4-20", Latitude: 10.3910, Longitude: -75.5550,
				Status: models.CourtActive, RatingAvg: 4.7, RatingCount: 4,
				Photos: []models.CourtPhoto{{Path: "/static/img/court-voley.svg", SortOrder: 0}},
			},
		}
		for i := range courts {
			if err := tx.Create(&courts[i]).Error; err != nil {
				return err
			}
		}

		now := time.Now().In(loc)
		day := func(offset int) time.Time {
			d := now.AddDate(0, 0, offset)
			// Las fechas de reserva se guardan como fecha calendario en UTC
			// para que la comparación y el índice único sean consistentes.
			return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
		}
		booking := func(court models.Court, offset int, start, end string, status string) models.Booking {
			total := court.PricePerHour
			commission := total * 10 / 100
			return models.Booking{
				CourtID: court.ID, UserID: player.ID, Date: day(offset), StartTime: start, EndTime: end,
				Hours: 1, TotalAmount: total, CommissionAmount: commission, NetAmount: total - commission,
				Status: status,
			}
		}

		completed1 := booking(courts[0], -12, "18:00", "19:00", models.BookingCompleted)
		completed1.CompletedAt = ptr(now.AddDate(0, 0, -12))
		completed2 := booking(courts[2], -8, "16:00", "17:00", models.BookingCompleted)
		completed2.CompletedAt = ptr(now.AddDate(0, 0, -8))
		completed3 := booking(courts[4], -5, "20:00", "21:00", models.BookingCompleted)
		completed3.CompletedAt = ptr(now.AddDate(0, 0, -5))
		confirmed1 := booking(courts[1], 2, "19:00", "21:00", models.BookingConfirmed)
		confirmed1.Hours = 2
		confirmed1.TotalAmount = courts[1].PricePerHour * 2
		confirmed1.CommissionAmount = confirmed1.TotalAmount / 10
		confirmed1.NetAmount = confirmed1.TotalAmount - confirmed1.CommissionAmount
		confirmed2 := booking(courts[4], 4, "18:00", "19:00", models.BookingConfirmed)
		pending := booking(courts[3], 3, "09:00", "10:00", models.BookingPending)

		for _, b := range []*models.Booking{&completed1, &completed2, &completed3, &confirmed1, &confirmed2, &pending} {
			if err := tx.Create(b).Error; err != nil {
				return err
			}
		}

		reviews := []models.Review{
			{BookingID: completed1.ID, CourtID: courts[0].ID, UserID: player.ID, Rating: 5, Comment: "Excelente cancha, muy bien iluminada y el césped impecable. Volveremos."},
			{BookingID: completed2.ID, CourtID: courts[2].ID, UserID: player.ID, Rating: 5, Comment: "Los tableros y el piso están en perfecto estado. Buena atención."},
			{BookingID: completed3.ID, CourtID: courts[4].ID, UserID: player.ID, Rating: 4, Comment: "Muy buena pista de pádel. Solo faltó un poco más de parqueadero."},
		}
		for i := range reviews {
			if err := tx.Create(&reviews[i]).Error; err != nil {
				return err
			}
		}

		notifications := []models.Notification{
			{UserID: player.ID, Type: models.NotifyPaymentOK, Title: "Pago confirmado", Message: "Tu reserva en La Bombonera 7 quedó confirmada.", Link: "/mis-reservas", Read: false},
			{UserID: owner.ID, Type: models.NotifyNewBooking, Title: "Nueva reserva recibida", Message: "Andrés Torres reservó La Bombonera 7.", Link: "/panel/reservas", Read: false},
			{UserID: player.ID, Type: models.NotifyReminder, Title: "Recordatorio de reserva", Message: "Tu partido en El Campín Fútbol 5 es pronto.", Link: "/mis-reservas", Read: true},
		}
		for i := range notifications {
			if err := tx.Create(&notifications[i]).Error; err != nil {
				return err
			}
		}

		slog.Info("seed creado", "usuarios", 4, "canchas", len(courts), "reservas", 6)
		return nil
	})
}

func ptr[T any](v T) *T { return &v }
