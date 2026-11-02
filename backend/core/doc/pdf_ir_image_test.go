package doc

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"lazymind/core/common/orm"
	"lazymind/core/pdfir"
)

func TestPDFIRImageAuthorizesSourceAndUpgradesRetainedPixels(t *testing.T) {
	installOriginalSourceTransport(t)
	db := newDocumentTestDB(t)
	db.AutoMigrate(&orm.AsyncJob{})
	seedDocumentListDataset(t, db, "ds", "owner")
	seedDocumentListDoc(t, db, "ds", "doc", "paper.pdf", time.Now(), "owner", nil)
	dir := t.TempDir()
	source := filepath.Join(dir, "paper.pdf")
	os.WriteFile(source, []byte("%PDF-original"), 0600)
	db.Model(&orm.Document{}).Where("id = ?", "doc").Update("ext", mustJSON(documentExt{StoredPath: source, OriginalFilename: "paper.pdf"}))
	hash, _ := pdfir.SourceHash(source)
	var pixels, archive bytes.Buffer
	png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 20, 12)))
	zw := zip.NewWriter(&archive)
	f, _ := zw.Create("paper_content_list.json")
	f.Write([]byte(`[{"type":"image","page_idx":0,"bbox":[100,100,900,500],"img_path":"images/figure.png","image_caption":["Figure 1. The method."]}]`))
	f, _ = zw.Create("images/figure.png")
	f.Write(pixels.Bytes())
	zw.Close()
	d, err := pdfir.AdaptZIP(archive.Bytes(), hash, "vlm")
	if err != nil {
		t.Fatal(err)
	}
	blockID := d.Blocks[0].ID
	d.ProjectionVersion = 3
	d.Blocks[0].Image.AssetAvailable = false
	irpath, rawpath := filepath.Join(dir, "ir.json"), filepath.Join(dir, "mineru-result.zip")
	os.WriteFile(irpath, mustJSON(d), 0600)
	os.WriteFile(rawpath, archive.Bytes(), 0600)
	db.Create(&orm.AsyncJob{ID: "job", JobType: pdfIRJobType, ResourceType: "document_pdf_ir", ResourceID: pdfIRResource("ds", "doc"), Status: "succeeded", PayloadJSON: mustJSON(pdfIRPayload{Revision: pdfir.Revision, SourceSHA256: hash}), ResultJSON: mustJSON(pdfIRResult{IRPath: irpath, RawPath: rawpath}), CreatedAt: time.Now()})
	call := func(user, block, expected string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/?source_sha256="+expected, nil)
		r.Header.Set("X-User-Id", user)
		r = mux.SetURLVars(r, map[string]string{"dataset": "ds", "document": "doc", "block": block})
		w := httptest.NewRecorder()
		GetPDFIRImage(w, r)
		return w
	}
	if w := call("stranger", blockID, hash); w.Code != 404 {
		t.Fatalf("ACL bypass %d", w.Code)
	}
	if w := call("owner", blockID, ""); w.Code != 400 {
		t.Fatalf("missing hash accepted %d", w.Code)
	}
	if w := call("owner", blockID, strings.Repeat("a", 64)); w.Code != 409 {
		t.Fatalf("stale source accepted %d", w.Code)
	}
	if w := call("owner", "../../secret", hash); w.Code != 404 {
		t.Fatalf("untrusted locator accepted %d", w.Code)
	}
	w := call("owner", blockID, hash)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || !bytes.Equal(w.Body.Bytes(), pixels.Bytes()) {
		t.Fatalf("wrong image response %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing raster response boundary")
	}
	os.WriteFile(source, []byte("%PDF-replaced"), 0600)
	if w := call("owner", blockID, hash); w.Code != 409 {
		t.Fatalf("replaced source image leaked %d", w.Code)
	}
}
