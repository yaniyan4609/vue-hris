package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/cryptodata"
)

type employeePersonalWrite struct {
	Nickname      *string `json:"nickname"`
	Gender        *string `json:"gender"`
	BirthDate     *string `json:"birth_date"`
	BirthPlace    *string `json:"birth_place"`
	MaritalStatus *string `json:"marital_status"`
	WorkEmail     *string `json:"work_email"`
	Mobile        *string `json:"mobile"`
	JoinDate      *string `json:"join_date"`
	EndDate       *string `json:"end_date"`
	PhotoFileID   *string `json:"photo_file_id"`
	NIK           *string `json:"nik"`
}

type employeeWrite struct {
	employee
	employeePersonalWrite
}

var nikPattern = regexp.MustCompile(`^[0-9]{16}$`)

func validEmployeePersonal(p employeePersonalWrite) error {
	for _, item := range []struct {
		name  string
		value *string
		max   int
	}{
		{"nickname", p.Nickname, 120}, {"birth_place", p.BirthPlace, 160},
		{"work_email", p.WorkEmail, 320}, {"mobile", p.Mobile, 32},
	} {
		if item.value != nil && len([]rune(strings.TrimSpace(*item.value))) > item.max {
			return fmt.Errorf("%s terlalu panjang", item.name)
		}
	}
	if p.Gender != nil && *p.Gender != "" && *p.Gender != "male" && *p.Gender != "female" && *p.Gender != "other" {
		return errors.New("gender tidak valid")
	}
	if p.MaritalStatus != nil && *p.MaritalStatus != "" && *p.MaritalStatus != "single" && *p.MaritalStatus != "married" && *p.MaritalStatus != "divorced" && *p.MaritalStatus != "widowed" {
		return errors.New("marital_status tidak valid")
	}
	for _, item := range []*string{p.BirthDate, p.JoinDate, p.EndDate} {
		if item != nil && *item != "" {
			date, err := time.Parse("2006-01-02", *item)
			if err != nil || date.Format("2006-01-02") != *item {
				return errors.New("tanggal profil tidak valid")
			}
		}
	}
	if p.PhotoFileID != nil && *p.PhotoFileID != "" && !fileIDPattern.MatchString(*p.PhotoFileID) {
		return errors.New("photo_file_id tidak valid")
	}
	if p.NIK != nil && *p.NIK != "" && !nikPattern.MatchString(*p.NIK) {
		return errors.New("NIK harus 16 digit")
	}
	if p.WorkEmail != nil && *p.WorkEmail != "" && (!strings.Contains(*p.WorkEmail, "@") || strings.ContainsAny(*p.WorkEmail, " \t\n")) {
		return errors.New("email kerja tidak valid")
	}
	return nil
}

func nullablePersonal(value string) any {
	if value == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func employeeSearchKey(ctx context.Context, tx *sql.Tx, tenantID string) ([]byte, error) {
	master, masterID, err := tenantMasterKey()
	if err != nil {
		return nil, err
	}
	var storedID string
	var wrapped []byte
	err = tx.QueryRowContext(ctx, `SELECT master_key_id,wrapped_search_key FROM tenant_search_keys WHERE tenant_id=?`, tenantID).Scan(&storedID, &wrapped)
	if err == nil {
		if storedID != masterID {
			return nil, errors.New("search key master mismatch")
		}
		return cryptodata.UnwrapSearchKey(master, wrapped, storedID, tenantID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	key, err := cryptodata.NewDataKey()
	if err != nil {
		return nil, err
	}
	wrapped, err = cryptodata.WrapSearchKey(master, key, masterID, tenantID)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO tenant_search_keys (tenant_id,master_key_id,wrapped_search_key) VALUES (?,?,?)`, tenantID, masterID, wrapped)
	return key, err
}

// The tenant row is locked before key creation; callers never audit plaintext NIK.
func persistEmployeePersonal(ctx context.Context, tx *sql.Tx, tenantID, employeeID string, p employeePersonalWrite) error {
	columns := map[string]*string{"nickname": p.Nickname, "gender": p.Gender, "birth_date": p.BirthDate, "birth_place": p.BirthPlace, "marital_status": p.MaritalStatus, "work_email": p.WorkEmail, "mobile": p.Mobile, "join_date": p.JoinDate, "end_date": p.EndDate, "photo_file_id": p.PhotoFileID}
	for _, column := range []string{"nickname", "gender", "birth_date", "birth_place", "marital_status", "work_email", "mobile", "join_date", "end_date", "photo_file_id"} {
		if value := columns[column]; value != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE employees SET `+column+`=? WHERE tenant_id=? AND id=?`, nullablePersonal(*value), tenantID, employeeID); err != nil {
				return err
			}
		}
	}
	if p.NIK == nil {
		return nil
	}
	var locked string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM tenants WHERE id=? FOR UPDATE`, tenantID).Scan(&locked); err != nil {
		return err
	}
	if *p.NIK == "" {
		_, err := tx.ExecContext(ctx, `UPDATE employees SET nik_enc=NULL,nik_key_version=NULL,nik_hash=NULL WHERE tenant_id=? AND id=?`, tenantID, employeeID)
		return err
	}
	searchKey, err := employeeSearchKey(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	index, err := cryptodata.BlindIndex(searchKey, tenantID, *p.NIK)
	if err != nil {
		return err
	}
	var duplicate int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM employees WHERE tenant_id=? AND nik_hash=? AND id<>?`, tenantID, index[:], employeeID).Scan(&duplicate); err != nil {
		return err
	}
	if duplicate > 0 {
		return errors.New("NIK_DUPLICATE")
	}
	dataKey, version, err := activeTenantDataKey(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	ciphertext, err := cryptodata.EncryptField(dataKey, tenantID, employeeID, "nik", version, []byte(*p.NIK))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE employees SET nik_enc=?,nik_key_version=?,nik_hash=? WHERE tenant_id=? AND id=?`, ciphertext, version, index[:], tenantID, employeeID)
	return err
}

func (a adminAPI) checkNIKWrite(w http.ResponseWriter, r *http.Request, c adminContext, employeeID string, target *access.Target, nik *string) bool {
	if nik == nil {
		return true
	}
	if _, _, err := tenantMasterKey(); err != nil {
		writeAPIError(w, r, 503, "KEY_UNAVAILABLE", "Kunci data sensitif tidak tersedia.")
		return false
	}
	subject, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "karyawan.lihat_sensitif")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa izin sensitif.")
		return false
	}
	if !decision.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak data sensitif diperlukan.")
		return false
	}
	if target != nil && !decision.Includes(subject, *target) {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Data sensitif di luar scope.")
		return false
	}
	if employeeID != "" {
		filter, args := access.EmployeeWhere(subject, decision)
		var count int
		err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e`+employeeScopeJoin+` WHERE e.tenant_id=? AND e.id=? AND `+filter, append([]any{c.tenant.ID, employeeID}, args...)...).Scan(&count)
		if err != nil || count == 0 {
			writeAPIError(w, r, 403, "SCOPE_DENIED", "Data sensitif di luar scope.")
			return false
		}
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum mengubah NIK.")
		return false
	}
	return true
}

func (a adminAPI) employeePersonalDetail(w http.ResponseWriter, r *http.Request, c adminContext, id string, base employee) {
	var nickname, gender, birthDate, birthPlace, marital, workEmail, mobile, joinDate, endDate, photo sql.NullString
	var cipher []byte
	var version sql.NullInt64
	err := a.db.QueryRowContext(r.Context(), `SELECT nickname,gender,DATE_FORMAT(birth_date,'%Y-%m-%d'),birth_place,marital_status,work_email,mobile,DATE_FORMAT(join_date,'%Y-%m-%d'),DATE_FORMAT(end_date,'%Y-%m-%d'),photo_file_id,nik_enc,nik_key_version FROM employees WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&nickname, &gender, &birthDate, &birthPlace, &marital, &workEmail, &mobile, &joinDate, &endDate, &photo, &cipher, &version)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca profil karyawan.")
		return
	}
	result := map[string]any{"id": base.ID, "employeeNumber": base.EmployeeNumber, "displayName": base.DisplayName, "status": base.Status, "baseOfficeId": base.BaseOfficeID, "officeName": base.OfficeName, "positionId": base.PositionID, "positionName": base.PositionName, "reportsToId": base.ReportsToID, "userId": base.UserID, "nickname": nullableNullString(nickname), "gender": nullableNullString(gender), "birth_date": nullableNullString(birthDate), "birth_place": nullableNullString(birthPlace), "marital_status": nullableNullString(marital), "work_email": nullableNullString(workEmail), "mobile": nullableNullString(mobile), "join_date": nullableNullString(joinDate), "end_date": nullableNullString(endDate), "photo_file_id": nullableNullString(photo), "nik": nil}
	if len(cipher) > 0 && version.Valid {
		result["nik"] = "************"
		subject, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "karyawan.lihat_sensitif")
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa izin sensitif.")
			return
		}
		if decision.Allowed {
			filter, args := access.EmployeeWhere(subject, decision)
			var count int
			err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e`+employeeScopeJoin+` WHERE e.tenant_id=? AND e.id=? AND `+filter, append([]any{c.tenant.ID, id}, args...)...).Scan(&count)
			if err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope sensitif.")
				return
			}
			if count > 0 {
				key, err := loadTenantDataKey(r.Context(), a.db, c.tenant.ID, uint32(version.Int64))
				if err != nil {
					writeAPIError(w, r, 500, "KEY_UNAVAILABLE", "Kunci data tidak tersedia.")
					return
				}
				plain, err := cryptodata.DecryptField(key, cipher, c.tenant.ID, id, "nik", uint32(version.Int64))
				if err != nil {
					writeAPIError(w, r, 500, "DECRYPT_FAILED", "Data sensitif tidak dapat dibaca.")
					return
				}
				result["nik"] = string(plain)
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, result)
}

func nullableNullString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}
