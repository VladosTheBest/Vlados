package service

import (
	"bytes"
	"fmt"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"time"

	"encoding/csv"
	"github.com/jung-kurt/gofpdf"
)

func CSVExport(data [][]string) ([]byte, error) {
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)

	for _, value := range data {
		if err := writer.Write(value); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func PDFExport(data [][]string, columnWidths []int, name string) ([]byte, error) {
	buf := bytes.Buffer{}

	// Create a new PDF document and write the title and the current date.
	pdf := newReport(name)

	// After that, create the table header and write data.
	pdf = header(pdf, data[0], columnWidths)
	pdf = table(pdf, data[1:], columnWidths)

	err := pdf.Output(&buf)

	return buf.Bytes(), err
}

func newReport(name string) *gofpdf.Fpdf {
	// * landscape ("L") or portrait ("P") orientation,
	// * the unit used for expressing lengths and sizes ("mm"),
	// * the paper format ("Letter"), and
	// * the path to a font directory.
	pdf := gofpdf.New("L", "mm", "Legal", "")

	// add a new page to the document.
	pdf.AddPage()

	// Set the font to "Times", the style to "bold", and the size to 28 points.
	pdf.SetFont("Times", "B", 28)

	pdf.Cell(40, 10, name)

	// The `Ln()` function moves the current position to a new line, with
	// an optional line height parameter.
	pdf.Ln(12)

	pdf.SetFont("Times", "", 20)
	pdf.Cell(40, 10, time.Now().Format("2 Jan 2006 15:04:05"))
	pdf.Ln(20)

	return pdf
}

func header(pdf *gofpdf.Fpdf, hdr []string, widths []int) *gofpdf.Fpdf {
	pdf.SetFont("Times", "B", 12)
	pdf.SetFillColor(240, 240, 240)

	for i, str := range hdr {
		// The `CellFormat()` method takes parameters to format the cell.
		// Create a visible border around
		// the cell, and to enable the background fill.
		pdf.CellFormat(float64(widths[i]), 7, str, "1", 0, "", true, 0, "")
	}

	// `-1` to `Ln()` uses the height of the last printed cell as
	// the line height.
	pdf.Ln(-1)
	return pdf
}

func table(pdf *gofpdf.Fpdf, tbl [][]string, widths []int) *gofpdf.Fpdf {
	// Reset font and fill color.
	pdf.SetFont("Times", "", 12)
	pdf.SetFillColor(255, 255, 255)

	for _, line := range tbl {
		for i, str := range line {
			pdf.CellFormat(float64(widths[i]), 7, str, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}
	return pdf
}

func PDFExportWithdrawPaymentReceipt(user *model.User, transaction *model.ClearJunctionRequest, country string, cfg config.ClearJunctionConfig) ([]byte, error) {
	buf := bytes.Buffer{}

	// Create a new PDF document and write the title and the current date.
	pdf := newPaymentReceipt(*user, transaction, country, cfg)

	err := pdf.Output(&buf)

	return buf.Bytes(), err
}

func newPaymentReceipt(user model.User, transaction *model.ClearJunctionRequest, country string, cfg config.ClearJunctionConfig) *gofpdf.Fpdf {
	pdf := gofpdf.New("P", "mm", "Legal", "")

	pdf.AddPage()

	pdf.Image("logo.png", 10, 10, 0, 0, false, "", 0, "")

	pdf.SetFont("Arial", "B", 12)
	pdf.Ln(20)
	pdf.CellFormat(150, 7, "IDENTIFIER", "0", 0, "L", false, 0, "")
	pdf.Ln(7)
	pdf.SetFillColor(148, 222, 242)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(113.5, 7, fmt.Sprintf("Name: %s", user.FullName()), "0", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(50, 7, "Receipt of payment", "1", 0, "L", true, 0, "")
	pdf.Ln(7)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(113.5, 7, fmt.Sprintf("Email: %s", user.Email), "0", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("Date: %s", transaction.CreatedAt.Format("2 Jan 2006")), "", 0, "L", false, 0, "")
	pdf.Ln(7)
	pdf.CellFormat(113.5, 7, fmt.Sprintf("Reference: %s", transaction.OrderRefId), "0", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("Bank Name: %s", cfg.Requisites.BankName), "", 0, "L", false, 0, "")
	pdf.Ln(7)
	pdf.CellFormat(113.5, 7, fmt.Sprintf("Country: %s", country), "0", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("IBAN: %s", cfg.Requisites.Iban), "", 0, "L", false, 0, "")
	pdf.Ln(7)
	pdf.CellFormat(149.5, 7, fmt.Sprintf("Swift/BIC: %s", cfg.Requisites.Swift), "", 0, "R", false, 0, "")
	pdf.Ln(7)
	pdf.CellFormat(200, 7, fmt.Sprintf("Bank Address: %s", cfg.Requisites.Address), "", 0, "R", false, 0, "")
	pdf.Ln(40)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(150, 7, "Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(0, 7, "Amount", "1", 0, "R", true, 0, "")
	pdf.Ln(7)
	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(150, 15, "Payment for service of exchanging a virtual currency against a fiat currency", "1", 0, "L", false, 0, "")
	pdf.CellFormat(0, 15, utils.FmtDecimalWithPrecision(transaction.Amount, 2), "1", 0, "R", false, 0, "")
	pdf.Ln(15)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 7, fmt.Sprintf("Invoice total (EUR) %s", utils.FmtDecimalWithPrecision(transaction.Amount, 2)), "", 0, "R", false, 0, "")
	pdf.SetLineWidth(0.5)
	pdf.Line(10, 170, 200, 170)
	pdf.Ln(40)
	pdf.SetFont("Arial", "B", 15)
	pdf.CellFormat(150, 15, "ParamountDax OU", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 15, "European Licensed Crypto Trading Platform", "", 0, "R", false, 0, "")
	pdf.Ln(10)
	pdf.CellFormat(150, 15, "Katusepapi tn 6-331, Lasnamae linnaosa,", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 15, "Operating Licence: FVT000492", "", 0, "R", false, 0, "")
	pdf.Ln(5)
	pdf.CellFormat(150, 15, "Tallinn, Harju maakond, 11412", "", 0, "L", false, 0, "")

	return pdf
}

func PDFExportContracts(data [][]string, columnWidths []int, name string, tradeData [][]string, tradeColumnWidths []int, name2 string) ([]byte, error) {
	buf := bytes.Buffer{}

	// Create a new PDF document and write the title and the current date.
	pdf := newReport(name)

	// After that, create the table header and write data.
	pdf = header(pdf, data[0], columnWidths)
	pdf = table(pdf, data[1:], columnWidths)
	pdf.Ln(10)
	pdf.SetFont("Times", "B", 20)
	pdf.Cell(40, 10, name2)
	pdf.Ln(10)
	pdf = header(pdf, tradeData[0], tradeColumnWidths)
	pdf = table(pdf, tradeData[1:], tradeColumnWidths)

	err := pdf.Output(&buf)

	return buf.Bytes(), err
}
