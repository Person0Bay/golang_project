package service

import (
	"fmt"

	"github.com/skip2/go-qrcode"
)

type QRGenerator interface {
	Generate(orderID int) ([]byte, error)
}

type DefaultQRGenerator struct {
	BaseURL string
}

func (g DefaultQRGenerator) Generate(orderID int) ([]byte, error) {
	qrData := fmt.Sprintf("%s/review.html?check_id=%d", g.BaseURL, orderID)
	return qrcode.Encode(qrData, qrcode.Medium, 256)
}
