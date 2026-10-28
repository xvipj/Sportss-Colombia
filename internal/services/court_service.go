package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"sportss/internal/cache"
	"sportss/internal/config"
	"sportss/internal/models"
	"sportss/internal/repositories"
)

var Sports = []string{
	"Fútbol 5", "Fútbol 7", "Fútbol 11", "Fútbol sala", "Baloncesto", "Voleibol",
	"Tenis", "Pádel", "Microfútbol", "Béisbol/Softbol", "Otro",
}

// SurfacesBySport alimenta el selector dinámico del formulario.
var SurfacesBySport = map[string][]string{
	"Fútbol 5":        {"Césped sintético", "Césped natural", "Cemento", "Parquet"},
	"Fútbol 7":        {"Césped sintético", "Césped natural", "Cemento"},
	"Fútbol 11":       {"Césped natural", "Césped sintético"},
	"Fútbol sala":     {"Parquet", "Cemento", "Césped sintético"},
	"Baloncesto":      {"Cemento", "Parquet", "Caucho"},
	"Voleibol":        {"Arena", "Parquet", "Cemento"},
	"Tenis":           {"Arcilla", "Cemento", "Césped sintético"},
	"Pádel":           {"Césped sintético", "Cemento"},
	"Microfútbol":     {"Parquet", "Cemento", "Césped sintético"},
	"Béisbol/Softbol": {"Arcilla", "Césped natural", "Arena"},
	"Otro":            {"Césped sintético", "Cemento", "Parquet", "Arcilla", "Arena", "Otro"},
}

type CourtService struct {
	cfg      *config.Config
	courts   repositories.CourtRepository
	bookings repositories.BookingRepository
	cache    cache.Store
}

type CourtInput struct {
	Name          string
	Description   string
	Sport         string
	Surface       string
	Length        float64
	Width         float64
	Covered       bool
	Floodlights   bool
	Parking       bool
	Bathrooms     bool
	Lockers       bool
	Cafeteria     bool
	PricePerHour  int64
	OpenTime      string
	CloseTime     string
	AvailableDays []string
	Department    string
	City          string
	Neighborhood  string
	Address       string
	Latitude      float64
	Longitude     float64
	Status        string
}

type AvailabilitySlot struct {
	Hour      string `json:"hour"`
	End       string `json:"end"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type SearchResult struct {
	Courts []models.Court `json:"courts"`
	Total  int64          `json:"total"`
	Page   int            `json:"page"`
	Limit  int            `json:"limit"`
}

func NewCourtService(cfg *config.Config, courts repositories.CourtRepository, bookings repositories.BookingRepository, cacheStore cache.Store) *CourtService {
	return &CourtService{cfg: cfg, courts: courts, bookings: bookings, cache: cacheStore}
}

func (s *CourtService) Create(ctx context.Context, ownerID uint, in CourtInput) (*models.Court, error) {
	days, err := normalizeDays(in.AvailableDays)
	if err != nil {
		return nil, err
	}
	if err := s.validate(in, days); err != nil {
		return nil, err
	}
	status := in.Status
	if status != models.CourtPaused {
		status = models.CourtActive
	}
	court := &models.Court{
		OwnerID: ownerID, Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description),
		Sport: in.Sport, Surface: in.Surface, Length: in.Length, Width: in.Width,
		Covered: in.Covered, Floodlights: in.Floodlights, Parking: in.Parking, Bathrooms: in.Bathrooms,
		Lockers: in.Lockers, Cafeteria: in.Cafeteria, PricePerHour: in.PricePerHour,
		OpenTime: in.OpenTime, CloseTime: in.CloseTime, AvailableDays: days,
		Department: strings.TrimSpace(in.Department), City: strings.TrimSpace(in.City),
		Neighborhood: strings.TrimSpace(in.Neighborhood), Address: strings.TrimSpace(in.Address),
		Latitude: in.Latitude, Longitude: in.Longitude, Status: status,
	}
	if err := s.courts.Create(ctx, court); err != nil {
		return nil, Internal("No fue posible publicar la cancha", err)
	}
	s.invalidate(ctx)
	return court, nil
}

func (s *CourtService) Update(ctx context.Context, courtID, ownerID uint, isAdmin bool, in CourtInput) (*models.Court, error) {
	court, err := s.courts.FindByID(ctx, courtID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, NotFound("Cancha no encontrada.")
		}
		return nil, Internal("No fue posible cargar la cancha", err)
	}
	if !isAdmin && court.OwnerID != ownerID {
		return nil, Forbidden("No tienes permiso para editar esta cancha.")
	}
	days, err := normalizeDays(in.AvailableDays)
	if err != nil {
		return nil, err
	}
	if err := s.validate(in, days); err != nil {
		return nil, err
	}
	status := in.Status
	if status != models.CourtActive && status != models.CourtPaused {
		status = court.Status
	}
	court.Name = strings.TrimSpace(in.Name)
	court.Description = strings.TrimSpace(in.Description)
	court.Sport = in.Sport
	court.Surface = in.Surface
	court.Length = in.Length
	court.Width = in.Width
	court.Covered = in.Covered
	court.Floodlights = in.Floodlights
	court.Parking = in.Parking
	court.Bathrooms = in.Bathrooms
	court.Lockers = in.Lockers
	court.Cafeteria = in.Cafeteria
	court.PricePerHour = in.PricePerHour
	court.OpenTime = in.OpenTime
	court.CloseTime = in.CloseTime
	court.AvailableDays = days
	court.Department = strings.TrimSpace(in.Department)
	court.City = strings.TrimSpace(in.City)
	court.Neighborhood = strings.TrimSpace(in.Neighborhood)
	court.Address = strings.TrimSpace(in.Address)
	court.Latitude = in.Latitude
	court.Longitude = in.Longitude
	court.Status = status
	if err := s.courts.Update(ctx, court); err != nil {
		return nil, Internal("No fue posible actualizar la cancha", err)
	}
	s.invalidate(ctx)
	return court, nil
}

func (s *CourtService) Get(ctx context.Context, id uint) (*models.Court, error) {
	court, err := s.courts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, NotFound("Cancha no encontrada.")
		}
		return nil, Internal("No fue posible cargar la cancha", err)
	}
	return court, nil
}

func (s *CourtService) Delete(ctx context.Context, courtID, ownerID uint, isAdmin bool) error {
	court, err := s.courts.FindByID(ctx, courtID)
	if err != nil {
		return NotFound("Cancha no encontrada.")
	}
	if !isAdmin && court.OwnerID != ownerID {
		return Forbidden("No tienes permiso para eliminar esta cancha.")
	}
	if err := s.courts.Delete(ctx, courtID); err != nil {
		return Internal("No fue posible eliminar la cancha", err)
	}
	s.invalidate(ctx)
	return nil
}

func (s *CourtService) SetStatus(ctx context.Context, courtID, ownerID uint, isAdmin bool, status string) error {
	if status != models.CourtActive && status != models.CourtPaused {
		return BadRequest("Estado de cancha inválido.")
	}
	court, err := s.courts.FindByID(ctx, courtID)
	if err != nil {
		return NotFound("Cancha no encontrada.")
	}
	if !isAdmin && court.OwnerID != ownerID {
		return Forbidden("No tienes permiso sobre esta cancha.")
	}
	if err := s.courts.SetStatus(ctx, courtID, status); err != nil {
		return Internal("No fue posible actualizar la cancha", err)
	}
	s.invalidate(ctx)
	return nil
}

func (s *CourtService) ListByOwner(ctx context.Context, ownerID uint) ([]models.Court, error) {
	courts, err := s.courts.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, Internal("No fue posible cargar tus canchas", err)
	}
	return courts, nil
}

func (s *CourtService) Search(ctx context.Context, p repositories.CourtSearchParams) (*SearchResult, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit <= 0 || p.Limit > 60 {
		p.Limit = 9
	}
	key := s.searchCacheKey(p)
	if raw, ok, _ := s.cache.Get(ctx, key); ok {
		var cached SearchResult
		if json.Unmarshal([]byte(raw), &cached) == nil {
			return &cached, nil
		}
	}
	courts, total, err := s.courts.Search(ctx, p)
	if err != nil {
		return nil, Internal("No fue posible buscar canchas", err)
	}
	result := &SearchResult{Courts: courts, Total: total, Page: p.Page, Limit: p.Limit}
	if b, err := json.Marshal(result); err == nil {
		_ = s.cache.Set(ctx, key, string(b), 2*time.Minute)
	}
	return result, nil
}

func (s *CourtService) Featured(ctx context.Context, limit int) ([]models.Court, error) {
	const key = "courts:featured"
	if raw, ok, _ := s.cache.Get(ctx, key); ok {
		var courts []models.Court
		if json.Unmarshal([]byte(raw), &courts) == nil {
			return courts, nil
		}
	}
	courts, err := s.courts.Featured(ctx, limit)
	if err != nil {
		return nil, Internal("No fue posible cargar las canchas destacadas", err)
	}
	if b, err := json.Marshal(courts); err == nil {
		_ = s.cache.Set(ctx, key, string(b), 3*time.Minute)
	}
	return courts, nil
}

func (s *CourtService) Availability(ctx context.Context, courtID uint, dateStr string) (*models.Court, time.Time, []AvailabilitySlot, error) {
	court, err := s.Get(ctx, courtID)
	if err != nil {
		return nil, time.Time{}, nil, err
	}
	if court.Status != models.CourtActive {
		return nil, time.Time{}, nil, BadRequest("Esta cancha está pausada temporalmente.")
	}
	date, err := s.ParseDate(dateStr)
	if err != nil {
		return nil, time.Time{}, nil, err
	}
	today := s.today()
	if date.Before(dateOnlyUTC(today)) {
		return nil, time.Time{}, nil, BadRequest("No puedes consultar fechas pasadas.")
	}
	if !dayAvailable(court.AvailableDays, int(date.Weekday())) {
		return court, date, []AvailabilitySlot{}, nil
	}
	bookings, err := s.activeBookingsFor(ctx, court.ID, date)
	if err != nil {
		return nil, time.Time{}, nil, err
	}
	start, err := time.Parse("15:04", court.OpenTime)
	if err != nil {
		return nil, time.Time{}, nil, Internal("Horario inválido en la cancha", err)
	}
	end, err := time.Parse("15:04", court.CloseTime)
	if err != nil {
		return nil, time.Time{}, nil, Internal("Horario inválido en la cancha", err)
	}
	now := time.Now().In(s.cfg.Location)
	slots := make([]AvailabilitySlot, 0)
	for t := start; t.Before(end); t = t.Add(time.Hour) {
		next := t.Add(time.Hour)
		slot := AvailabilitySlot{Hour: t.Format("15:04"), End: next.Format("15:04"), Available: true}
		slotStart := time.Date(date.Year(), date.Month(), date.Day(), t.Hour(), t.Minute(), 0, 0, s.cfg.Location)
		if date.Equal(dateOnlyUTC(today)) && !slotStart.After(now) {
			slot.Available = false
			slot.Reason = "Hora pasada"
		}
		for _, b := range bookings {
			if b.StartTime < slot.End && b.EndTime > slot.Hour {
				slot.Available = false
				slot.Reason = "Ocupada"
				break
			}
		}
		slots = append(slots, slot)
	}
	return court, date, slots, nil
}

func (s *CourtService) AddPhoto(ctx context.Context, courtID, ownerID uint, path string) error {
	court, err := s.courts.FindByID(ctx, courtID)
	if err != nil {
		return NotFound("Cancha no encontrada.")
	}
	if court.OwnerID != ownerID {
		return Forbidden("No tienes permiso sobre esta cancha.")
	}
	n, err := s.courts.CountPhotos(ctx, courtID)
	if err != nil {
		return Internal("No fue posible validar las fotos", err)
	}
	if n >= 5 {
		return BadRequest("Puedes subir máximo 5 fotos por cancha.")
	}
	if err := s.courts.AddPhoto(ctx, &models.CourtPhoto{CourtID: courtID, Path: path, SortOrder: int(n)}); err != nil {
		return Internal("No fue posible guardar la foto", err)
	}
	s.invalidate(ctx)
	return nil
}

func (s *CourtService) DeletePhoto(ctx context.Context, courtID, photoID, ownerID uint, isAdmin bool) error {
	court, err := s.courts.FindByID(ctx, courtID)
	if err != nil {
		return NotFound("Cancha no encontrada.")
	}
	if !isAdmin && court.OwnerID != ownerID {
		return Forbidden("No tienes permiso sobre esta cancha.")
	}
	path, err := s.courts.DeletePhoto(ctx, courtID, photoID)
	if err != nil {
		return Internal("No fue posible eliminar la foto", err)
	}
	name := filepath.Base(path)
	if strings.HasPrefix(path, "/static/uploads/") && name != "." && name != "" {
		_ = os.Remove(filepath.Join(s.cfg.UploadDir, name))
	}
	s.invalidate(ctx)
	return nil
}

func (s *CourtService) ListAll(ctx context.Context, search, status string, page, limit int) ([]models.Court, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	courts, total, err := s.courts.ListAll(ctx, search, status, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, Internal("No fue posible cargar las canchas", err)
	}
	return courts, total, nil
}

func (s *CourtService) CountByOwner(ctx context.Context, ownerID uint) (int64, error) {
	return s.courts.CountByOwner(ctx, ownerID)
}

func (s *CourtService) CountAll(ctx context.Context) (int64, error) { return s.courts.CountAll(ctx) }

func (s *CourtService) ParseDate(dateStr string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", dateStr, s.cfg.Location)
	if err != nil {
		return time.Time{}, BadRequest("Selecciona una fecha válida.")
	}
	return dateOnlyUTC(t), nil
}

func (s *CourtService) today() time.Time {
	now := time.Now().In(s.cfg.Location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.cfg.Location)
}

func (s *CourtService) validate(in CourtInput, days string) error {
	if len(strings.TrimSpace(in.Name)) < 3 {
		return BadRequest("El nombre de la cancha debe tener al menos 3 caracteres.")
	}
	validSport := false
	for _, sp := range Sports {
		if in.Sport == sp {
			validSport = true
			break
		}
	}
	if !validSport {
		return BadRequest("Selecciona un deporte válido.")
	}
	if strings.TrimSpace(in.Surface) == "" {
		return BadRequest("Selecciona el tipo de superficie.")
	}
	if in.Length <= 0 || in.Width <= 0 || in.Length > 200 || in.Width > 200 {
		return BadRequest("Las medidas deben ser valores positivos en metros.")
	}
	if in.PricePerHour < 1000 {
		return BadRequest("El precio por hora debe ser de al menos $1.000 COP.")
	}
	open, err := time.Parse("15:04", in.OpenTime)
	if err != nil {
		return BadRequest("La hora de apertura no es válida.")
	}
	close, err := time.Parse("15:04", in.CloseTime)
	if err != nil {
		return BadRequest("La hora de cierre no es válida.")
	}
	if !close.After(open) {
		return BadRequest("La hora de cierre debe ser posterior a la apertura.")
	}
	if days == "" {
		return BadRequest("Selecciona al menos un día disponible.")
	}
	if strings.TrimSpace(in.City) == "" || strings.TrimSpace(in.Department) == "" || strings.TrimSpace(in.Address) == "" {
		return BadRequest("Completa la ubicación de la cancha.")
	}
	if in.Latitude < -90 || in.Latitude > 90 || in.Longitude < -180 || in.Longitude > 180 {
		return BadRequest("Las coordenadas no son válidas.")
	}
	return nil
}

func (s *CourtService) searchCacheKey(p repositories.CourtSearchParams) string {
	data, _ := json.Marshal(p)
	sum := sha256.Sum256(data)
	return "courts:search:" + hex.EncodeToString(sum[:12])
}

func (s *CourtService) invalidate(ctx context.Context) {
	_ = s.cache.DeletePrefix(ctx, "courts:")
}

func (s *CourtService) activeBookingsFor(ctx context.Context, courtID uint, date time.Time) ([]models.Booking, error) {
	result, err := s.bookings.ActiveForDate(ctx, courtID, date)
	if err != nil {
		return nil, Internal("No fue posible consultar la disponibilidad", err)
	}
	return result, nil
}

func normalizeDays(days []string) (string, error) {
	seen := make(map[int]bool)
	for _, d := range days {
		n, err := strconv.Atoi(strings.TrimSpace(d))
		if err != nil || n < 0 || n > 6 {
			return "", BadRequest("Día disponible inválido.")
		}
		seen[n] = true
	}
	if len(seen) == 0 {
		return "", BadRequest("Selecciona al menos un día disponible.")
	}
	values := make([]int, 0, len(seen))
	for n := range seen {
		values = append(values, n)
	}
	sort.Ints(values)
	parts := make([]string, len(values))
	for i, n := range values {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ","), nil
}

func dayAvailable(days string, weekday int) bool {
	for _, part := range strings.Split(days, ",") {
		if strings.TrimSpace(part) == strconv.Itoa(weekday) {
			return true
		}
	}
	return false
}

func dateOnlyUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// FormatCOP devuelve un valor monetario legible en pesos colombianos.
func FormatCOP(v int64) string {
	s := strconv.FormatInt(v, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return "$" + s
}

// DayName traduce el día de la semana para las plantillas.
func DayName(day int) string {
	names := []string{"Domingo", "Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado"}
	if day < 0 || day > 6 {
		return ""
	}
	return names[day]
}

// DaysLabel convierte "1,2,3" en "Lunes, Martes, Miércoles".
func DaysLabel(days string) string {
	parts := strings.Split(days, ",")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil {
			names = append(names, DayName(n))
		}
	}
	return strings.Join(names, ", ")
}

// AddHours devuelve la hora final de una reserva.
func AddHours(hour string, hours int) string {
	t, err := time.Parse("15:04", hour)
	if err != nil {
		return hour
	}
	return fmt.Sprintf("%s", t.Add(time.Duration(hours)*time.Hour).Format("15:04"))
}
