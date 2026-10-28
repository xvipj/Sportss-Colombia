package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"sportss/internal/config"
	"sportss/internal/models"
	"sportss/internal/repositories"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]{2,}$`)

type AuthService struct {
	cfg   *config.Config
	users repositories.UserRepository
}

type RegisterInput struct {
	Name            string
	Email           string
	Password        string
	ConfirmPassword string
	AccountType     string
	BusinessName    string
	NIT             string
	Phone           string
}

type ProfileInput struct {
	Name         string
	BusinessName string
	NIT          string
	Phone        string
	EmailNotify  bool
	AvatarPath   string
}

type Claims struct {
	UserID uint   `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(cfg *config.Config, users repositories.UserRepository) *AuthService {
	return &AuthService{cfg: cfg, users: users}
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*models.User, string, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.AccountType = strings.ToLower(strings.TrimSpace(in.AccountType))

	if len(in.Name) < 2 {
		return nil, "", BadRequest("Escribe tu nombre completo.")
	}
	if !emailPattern.MatchString(in.Email) {
		return nil, "", BadRequest("El correo electrónico no tiene un formato válido.")
	}
	if len(in.Password) < 8 {
		return nil, "", BadRequest("La contraseña debe tener al menos 8 caracteres.")
	}
	if in.Password != in.ConfirmPassword {
		return nil, "", BadRequest("Las contraseñas no coinciden.")
	}
	if in.AccountType != models.AccountPerson && in.AccountType != models.AccountBusiness {
		return nil, "", BadRequest("Selecciona un tipo de cuenta válido.")
	}

	role := models.RolePlayer
	if in.AccountType == models.AccountBusiness {
		role = models.RoleBusiness
		in.BusinessName = strings.TrimSpace(in.BusinessName)
		in.Phone = strings.TrimSpace(in.Phone)
		if len(in.BusinessName) < 2 {
			return nil, "", BadRequest("Escribe el nombre de tu negocio.")
		}
		if len(in.Phone) < 7 {
			return nil, "", BadRequest("Escribe un teléfono de contacto válido.")
		}
	} else {
		in.BusinessName, in.NIT, in.Phone = "", "", ""
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", Internal("No fue posible proteger la contraseña", err)
	}

	user := &models.User{
		Name: in.Name, Email: in.Email, PasswordHash: string(hash), Role: role,
		AccountType: in.AccountType, BusinessName: in.BusinessName, NIT: strings.TrimSpace(in.NIT),
		Phone: in.Phone, Active: true, EmailNotify: true,
	}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, repositories.ErrDuplicate) {
			return nil, "", Conflict("Ya existe una cuenta con este correo.")
		}
		return nil, "", Internal("No fue posible crear la cuenta", err)
	}
	token, err := s.GenerateToken(user)
	return user, token, err
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	user, err := s.users.FindByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, "", Unauthorized("Correo o contraseña incorrectos.")
		}
		return nil, "", Internal("No fue posible iniciar sesión", err)
	}
	if !user.Active {
		return nil, "", Forbidden("Tu cuenta está bloqueada. Contacta al administrador.")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", Unauthorized("Correo o contraseña incorrectos.")
	}
	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", Internal("No fue posible generar la sesión", err)
	}
	return user, token, nil
}

func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			Issuer:    "sportss-colombia",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) Authenticate(ctx context.Context, tokenString string) (*models.User, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma inesperado")
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, Unauthorized("La sesión no es válida o expiró.")
	}
	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, Unauthorized("La cuenta ya no está disponible.")
	}
	if !user.Active {
		return nil, Forbidden("Tu cuenta está bloqueada.")
	}
	return user, nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, user *models.User, in ProfileInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 {
		return BadRequest("Escribe un nombre válido.")
	}
	user.Name = in.Name
	user.Phone = strings.TrimSpace(in.Phone)
	user.EmailNotify = in.EmailNotify
	if user.Role == models.RoleBusiness || user.Role == models.RoleAdmin {
		in.BusinessName = strings.TrimSpace(in.BusinessName)
		if len(in.BusinessName) < 2 {
			return BadRequest("El nombre del negocio es obligatorio.")
		}
		user.BusinessName = in.BusinessName
		user.NIT = strings.TrimSpace(in.NIT)
	}
	if in.AvatarPath != "" {
		user.AvatarPath = in.AvatarPath
	}
	if err := s.users.Update(ctx, user); err != nil {
		return Internal("No fue posible actualizar el perfil", err)
	}
	return nil
}

func (s *AuthService) ChangePassword(ctx context.Context, user *models.User, current, next, confirm string) error {
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		return BadRequest("La contraseña actual no es correcta.")
	}
	if len(next) < 8 {
		return BadRequest("La nueva contraseña debe tener al menos 8 caracteres.")
	}
	if next != confirm {
		return BadRequest("La confirmación de la nueva contraseña no coincide.")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return Internal("No fue posible actualizar la contraseña", err)
	}
	if err := s.users.SetPassword(ctx, user.ID, string(hash)); err != nil {
		return Internal("No fue posible actualizar la contraseña", err)
	}
	return nil
}

func (s *AuthService) SetUserActive(ctx context.Context, id uint, active bool) error {
	if err := s.users.SetActive(ctx, id, active); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return NotFound("Usuario no encontrado.")
		}
		return Internal("No fue posible cambiar el estado del usuario", err)
	}
	return nil
}

func (s *AuthService) ListUsers(ctx context.Context, search string, page, limit int) ([]models.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	users, err := s.users.List(ctx, search, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, Internal("No fue posible cargar los usuarios", err)
	}
	total, err := s.users.Count(ctx, search)
	if err != nil {
		return nil, 0, Internal("No fue posible contar los usuarios", err)
	}
	return users, total, nil
}

func (s *AuthService) CountUsers(ctx context.Context) (int64, error) {
	n, err := s.users.CountAll(ctx)
	if err != nil {
		return 0, Internal("No fue posible contar los usuarios", err)
	}
	return n, nil
}
