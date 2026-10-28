package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// InvoiceData contiene todo lo necesario para renderizar una factura.
type InvoiceData struct {
	Number       string
	IssuedAt     string
	CustomerName string
	CustomerMail string
	CustomerDoc  string
	CourtName    string
	CourtSport   string
	CourtCity    string
	OwnerName    string
	BusinessName string
	OwnerNIT     string
	Date         string
	StartTime    string
	EndTime      string
	Hours        int
	PricePerHour string
	Total        string
	Commission   string
	Net          string
	Method       string
	BookingID    uint
}

// GenerateInvoice crea un PDF limpio y devuelve la ruta escrita.
func GenerateInvoice(data InvoiceData, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	filename := strings.ReplaceAll(data.Number, "/", "-") + ".pdf"
	outPath := filepath.Join(dir, filename)

	p := gofpdf.New("P", "mm", "A4", "")
	tr := p.UnicodeTranslatorFromDescriptor("")
	p.SetMargins(18, 16, 18)
	p.SetAutoPageBreak(true, 20)
	p.AddPage()

	// Encabezado de marca.
	p.SetFillColor(15, 118, 110)
	p.Rect(0, 0, 210, 40, "F")
	p.SetTextColor(255, 255, 255)
	p.SetFont("Helvetica", "B", 22)
	p.SetXY(18, 12)
	p.CellFormat(120, 10, tr("Sportss Colombia"), "", 0, "L", false, 0, "")
	p.SetFont("Helvetica", "", 9)
	p.SetXY(18, 24)
	p.CellFormat(120, 6, tr("Reserva tu juego. Vive el deporte."), "", 0, "L", false, 0, "")
	p.SetFont("Helvetica", "B", 13)
	p.SetXY(130, 13)
	p.CellFormat(62, 7, tr("FACTURA DE VENTA"), "", 1, "R", false, 0, "")
	p.SetFont("Helvetica", "", 9)
	p.SetXY(130, 21)
	p.CellFormat(62, 6, tr(data.Number), "", 1, "R", false, 0, "")
	p.SetXY(130, 27)
	p.CellFormat(62, 6, tr("Emitida: "+data.IssuedAt), "", 1, "R", false, 0, "")

	p.SetTextColor(31, 41, 55)
	p.SetY(50)
	p.SetFont("Helvetica", "B", 10)
	p.CellFormat(87, 7, tr("CLIENTE"), "", 0, "L", false, 0, "")
	p.CellFormat(87, 7, tr("PROPIETARIO / ESTABLECIMIENTO"), "", 1, "L", false, 0, "")
	p.SetFont("Helvetica", "", 9)
	p.SetX(18)
	p.MultiCell(87, 5, tr(fmt.Sprintf("%s\n%s\nDocumento: %s", data.CustomerName, data.CustomerMail, emptyDash(data.CustomerDoc))), "", "L", false)
	p.SetXY(105, 57)
	p.MultiCell(87, 5, tr(fmt.Sprintf("%s\n%s\nNIT: %s", data.BusinessName, data.OwnerName, emptyDash(data.OwnerNIT))), "", "L", false)

	p.SetY(82)
	p.SetDrawColor(229, 231, 235)
	p.Line(18, p.GetY(), 192, p.GetY())
	p.Ln(5)
	p.SetFont("Helvetica", "B", 10)
	p.CellFormat(174, 7, tr("DETALLE DE LA RESERVA"), "", 1, "L", false, 0, "")

	// Tabla de detalle.
	p.SetFillColor(243, 244, 246)
	p.SetFont("Helvetica", "B", 9)
	p.CellFormat(78, 8, tr("Cancha"), "1", 0, "L", true, 0, "")
	p.CellFormat(28, 8, tr("Fecha"), "1", 0, "C", true, 0, "")
	p.CellFormat(28, 8, tr("Horario"), "1", 0, "C", true, 0, "")
	p.CellFormat(16, 8, tr("Horas"), "1", 0, "C", true, 0, "")
	p.CellFormat(24, 8, tr("Precio/h"), "1", 1, "R", true, 0, "")
	p.SetFont("Helvetica", "", 9)
	p.CellFormat(78, 9, tr(truncate(data.CourtName, 42)), "1", 0, "L", false, 0, "")
	p.CellFormat(28, 9, tr(data.Date), "1", 0, "C", false, 0, "")
	p.CellFormat(28, 9, tr(data.StartTime+" - "+data.EndTime), "1", 0, "C", false, 0, "")
	p.CellFormat(16, 9, tr(fmt.Sprintf("%d", data.Hours)), "1", 0, "C", false, 0, "")
	p.CellFormat(24, 9, tr(data.PricePerHour), "1", 1, "R", false, 0, "")

	p.SetFont("Helvetica", "", 9)
	p.SetTextColor(75, 85, 99)
	p.CellFormat(174, 8, tr(fmt.Sprintf("%s | %s | Reserva #%d", data.CourtSport, data.CourtCity, data.BookingID)), "", 1, "L", false, 0, "")

	// Totales alineados a la derecha.
	p.SetTextColor(31, 41, 55)
	p.SetY(p.GetY() + 6)
	p.SetFont("Helvetica", "", 9)
	label := func(l, v string, bold bool) {
		p.SetX(118)
		if bold {
			p.SetFont("Helvetica", "B", 10)
		} else {
			p.SetFont("Helvetica", "", 9)
		}
		p.CellFormat(42, 7, tr(l), "", 0, "R", false, 0, "")
		p.CellFormat(32, 7, tr(v), "", 1, "R", false, 0, "")
	}
	label("Valor de la reserva", data.Total, false)
	label("Comisión plataforma", data.Commission, false)
	label("Neto para el propietario", data.Net, false)
	p.SetDrawColor(15, 118, 110)
	p.Line(118, p.GetY()+1, 192, p.GetY()+1)
	p.Ln(3)
	label("TOTAL PAGADO", data.Total, true)
	label("Método de pago", data.Method, false)

	p.SetY(p.GetY() + 12)
	p.SetFillColor(236, 253, 245)
	p.SetTextColor(6, 95, 70)
	p.SetFont("Helvetica", "B", 9)
	p.CellFormat(174, 10, tr("Pago aprobado. ¡Gracias por reservar con Sportss Colombia!"), "", 1, "C", true, 0, "")

	p.SetY(270)
	p.SetTextColor(107, 114, 128)
	p.SetFont("Helvetica", "", 8)
	p.MultiCell(174, 4, tr("Esta factura fue generada automáticamente por Sportss Colombia. "+
		"El valor incluye la comisión de servicio de la plataforma. Para soporte escribe a hola@sportss.co."), "", "C", false)

	if err := p.OutputFileAndClose(outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "."
}
