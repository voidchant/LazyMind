package pdfir

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"path"
	"strings"
)

const MaxImageBytes int64 = 16 << 20

// ImageAsset contains original raster bytes from the source-matched MinerU job.
// Neither the public IR nor callers supply an archive or filesystem path.
type ImageAsset struct {
	Data        []byte
	ContentType string
}

func safeImageMember(name string) bool {
	return name != "" && !path.IsAbs(name) && !strings.ContainsAny(name, "\\:\x00") && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../")
}
func imageMember(z *zip.Reader, directory, reference string) *zip.File {
	if !safeImageMember(reference) {
		return nil
	}
	ext := strings.ToLower(path.Ext(reference))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return nil
	}
	candidates := map[string]bool{reference: true, path.Join(directory, reference): true}
	var found *zip.File
	for _, f := range z.File {
		if !candidates[f.Name] {
			continue
		}
		if found != nil || f.FileInfo().IsDir() || f.UncompressedSize64 > uint64(MaxImageBytes) {
			return nil
		}
		found = f
	}
	return found
}
func readImageMember(f *zip.File) (ImageAsset, error) {
	if f == nil {
		return ImageAsset{}, errors.New("source image is unavailable")
	}
	r, err := f.Open()
	if err != nil {
		return ImageAsset{}, err
	}
	defer r.Close()
	raw, err := bounded(r, MaxImageBytes)
	if err != nil {
		return ImageAsset{}, err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 16000 || cfg.Height > 16000 || int64(cfg.Width)*int64(cfg.Height) > 32_000_000 {
		return ImageAsset{}, errors.New("invalid or oversized raster image")
	}
	return ImageAsset{Data: raw, ContentType: "image/" + format}, nil
}
func projectImageAssets(d *Document, items []contentItem, z *zip.Reader, directory string) {
	for i, item := range items {
		if d.Blocks[i].Image == nil {
			continue
		}
		_, err := readImageMember(imageMember(z, directory, item.ImagePath))
		d.Blocks[i].Image.AssetAvailable = err == nil
	}
}

// ReadImageZIP resolves an opaque block ID against the retained content list.
// Files are read inside the ZIP, never extracted to a caller-controlled path.
func ReadImageZIP(raw []byte, sourceHash, model, blockID string) (ImageAsset, error) {
	if int64(len(raw)) > MaxArchiveBytes {
		return ImageAsset{}, errors.New("archive too large")
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return ImageAsset{}, err
	}
	var content *zip.File
	for _, f := range z.File {
		if !safeImageMember(strings.TrimSuffix(f.Name, "/")) {
			return ImageAsset{}, errors.New("unsafe archive member")
		}
		if strings.HasSuffix(f.Name, "_content_list.json") {
			if content != nil {
				return ImageAsset{}, errors.New("ambiguous content lists")
			}
			content = f
		}
	}
	if content == nil {
		return ImageAsset{}, errors.New("missing content list")
	}
	r, err := content.Open()
	if err != nil {
		return ImageAsset{}, err
	}
	data, err := bounded(r, maxJSONBytes)
	r.Close()
	if err != nil {
		return ImageAsset{}, err
	}
	var items []contentItem
	if json.Unmarshal(data, &items) != nil {
		return ImageAsset{}, errors.New("invalid content list")
	}
	for i, item := range items {
		if Digest(fmt.Sprintf("%s:%s:%s:%d", sourceHash, Revision, model, i))[:24] != blockID {
			continue
		}
		if item.Type != "image" || item.Page == nil || *item.Page < 0 {
			break
		}
		return readImageMember(imageMember(z, path.Dir(content.Name), item.ImagePath))
	}
	return ImageAsset{}, errors.New("source image not found")
}
