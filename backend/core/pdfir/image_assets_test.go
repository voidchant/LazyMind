package pdfir

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

func imageArchive(t *testing.T, contentName, content, imageName string, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create(contentName)
	_, _ = f.Write([]byte(content))
	f, _ = z.Create(imageName)
	_, _ = f.Write(data)
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func testImage(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 12, 8))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestImageAssetUsesSourceBoundIDAndTrustedArchivePath(t *testing.T) {
	raw := imageArchive(t, "paper/abc_content_list.json", `[{"type":"image","page_idx":0,"img_path":"images/figure.png","bbox":[0,0,1000,1000]}]`, "paper/images/figure.png", testImage(t))
	d, err := AdaptZIP(raw, "source", "vlm")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Blocks[0].Image.AssetAvailable {
		t.Fatal("real image not exposed")
	}
	asset, err := ReadImageZIP(raw, "source", "vlm", d.Blocks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if asset.ContentType != "image/png" || !bytes.Equal(asset.Data, testImage(t)) {
		t.Fatal("wrong image")
	}
	for _, id := range []string{"images/figure.png", Digest("changed:" + Revision + ":vlm:0")[:24]} {
		if _, err := ReadImageZIP(raw, "source", "vlm", id); err == nil {
			t.Fatal("non-source-bound id accepted")
		}
	}
}
func TestImageAssetRejectsPathsContentAndAmbiguity(t *testing.T) {
	for _, name := range []string{"../figure.png", "/figure.png", "images\\figure.png", "https://evil/figure.png", "images/../../figure.png"} {
		raw := imageArchive(t, "paper_content_list.json", `[{"type":"image","page_idx":0,"img_path":"`+strings.ReplaceAll(name, "\\", "\\\\")+`"}]`, "images/figure.png", testImage(t))
		if _, err := ReadImageZIP(raw, "source", "vlm", Digest("source:" + Revision + ":vlm:0")[:24]); err == nil {
			t.Fatalf("unsafe path accepted: %s", name)
		}
	}
	raw := imageArchive(t, "paper_content_list.json", `[{"type":"image","page_idx":0,"img_path":"figure.png"}]`, "figure.png", []byte(`<svg onload="bad()"/>`))
	if _, err := ReadImageZIP(raw, "source", "vlm", Digest("source:" + Revision + ":vlm:0")[:24]); err == nil {
		t.Fatal("non-raster payload accepted")
	}
	d, err := AdaptZIP(raw, "source", "vlm")
	if err != nil {
		t.Fatal(err)
	}
	if d.Blocks[0].Image.AssetAvailable {
		t.Fatal("invalid image marked available")
	}
}
