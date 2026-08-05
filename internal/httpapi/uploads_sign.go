package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/url"

	"github.com/bpskaltim/eform-backend/internal/models"
	"strconv"
	"strings"
	"time"
)

/* Tautan berkas lampiran yang ditandatangani.

   Sebelumnya /uploads/... disajikan tanpa pemeriksaan apa pun: nama berkasnya memang
   acak, tapi siapa pun yang pernah memegang tautannya bisa mengunduh selamanya, dan
   tautan itu sama sekali tidak terikat pada pembatasan viewer/editor/API key. Artinya
   seluruh aturan masking kolom & filter baris tidak berlaku untuk lampiran.

   Sekarang: pemeriksaan izin dilakukan sekali saat jawaban disajikan, lalu keputusan
   itu "dibawa" oleh tanda tangan HMAC berumur pendek yang ditempelkan ke URL-nya.
   Bentuk ini dipilih karena lampiran dirender lewat <img src> dan <a href>, yang tidak
   bisa mengirim header Authorization. */

// uploadURLTTL adalah masa berlaku tautan lampiran. Cukup panjang untuk membuka dan
// membaca satu halaman jawaban, cukup pendek agar tautan yang bocor cepat mati.
const uploadURLTTL = 2 * time.Hour

// uploadSigKey menurunkan kunci tanda tangan dari JWT secret, supaya tidak ada
// konfigurasi rahasia baru yang harus diatur operator.
func (s *Server) uploadSigKey() []byte {
	sum := sha256.Sum256(append([]byte("eform-upload-sig|"), s.cfg.JWTSecret...))
	return sum[:]
}

func (s *Server) uploadSig(path string, exp int64) string {
	mac := hmac.New(sha256.New, s.uploadSigKey())
	mac.Write([]byte(path))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(exp, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// signUploadURL menambahkan parameter kedaluwarsa + tanda tangan ke path lampiran.
// Nilai yang bukan path /uploads/ dikembalikan apa adanya.
func (s *Server) signUploadURL(raw string) string {
	if !isUploadPath(raw) {
		return raw
	}
	// Buang query lama supaya tanda tangan tidak menumpuk saat data dilewatkan dua kali.
	path := raw
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	exp := time.Now().Add(uploadURLTTL).Unix()
	return path + "?e=" + strconv.FormatInt(exp, 10) + "&s=" + s.uploadSig(path, exp)
}

// verifyUploadURL memeriksa tanda tangan pada permintaan berkas.
func (s *Server) verifyUploadURL(path string, q url.Values) bool {
	expStr, sig := q.Get("e"), q.Get("s")
	if expStr == "" || sig == "" {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	// hmac.Equal, bukan ==, supaya lama pembandingan tidak membocorkan tanda tangan.
	return hmac.Equal([]byte(sig), []byte(s.uploadSig(path, exp)))
}

func isUploadPath(v string) bool {
	return strings.HasPrefix(v, "/uploads/")
}

/* ---- menandatangani lampiran di dalam jawaban ---- */

// signAnswerUploads menulis ulang setiap path /uploads/ di dalam JSON jawaban menjadi
// URL bertanda tangan. Dipanggil TEPAT SETELAH otorisasi, di titik jawaban diserialisasi.
//
// Struktur jawaban bebas (nilai bisa string, array, atau objek), jadi penelusurannya
// rekursif dan hanya menyentuh string yang berbentuk path unggahan.
func (s *Server) signAnswerUploads(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, changed := s.signAny(v)
	if !changed {
		return raw
	}
	b, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return b
}

// signResponse menandatangani lampiran pada satu jawaban.
func (s *Server) signResponse(rr *models.Response) *models.Response {
	if rr != nil {
		rr.Answers = s.signAnswerUploads(rr.Answers)
	}
	return rr
}

// signResponses menandatangani lampiran pada sekumpulan jawaban.
func (s *Server) signResponses(rows []models.Response) []models.Response {
	for i := range rows {
		rows[i].Answers = s.signAnswerUploads(rows[i].Answers)
	}
	return rows
}

func (s *Server) signAny(v any) (any, bool) {
	switch t := v.(type) {
	case string:
		if isUploadPath(t) {
			return s.signUploadURL(t), true
		}
	case []any:
		changed := false
		for i, item := range t {
			nv, c := s.signAny(item)
			if c {
				t[i] = nv
				changed = true
			}
		}
		return t, changed
	case map[string]any:
		changed := false
		for k, item := range t {
			nv, c := s.signAny(item)
			if c {
				t[k] = nv
				changed = true
			}
		}
		return t, changed
	}
	return v, false
}
