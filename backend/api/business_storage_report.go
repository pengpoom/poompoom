package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"imagestudio/internal/businessimage"
)

type businessStorageReport struct {
	Summary               businessStorageReportSummary       `json:"summary"`
	MissingFiles          []businessStorageReportFile        `json:"missingFiles"`
	OrphanFiles           []businessStorageReportDiskFile    `json:"orphanFiles"`
	LegacyReferencedFiles []businessStorageReportReference   `json:"legacyReferencedFiles"`
	BrokenAssets          []businessStorageReportBrokenAsset `json:"brokenAssets"`
	Directories           []string                           `json:"directories"`
}

type businessStorageReportSummary struct {
	AssetFiles            int   `json:"assetFiles"`
	DiskFiles             int   `json:"diskFiles"`
	ReferencedFiles       int   `json:"referencedFiles"`
	MissingFiles          int   `json:"missingFiles"`
	OrphanFiles           int   `json:"orphanFiles"`
	LegacyReferencedFiles int   `json:"legacyReferencedFiles"`
	BrokenAssets          int   `json:"brokenAssets"`
	AssetBytes            int64 `json:"assetBytes"`
	DiskBytes             int64 `json:"diskBytes"`
	OrphanBytes           int64 `json:"orphanBytes"`
}

type businessStorageReportFile struct {
	FileName     string `json:"fileName"`
	UserID       string `json:"userId,omitempty"`
	GenerationID string `json:"generationId,omitempty"`
	ExpectedPath string `json:"expectedPath,omitempty"`
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
}

type businessStorageReportDiskFile struct {
	FileName  string `json:"fileName"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}

type businessStorageReportReference struct {
	FileName     string `json:"fileName"`
	UserID       string `json:"userId"`
	GenerationID string `json:"generationId"`
	OnDisk       bool   `json:"onDisk"`
}

type businessStorageReportBrokenAsset struct {
	FileName     string `json:"fileName"`
	UserID       string `json:"userId"`
	GenerationID string `json:"generationId"`
	Reason       string `json:"reason"`
}

type businessStorageBackfillResponse struct {
	Result businessimage.LegacyAssetBackfillResult `json:"result"`
	Report businessStorageReport                   `json:"report"`
}

func (s *Server) handleBusinessStorageReport(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessImageStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_storage_report_failed", err.Error())
		return
	}
	defer store.Close()

	data, err := store.StorageReportData(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_storage_report_failed", err.Error())
		return
	}
	diskFiles, err := s.scanBusinessImageDiskFiles()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_storage_scan_failed", err.Error())
		return
	}

	report := buildBusinessStorageReport(data, diskFiles, s.businessImageStorageDirs())
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleBackfillBusinessStorageAssets(w http.ResponseWriter, r *http.Request) {
	diskFiles, err := s.scanBusinessImageDiskFiles()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_storage_scan_failed", err.Error())
		return
	}
	backfillFiles, err := buildLegacyAssetBackfillFiles(diskFiles)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_storage_backfill_failed", err.Error())
		return
	}
	store, err := s.newBusinessImageStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_storage_backfill_failed", err.Error())
		return
	}
	defer store.Close()

	result, err := store.BackfillLegacyAssets(r.Context(), backfillFiles)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_storage_backfill_failed", err.Error())
		return
	}
	data, err := store.StorageReportData(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_storage_report_failed", err.Error())
		return
	}
	report := buildBusinessStorageReport(data, diskFiles, s.businessImageStorageDirs())
	writeJSON(w, http.StatusOK, businessStorageBackfillResponse{
		Result: result,
		Report: report,
	})
}

func buildBusinessStorageReport(data businessimage.StorageReportData, diskFiles map[string]businessStorageReportDiskFile, dirs []string) businessStorageReport {
	assetByFile := map[string]businessimage.StorageAsset{}
	referenced := map[string]struct{}{}
	assetBytes := int64(0)
	for _, asset := range data.Assets {
		fileName := filepath.Base(strings.TrimSpace(asset.FileName))
		if fileName == "" {
			continue
		}
		assetByFile[fileName] = asset
		referenced[fileName] = struct{}{}
		assetBytes += asset.SizeBytes
	}
	for _, reference := range data.References {
		fileName := filepath.Base(strings.TrimSpace(reference.FileName))
		if fileName == "" {
			continue
		}
		referenced[fileName] = struct{}{}
	}

	missingFiles := []businessStorageReportFile{}
	for fileName, asset := range assetByFile {
		if _, ok := diskFiles[fileName]; ok {
			continue
		}
		missingFiles = append(missingFiles, businessStorageReportFile{
			FileName:     fileName,
			UserID:       asset.UserID,
			GenerationID: asset.GenerationID,
			ExpectedPath: asset.FilePath,
			SizeBytes:    asset.SizeBytes,
		})
	}
	sort.Slice(missingFiles, func(i, j int) bool {
		return missingFiles[i].FileName < missingFiles[j].FileName
	})

	orphanFiles := []businessStorageReportDiskFile{}
	diskBytes := int64(0)
	orphanBytes := int64(0)
	for fileName, diskFile := range diskFiles {
		diskBytes += diskFile.SizeBytes
		if _, ok := referenced[fileName]; ok {
			continue
		}
		orphanFiles = append(orphanFiles, diskFile)
		orphanBytes += diskFile.SizeBytes
	}
	sort.Slice(orphanFiles, func(i, j int) bool {
		return orphanFiles[i].FileName < orphanFiles[j].FileName
	})

	legacyReferences := make([]businessStorageReportReference, 0, len(data.LegacyReferences))
	legacySeen := map[string]struct{}{}
	for _, reference := range data.LegacyReferences {
		fileName := filepath.Base(strings.TrimSpace(reference.FileName))
		if fileName == "" {
			continue
		}
		key := reference.UserID + "\x00" + reference.GenerationID + "\x00" + fileName
		if _, ok := legacySeen[key]; ok {
			continue
		}
		legacySeen[key] = struct{}{}
		_, onDisk := diskFiles[fileName]
		legacyReferences = append(legacyReferences, businessStorageReportReference{
			FileName:     fileName,
			UserID:       reference.UserID,
			GenerationID: reference.GenerationID,
			OnDisk:       onDisk,
		})
	}
	sort.Slice(legacyReferences, func(i, j int) bool {
		if legacyReferences[i].FileName == legacyReferences[j].FileName {
			return legacyReferences[i].GenerationID < legacyReferences[j].GenerationID
		}
		return legacyReferences[i].FileName < legacyReferences[j].FileName
	})

	brokenAssets := make([]businessStorageReportBrokenAsset, 0, len(data.BrokenAssets))
	for _, item := range data.BrokenAssets {
		brokenAssets = append(brokenAssets, businessStorageReportBrokenAsset{
			FileName:     item.FileName,
			UserID:       item.UserID,
			GenerationID: item.GenerationID,
			Reason:       item.Reason,
		})
	}
	sort.Slice(brokenAssets, func(i, j int) bool {
		return brokenAssets[i].FileName < brokenAssets[j].FileName
	})

	return businessStorageReport{
		Summary: businessStorageReportSummary{
			AssetFiles:            len(assetByFile),
			DiskFiles:             len(diskFiles),
			ReferencedFiles:       len(referenced),
			MissingFiles:          len(missingFiles),
			OrphanFiles:           len(orphanFiles),
			LegacyReferencedFiles: len(legacyReferences),
			BrokenAssets:          len(brokenAssets),
			AssetBytes:            assetBytes,
			DiskBytes:             diskBytes,
			OrphanBytes:           orphanBytes,
		},
		MissingFiles:          missingFiles,
		OrphanFiles:           orphanFiles,
		LegacyReferencedFiles: legacyReferences,
		BrokenAssets:          brokenAssets,
		Directories:           dirs,
	}
}

func (s *Server) scanBusinessImageDiskFiles() (map[string]businessStorageReportDiskFile, error) {
	files := map[string]businessStorageReportDiskFile{}
	for _, dir := range s.businessImageStorageDirs() {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != dir {
					return filepath.SkipDir
				}
				return nil
			}
			fileName := filepath.Base(entry.Name())
			if !strings.HasPrefix(fileName, "business-") {
				return nil
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				return err
			}
			if current, ok := files[fileName]; ok && current.Path <= path {
				return nil
			}
			files[fileName] = businessStorageReportDiskFile{
				FileName:  fileName,
				Path:      path,
				SizeBytes: info.Size(),
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func buildLegacyAssetBackfillFiles(diskFiles map[string]businessStorageReportDiskFile) (map[string]businessimage.LegacyAssetBackfillFile, error) {
	files := map[string]businessimage.LegacyAssetBackfillFile{}
	for fileName, diskFile := range diskFiles {
		payload, err := os.ReadFile(diskFile.Path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(payload)
		mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(fileName)))
		if strings.TrimSpace(mimeType) == "" {
			mimeType = http.DetectContentType(payload)
		}
		files[fileName] = businessimage.LegacyAssetBackfillFile{
			FileName:  fileName,
			FilePath:  diskFile.Path,
			MimeType:  mimeType,
			SizeBytes: diskFile.SizeBytes,
			SHA256:    hex.EncodeToString(sum[:]),
		}
	}
	return files, nil
}
