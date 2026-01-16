package billing

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

type PaymentOnlyRequest struct {
	NoLab      string       `json:"no_lab"`
	IDKaryawan string       `json:"id_karyawan"`
	Payments   []PaymentRow `json:"payments"`
}

type PaymentRow struct {
	ID       any    `json:"id,omitempty"`
	Tanggal  string `json:"tanggal,omitempty"`
	Bayar    any    `json:"bayar,omitempty"`
	JnsBayar string `json:"jnsbayar,omitempty"`
	Bank     string `json:"bank,omitempty"`
	NoRek    string `json:"no_rek,omitempty"`
	NamaRek  string `json:"nama_rek,omitempty"`
	RekTujuan string `json:"rek_tujuan,omitempty"`
}

type PaymentOnlyResult struct {
	NoLab string
}

type PaymentOnlyService struct {
	JualTable    string
	PaymentTable string
}

func NewPaymentOnlyService() *PaymentOnlyService {
	return &PaymentOnlyService{
		JualTable:    "jual",
		PaymentTable: "bdown_pay",
	}
}

func (s *PaymentOnlyService) SavePaymentOnly(ctx context.Context, tx *sql.Tx, req PaymentOnlyRequest) (PaymentOnlyResult, error) {
	noLab := strings.ToUpper(strings.TrimSpace(req.NoLab))
	if noLab == "" {
		return PaymentOnlyResult{}, NewValidationError("Validation failed.", map[string]string{"no_lab": "no_lab is required."})
	}

	if len(req.Payments) == 0 {
		return PaymentOnlyResult{}, NewValidationError("Validation failed.", map[string]string{"payments": "payments is required and must not be empty."})
	}

	header, err := s.loadJualHeader(ctx, tx, noLab)
	if err != nil {
		return PaymentOnlyResult{}, err
	}

	idKaryawan := strings.TrimSpace(req.IDKaryawan)
	if idKaryawan == "" {
		idKaryawan = strings.TrimSpace(header.KDKasir)
	}
	if idKaryawan == "" {
		return PaymentOnlyResult{}, NewValidationError("Validation failed.", map[string]string{"id_karyawan": "id_karyawan is required (or jual.kd_kasir must exist)."})
	}

	// Filter: skip bayar<=0 when id is empty (avoid spam zero rows). Updates by id are allowed even if bayar=0.
	filtered := make([]normalizedPaymentRow, 0, len(req.Payments))
	errors := map[string]string{}
	for i, p := range req.Payments {
		idStr := normalizeIDToString(p.ID)
		bayarInt, bayarOK := normalizeInt(p.Bayar)
		if !bayarOK {
			errors["payments."+strconv.Itoa(i)+".bayar"] = "bayar must be numeric."
			continue
		}

		if idStr == "" && bayarInt <= 0 {
			continue
		}

		jns := strings.TrimSpace(p.JnsBayar)
		if bayarInt <= 0 || jns == "" {
			jns = "2"
		}

		tanggal := strings.TrimSpace(p.Tanggal)
		if tanggal == "" {
			tanggal = header.Tanggal
			if tanggal == "" {
				tanggal = time.Now().Format("2006/01/02")
			}
		}

		filtered = append(filtered, normalizedPaymentRow{
			ID:        idStr,
			Tanggal:   tanggal,
			Bayar:     bayarInt,
			JnsBayar:  jns,
			Bank:      strings.TrimSpace(p.Bank),
			NoRek:     strings.TrimSpace(p.NoRek),
			NamaRek:   strings.TrimSpace(p.NamaRek),
			RekTujuan: strings.TrimSpace(p.RekTujuan),
		})
	}
	if len(errors) > 0 {
		return PaymentOnlyResult{}, NewValidationError("Validation failed.", errors)
	}

	payload := paymentContext{
		NoLab:      noLab,
		Tanggal:    header.Tanggal,
		KDPs:       header.KDPs,
		KDKasir:    idKaryawan,
		GrandTotal: header.GrandTotal,
		Sisa:       header.Sisa,
	}

	if len(filtered) == 0 {
		if err := s.ensurePaymentRow(ctx, tx, payload); err != nil {
			return PaymentOnlyResult{}, err
		}
	} else {
		if err := s.upsertPayments(ctx, tx, payload, filtered); err != nil {
			return PaymentOnlyResult{}, err
		}
	}

	if err := s.recalculateBayarSisaToJual(ctx, tx, noLab); err != nil {
		return PaymentOnlyResult{}, err
	}

	return PaymentOnlyResult{NoLab: noLab}, nil
}

type jualHeader struct {
	NoLab      string
	Tanggal    string
	KDPs       string
	KDDr       string
	KDKasir    string
	GrandTotal int
	Sisa       int
}

type paymentContext struct {
	NoLab      string
	Tanggal    string
	KDPs       string
	KDKasir    string
	GrandTotal int
	Sisa       int
}

type normalizedPaymentRow struct {
	ID        string
	Tanggal   string
	Bayar     int
	JnsBayar  string
	Bank      string
	NoRek     string
	NamaRek   string
	RekTujuan string
}

func (s *PaymentOnlyService) loadJualHeader(ctx context.Context, tx *sql.Tx, noLab string) (jualHeader, error) {
	row := tx.QueryRowContext(ctx,
		"SELECT no_lab, tanggal, kd_ps, kd_dr, kd_kasir, COALESCE(grandtotal,0), COALESCE(sisa,0) FROM "+s.JualTable+" WHERE no_lab = $1",
		noLab,
	)

	var h jualHeader
	if err := row.Scan(&h.NoLab, &h.Tanggal, &h.KDPs, &h.KDDr, &h.KDKasir, &h.GrandTotal, &h.Sisa); err != nil {
		if err == sql.ErrNoRows {
			return jualHeader{}, NewValidationError("Validation failed.", map[string]string{"no_lab": "no_lab not found."})
		}
		return jualHeader{}, err
	}
	return h, nil
}

func (s *PaymentOnlyService) upsertPayments(ctx context.Context, tx *sql.Tx, payload paymentContext, rows []normalizedPaymentRow) error {
	for _, p := range rows {
		if p.ID == "" {
			_, err := tx.ExecContext(ctx,
				"INSERT INTO "+s.PaymentTable+" (no_lab, grandtotal, bayar, sisa, tanggal, kd_kasir, waktu, lunas, kd_ps, jnsbayar, bank, no_rek, nama_rek, rek_tujuan, jns_kartu, card_name, batchno, no_kartu, mcu_reg, no_tagihan) "+
					"VALUES ($1,$2,$3,$4,$5,$6,'', $7,$8,$9,$10,$11,$12,$13,'','','','', $14, $15)",
				payload.NoLab,
				payload.GrandTotal,
				p.Bayar,
				payload.Sisa,
				p.Tanggal,
				payload.KDKasir,
				p.Bayar,
				payload.KDPs,
				p.JnsBayar,
				p.Bank,
				p.NoRek,
				p.NamaRek,
				p.RekTujuan,
				payload.NoLab, // mcu_reg default
				payload.NoLab, // no_tagihan default
			)
			if err != nil {
				return err
			}
			continue
		}

		// Update by id: follow core behavior (do not overwrite kd_kasir, kd_ps, waktu)
		_, err := tx.ExecContext(ctx,
			"UPDATE "+s.PaymentTable+" SET grandtotal=$1, bayar=$2, sisa=$3, tanggal=$4, lunas=$5, jnsbayar=$6, bank=$7, no_rek=$8, nama_rek=$9, rek_tujuan=$10, mcu_reg=$11, no_tagihan=$12 WHERE id = $13",
			payload.GrandTotal,
			p.Bayar,
			payload.Sisa,
			p.Tanggal,
			p.Bayar,
			p.JnsBayar,
			p.Bank,
			p.NoRek,
			p.NamaRek,
			p.RekTujuan,
			payload.NoLab,
			payload.NoLab,
			p.ID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *PaymentOnlyService) ensurePaymentRow(ctx context.Context, tx *sql.Tx, payload paymentContext) error {
	var id any
	err := tx.QueryRowContext(ctx, "SELECT id FROM "+s.PaymentTable+" WHERE no_lab = $1 LIMIT 1", payload.NoLab).Scan(&id)
	if err == nil {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO "+s.PaymentTable+" (no_lab, grandtotal, bayar, sisa, tanggal, kd_kasir, waktu, lunas, kd_ps, jnsbayar, bank, no_rek, nama_rek, rek_tujuan, jns_kartu, card_name, batchno, no_kartu, mcu_reg, no_tagihan) "+
			"VALUES ($1,$2,0,$3,$4,$5,'',0,$6,'2','','','','','','','','', $7, $8)",
		payload.NoLab,
		payload.GrandTotal,
		payload.Sisa,
		payload.Tanggal,
		payload.KDKasir,
		payload.KDPs,
		payload.NoLab,
		payload.NoLab,
	)
	return err
}

func (s *PaymentOnlyService) recalculateBayarSisaToJual(ctx context.Context, tx *sql.Tx, noLab string) error {
	row := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(grandtotal),0) as grandtotal, COALESCE(SUM(bayar),0) as byr FROM "+s.PaymentTable+" WHERE no_lab = $1 GROUP BY no_lab",
		noLab,
	)

	var grand, byr int
	if err := row.Scan(&grand, &byr); err != nil {
		if err == sql.ErrNoRows {
			// No payments at all, nothing to do.
			return nil
		}
		return err
	}

	sisa := grand - byr
	if sisa < 0 {
		sisa = 0
	}

	_, err := tx.ExecContext(ctx, "UPDATE "+s.JualTable+" SET bayar=$1, sisa=$2 WHERE no_lab = $3", byr, sisa, noLab)
	return err
}

func normalizeIDToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		// json numbers decode as float64
		if t == 0 {
			return ""
		}
		return strconv.FormatInt(int64(t), 10)
	case int:
		if t == 0 {
			return ""
		}
		return strconv.Itoa(t)
	case int64:
		if t == 0 {
			return ""
		}
		return strconv.FormatInt(t, 10)
	default:
		return strings.TrimSpace(toString(t))
	}
}

func normalizeInt(v any) (int, bool) {
	switch t := v.(type) {
	case nil:
		return 0, true
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	case string:
		if strings.TrimSpace(t) == "" {
			return 0, true
		}
		i, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}
