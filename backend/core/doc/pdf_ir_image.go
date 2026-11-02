package doc

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"lazymind/core/common/orm"
	"lazymind/core/pdfir"
	"lazymind/core/store"
)

// GetPDFIRImage serves original figure pixels from this document's retained
// MinerU archive. The only caller-controlled locator is an opaque source block ID.
func GetPDFIRImage(w http.ResponseWriter, r *http.Request) {
	row, ext, _, ok := loadPDFDocument(r, false)
	if !ok {
		formulaExplanationError(w, 404, "Document not found or access denied.")
		return
	}
	expected := r.URL.Query().Get("source_sha256")
	if len(expected) != 64 {
		formulaExplanationError(w, 400, "A source_sha256 is required for paper images.")
		return
	}
	if _, err := hex.DecodeString(expected); err != nil {
		formulaExplanationError(w, 400, "Invalid source_sha256.")
		return
	}
	hash, err := pdfIRHash(ext)
	if err != nil || hash != strings.ToLower(expected) {
		formulaExplanationError(w, 409, "The original PDF changed or is unavailable.")
		return
	}
	state, err := pdfIRStatus(r.Context(), row, hash)
	if err != nil || state.Status != "ready" || state.Document == nil {
		formulaExplanationError(w, 409, "Parse the original PDF before loading its images.")
		return
	}
	blockID := mux.Vars(r)["block"]
	found := false
	for _, block := range state.Document.Blocks {
		if block.ID == blockID && block.Type == "image" && block.Image != nil && block.Image.AssetAvailable {
			found = true
			break
		}
	}
	if !found {
		formulaExplanationError(w, 404, "Original figure pixels are unavailable for this block.")
		return
	}
	var jobs []orm.AsyncJob
	err = store.DB().WithContext(r.Context()).Where("job_type = ? AND resource_type = ? AND resource_id = ?", pdfIRJobType, "document_pdf_ir", pdfIRResource(row.DatasetID, row.ID)).Order("created_at DESC").Find(&jobs).Error
	if err != nil {
		formulaExplanationError(w, 500, "Cannot read the image artifact.")
		return
	}
	archivePath := ""
	for _, job := range jobs {
		var payload pdfIRPayload
		if json.Unmarshal(job.PayloadJSON, &payload) != nil || payload.SourceSHA256 != hash || payload.Revision != pdfir.Revision {
			continue
		}
		var result pdfIRResult
		if job.Status == "succeeded" && json.Unmarshal(job.ResultJSON, &result) == nil {
			archivePath = result.RawPath
		}
		break
	}
	f, err := os.Open(archivePath)
	if err != nil {
		formulaExplanationError(w, 404, "Original figure archive is unavailable.")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(f, pdfir.MaxArchiveBytes+1))
	f.Close()
	if err != nil || int64(len(raw)) > pdfir.MaxArchiveBytes {
		formulaExplanationError(w, 404, "Original figure archive is unavailable.")
		return
	}
	asset, err := pdfir.ReadImageZIP(raw, hash, state.Document.Parser.Model, blockID)
	if err != nil {
		formulaExplanationError(w, 404, "Original figure pixels are unavailable for this block.")
		return
	}
	_, latestExt, _, ok := loadPDFDocument(r, false)
	if !ok {
		formulaExplanationError(w, 404, "Document not found or access denied.")
		return
	}
	latestHash, err := pdfIRHash(latestExt)
	if err != nil || latestHash != hash {
		formulaExplanationError(w, 409, "The original PDF changed while loading the image.")
		return
	}
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", `"`+pdfir.Digest(hash+":"+blockID)+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(asset.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(asset.Data)
}
