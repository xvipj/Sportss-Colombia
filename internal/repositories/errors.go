package repositories

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrNotFound  = errors.New("registro no encontrado")
	ErrDuplicate = errors.New("el registro ya existe o el horario está ocupado")
)

// translateError normaliza errores de GORM para que los servicios no dependan
// del driver de base de datos.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate") {
		return ErrDuplicate
	}
	return err
}
